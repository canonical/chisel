package deb

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/blakesmith/ar"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"

	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/strdist"
)

type ExtractOptions struct {
	Package   string
	TargetDir string
	Extract   map[string][]ExtractInfo
	Globbed   map[string][]string
}

type ExtractInfo struct {
	Path     string
	Mode     uint
	Optional bool
}

func checkExtractOptions(options *ExtractOptions) error {
	for extractPath, extractInfos := range options.Extract {
		isGlob := strings.ContainsAny(extractPath, "*?")
		if isGlob {
			if len(extractInfos) != 1 || extractInfos[0].Path != extractPath || extractInfos[0].Mode != 0 {
				return fmt.Errorf("when using wildcards source and target paths must match: %s", extractPath)
			}
		}
	}
	return nil
}

func Extract(pkgReader io.Reader, options *ExtractOptions) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot extract from package %q: %w", options.Package, err)
		}
	}()

	logf("Extracting files from package %q...", options.Package)

	err = checkExtractOptions(options)
	if err != nil {
		return err
	}

	_, err = os.Stat(options.TargetDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("target directory does not exist")
	} else if err != nil {
		return err
	}

	arReader := ar.NewReader(pkgReader)
	var dataReader io.Reader
	for dataReader == nil {
		arHeader, err := arReader.Next()
		if err == io.EOF {
			return fmt.Errorf("no data payload")
		}
		if err != nil {
			return err
		}
		switch arHeader.Name {
		case "data.tar.gz":
			gzipReader, err := gzip.NewReader(arReader)
			if err != nil {
				return err
			}
			defer gzipReader.Close()
			dataReader = gzipReader
		case "data.tar.xz":
			xzReader, err := xz.NewReader(arReader)
			if err != nil {
				return err
			}
			dataReader = xzReader
		case "data.tar.zst":
			zstdReader, err := zstd.NewReader(arReader)
			if err != nil {
				return err
			}
			defer zstdReader.Close()
			dataReader = zstdReader
		}
	}
	return extractData(dataReader, options)
}

