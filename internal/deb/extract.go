package deb

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
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
			globs = append(globs, extractPath)
		}
	}

	// dirInfo represents a directory that we
	//   a) encountered in the tarball,
	//   b) created, or
	//   c) both a) and c).
	type dirInfo struct {
		// mode of the directory with which we
		//   a) encountered it in the tarball, or
		//   b) created it
		mode fs.FileMode
		// whether we created this directory
		created bool
	}
	// directories we encountered and/or created
	dirInfos := make(map[string]dirInfo)

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
		sourceMode := tarHeader.FileInfo().Mode()

		globPath := ""
		extractInfos := options.Extract[sourcePath]

		if extractInfos == nil {
			for _, glob := range globs {
				if strdist.GlobPath(glob, sourcePath) {
					globPath = glob
					extractInfos = []ExtractInfo{{Path: glob}}
					break
				}
			}
		}

		// Is this a directory path that was not requested?
		if extractInfos == nil && sourceMode.IsDir() {
			if info := dirInfos[sourcePath]; info.mode != sourceMode {
				// We have not seen this directory yet, or we
				// have seen or created it with a different mode
				// before. Record the source path mode.
				info.mode = sourceMode
				dirInfos[sourcePath] = info
				if info.created {
					// We have created this directory before
					// with a different mode. Create it
					// again with the proper mode.
					extractInfos = []ExtractInfo{{Path: sourcePath}}
				}
			}
		}

		if extractInfos == nil {
			continue
		}

		if globPath != "" {
			if options.Globbed != nil {
				options.Globbed[globPath] = append(options.Globbed[globPath], sourcePath)
			}
			delete(pendingPaths, globPath)
		} else {
			delete(pendingPaths, sourcePath)
		}

		// createParents creates missing parent directories of the path
		// with modes with which they were encountered in the tarball or
		// 0755 if they were not encountered yet.
		var createParents func(path string) error
		createParents = func(path string) error {
			dir := fsutil.SlashedPathDir(path)
			if dir == "/" {
				return nil
			}
			info := dirInfos[dir]
			if info.created {
				return nil
			} else if info.mode == 0 {
				info.mode = fs.ModeDir | 0755
			}
			if err := createParents(dir); err != nil {
				return err
			}
			create := fsutil.CreateOptions{
				Path: filepath.Join(options.TargetDir, dir),
				Mode: info.mode,
			}
			if err := fsutil.Create(&create); err != nil {
				return err
			}
			info.created = true
			dirInfos[dir] = info
			return nil
		}

		var contentCache []byte
		var contentIsCached = len(extractInfos) > 1 && sourceMode.IsRegular() && globPath == ""
		if contentIsCached {
			// Read and cache the content so it may be reused.
			// As an alternative, to avoid having an entire file in
			// memory at once this logic might open the first file
			// written and copy it every time. For now, the choice
			// is speed over memory efficiency.
			data, err := io.ReadAll(tarReader)
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
			fsMode := tarHeader.FileInfo().Mode()
			err := fsutil.Create(&fsutil.CreateOptions{
				Path: filepath.Join(options.TargetDir, targetPath),
				Mode: fsMode,
				Data: pathReader,
				Link: tarHeader.Linkname,
			})
			if err != nil {
				return err
			}
			if fsMode.IsDir() {
				// Record the target directory mode.
				dirInfos[targetPath] = dirInfo{fsMode, true}
			}
			if globPath != "" {
				break
			}
		}
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
