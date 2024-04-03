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
	"unicode/utf8"

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
	// Create can optionally be set to control the creation of extracted entries.
	// extractInfo is set to the matching entry in Extract, and is nil in cases where
	// the created entry is implicit and unlisted (for example, parent directories).
	Create func(extractInfo *ExtractInfo, options *fsutil.CreateOptions) error
}

type ExtractInfo struct {
	Path     string
	Mode     uint
	Optional bool
}

func getValidOptions(options *ExtractOptions) (*ExtractOptions, error) {
	for extractPath, extractInfos := range options.Extract {
		isGlob := strings.ContainsAny(extractPath, "*?")
		if isGlob {
			if len(extractInfos) != 1 || extractInfos[0].Path != extractPath || extractInfos[0].Mode != 0 {
				return nil, fmt.Errorf("when using wildcards source and target paths must match: %s", extractPath)
			}
		}
	}

	if options.Create == nil {
		validOpts := *options
		validOpts.Create = func(_ *ExtractInfo, o *fsutil.CreateOptions) error {
			_, err := fsutil.Create(o)
			return err
		}
		return &validOpts, nil
	}

	return options, nil
}

func Extract(pkgReader io.Reader, options *ExtractOptions) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot extract from package %q: %w", options.Package, err)
		}
	}()

	logf("Extracting files from package %q...", options.Package)

	validOpts, err := getValidOptions(options)
	if err != nil {
		return err
	}

	_, err = os.Stat(validOpts.TargetDir)
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
	return extractData(dataReader, validOpts)
}

func extractData(dataReader io.Reader, options *ExtractOptions) error {

	oldUmask := syscall.Umask(0)
	defer func() {
		syscall.Umask(oldUmask)
	}()

	shouldExtract := func(pkgPath string) (globPath string, ok bool) {
		if pkgPath == "" {
			return "", false
		}
		pkgPathIsDir := pkgPath[len(pkgPath)-1] == '/'
		for extractPath, extractInfos := range options.Extract {
			if extractPath == "" {
				continue
			}
			switch {
			case strings.ContainsAny(extractPath, "*?"):
				if strdist.GlobPath(extractPath, pkgPath) {
					return extractPath, true
				}
			case extractPath == pkgPath:
				return "", true
			case pkgPathIsDir:
				for _, extractInfo := range extractInfos {
					if strings.HasPrefix(extractInfo.Path, pkgPath) {
						return "", true
					}
				}
			}
		}
		return "", false
	}

	pendingPaths := make(map[string]bool)
	for extractPath, extractInfos := range options.Extract {
		for _, extractInfo := range extractInfos {
			if !extractInfo.Optional {
				pendingPaths[extractPath] = true
				break
			}
		}
	}

	// When creating a file we will iterate through its parent directories and
	// create them with the permissions defined in the tarball.
	//
	// The assumption is that the tar entries of the parent directories appear
	// before the entry for the file itself. This is the case for .deb files but
	// not for all tarballs.
	tarDirMode := make(map[string]fs.FileMode)
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
		globPath, ok := shouldExtract(sourcePath)
		if !ok {
			continue
		}

		sourceIsDir := sourcePath[len(sourcePath)-1] == '/'
		if sourceIsDir {
			tarDirMode[sourcePath] = tarHeader.FileInfo().Mode()
		}

		//debugf("Extracting header: %#v", tarHeader)

		var extractInfos []ExtractInfo
		if globPath != "" {
			extractInfos = options.Extract[globPath]
			delete(pendingPaths, globPath)
		} else {
			extractInfos, ok = options.Extract[sourcePath]
			if !ok {
				continue
			}
			delete(pendingPaths, sourcePath)
		}

		var contentCache []byte
		var contentIsCached = len(extractInfos) > 1 && !sourceIsDir && globPath == ""
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
			var relPath string
			if globPath == "" {
				relPath = extractInfo.Path
			} else {
				relPath = sourcePath
			}
			if extractInfo.Mode != 0 {
				tarHeader.Mode = int64(extractInfo.Mode)
			}
			// Create the parent directories using the permissions from the tarball.
			parents := parentDirs(relPath)
			for _, path := range parents {
				if path == "/" {
					continue
				}
				mode, ok := tarDirMode[path]
				if !ok {
					continue
				}
				delete(tarDirMode, path)

				absPath := filepath.Join(options.TargetDir, path)
				if _, err := os.Stat(absPath); !os.IsNotExist(err) {
					// We dont want to this implicit parent directory to overwrite
					// an existing directory.
					continue
				}
				createOptions := &fsutil.CreateOptions{
					Path:        absPath,
					Mode:        mode,
					MakeParents: true,
				}
				err := options.Create(nil, createOptions)
				if err != nil {
					return err
				}
			}
			// Create the entry itself.
			createOptions := &fsutil.CreateOptions{
				Path:        filepath.Join(options.TargetDir, relPath),
				Mode:        tarHeader.FileInfo().Mode(),
				Data:        pathReader,
				Link:        tarHeader.Linkname,
				MakeParents: true,
			}
			err := options.Create(&extractInfo, createOptions)
			if err != nil {
				return err
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

func parentDirs(path string) []string {
	path = filepath.Clean(path)
	parents := make([]string, strings.Count(path, "/"))
	count := 0
	for i, width := 0, 0; i < len(path); i += width {
		c, w := utf8.DecodeRuneInString(path[i:])
		if c == '/' {
			parents[count] = path[:i+w]
			count++
		}
		width = w
	}
	return parents
}