func extractData(dataReader io.Reader, options *ExtractOptions) error {

	oldUmask := syscall.Umask(0)
	defer func() {
		syscall.Umask(oldUmask)
	}()

	pendingPaths := make(map[string]bool)
	var globs []string
	for extractPath, extractInfos := range options.Extract {
		for _, extractInfo := range extractInfos {
			if !extractInfo.Optional {
				pendingPaths[extractPath] = true
				break
			}
		}
		if strings.ContainsAny(extractPath, "*?") {
			globs = append(globs, extractInfos[0].Path)
		}
	}
	sort.Strings(globs)

	// A visitInfo contains information about an associated package path in
	// the visitedPaths map.
	//
	// When created is true, it was created, either because it was matched
	// in the extract map or because it was a parent of a matched path.
	// When it is false, the path was skipped, but we still want to keep it
	// in the visitedPaths map, because later when a glob matches a path
	// below this path, we want to create this path with the metadata from
	// the header, in which case, we will set created to true.
	//
	// The header is a tar header associated with the path. When it is nil,
	// then created must be true and this visitInfo represents a "future
	// visit". This may happen if we match a package path that's to be
	// copied to a different target path. The target path directory parents
	// may not yet exist though, so we create them with 0755 mode and
	// associate with them visitInfo structures with nil header in
	// visitedPaths map. Later, when we encounter the directories in the
	// package and we find the associated visitInfo structures with nil
	// headers, we use the new tar headers to adjust modes of the
	// previously created directories.
	type visitInfo struct {
		created bool
		header  *tar.Header
	}
	visitedPaths := make(map[string]*visitInfo)

	tarReader := tar.NewReader(dataReader)
	for {
		tarHeader, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		sourcePath := tarHeader.Name
		if len(sourcePath) < 3 || sourcePath[0] != '.' || sourcePath[1] != '/' {
			continue
		}
		sourcePath = sourcePath[1:]

		sourceIsDir := sourcePath[len(sourcePath)-1] == '/'

		visit := visitedPaths[sourcePath]
		if visit == nil {
			visit = &visitInfo{header: tarHeader}
			visitedPaths[sourcePath] = visit
		} else if !sourceIsDir || visit.header != nil {
			panic("internal error: visited a file for the second time")
		}

		globPath := ""
		extractInfos, want := options.Extract[sourcePath]
		if !want {
			for _, globPath = range globs {
				if want = strdist.GlobPath(globPath, sourcePath); want {
					break
				}
			}
		}

		if !want {
			if visit.header != nil {
				// This visit was created above in this
				// iteration and it is not a wanted path, so
				// skip it.
				continue
			}
			// This is a "future visit" representing previously
			// created parent directory for which we didn't have a
			// tar header yet. We have it now. Continue as if it's
			// wanted so that fsutil.Create() can adjust its mode.
			visit.header = tarHeader
			extractInfos = []ExtractInfo{{Path: sourcePath}}
		}

		if globPath != "" {
			extractInfos = []ExtractInfo{{Path: globPath}}
			if options.Globbed != nil {
				options.Globbed[globPath] = append(options.Globbed[globPath], sourcePath)
			}
			delete(pendingPaths, globPath)
		} else {
			delete(pendingPaths, sourcePath)
		}

		var createParents func(path string) error
		createParents = func(path string) error {
			dir := fsutil.Dir(path)
			if dir == "/" {
				return nil
			}
			visit := visitedPaths[dir]
			if visit == nil {
				// We didn't see nor create this directory
				// before.
				visit = &visitInfo{}
				visitedPaths[dir] = visit
			} else if visit.created {
				// This directory was either in the extract map
				// or it was created by this function before.
				return nil
			}
			// Create parents first.
			if err := createParents(dir); err != nil {
				return err
			}
			create := fsutil.CreateOptions{
				Path: filepath.Join(options.TargetDir, dir),
				Mode: fs.ModeDir | 0755,
			}
			if visit.header != nil {
				// This directory was already encountered in
				// the package but it wasn't in the extract map
				// so it wasn't created.
				create.Mode = visit.header.FileInfo().Mode()
			}
			if err := fsutil.Create(&create); err != nil {
				return err
			}
			visit.created = true
			return nil
		}

		var contentCache []byte
		var contentIsCached = len(extractInfos) > 1 && !sourceIsDir && globPath == ""
		if contentIsCached {
			// Read and cache the content so it may be reused.
			// As an alternative, to avoid having an entire file in
			// memory at once this logic might open the first file
			// written and copy it every time. For now, the choice
			// is speed over memory efficiency.
			data, err := ioutil.ReadAll(tarReader)
			if err != nil {
				return err
			}
			contentCache = data
		}

		var pathReader io.Reader = tarReader
		for _, extractInfo := range extractInfos {
			if contentIsCached {
				pathReader = bytes.NewReader(contentCache)
			}
			var targetPath string
			if globPath == "" {
				targetPath = extractInfo.Path
			} else {
				targetPath = sourcePath
			}
			if err := createParents(targetPath); err != nil {
				return err
			}
			if extractInfo.Mode != 0 {
				tarHeader.Mode = int64(extractInfo.Mode)
			}
			err := fsutil.Create(&fsutil.CreateOptions{
				Path: filepath.Join(options.TargetDir, targetPath),
				Mode: tarHeader.FileInfo().Mode(),
				Data: pathReader,
				Link: tarHeader.Linkname,
			})
			if err != nil {
				return err
			}
			if globPath != "" {
				break
			}
		}
		visit.created = true
	}

	if len(pendingPaths) > 0 {
		pendingList := make([]string, 0, len(pendingPaths))
		for pendingPath := range pendingPaths {
			pendingList = append(pendingList, pendingPath)
		}
		if len(pendingList) == 1 {
			return fmt.Errorf("no content at %s", pendingList[0])
		} else {
			sort.Strings(pendingList)
			return fmt.Errorf("no content at:\n- %s", strings.Join(pendingList, "\n- "))
		}
	}

	return nil
}
