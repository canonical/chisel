package deb

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
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

		//debugf("Extracting header: %#v", tarHeader)

		var extractInfos []ExtractInfo
		if globPath != "" {
			extractInfos = options.Extract[globPath]
			delete(pendingPaths, globPath)
			if options.Globbed != nil {
				options.Globbed[globPath] = append(options.Globbed[globPath], sourcePath)
			}
		} else {
			extractInfos, ok = options.Extract[sourcePath]
			if ok {
				delete(pendingPaths, sourcePath)
			} else {
				// Base directory for extracted content. Relevant mainly to preserve
				// the metadata, since the extracted content itself will also create
				// any missing directories unaccounted for in the options.
				err := fsutil.Create(&fsutil.CreateOptions{
					Path: filepath.Join(options.TargetDir, sourcePath),
					Mode: tarHeader.FileInfo().Mode(),
				})
				if err != nil {
					return err
				}
				continue
			}
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
				targetPath = filepath.Join(options.TargetDir, extractInfo.Path)
			} else {
				targetPath = filepath.Join(options.TargetDir, sourcePath)
			}
			if extractInfo.Mode != 0 {
				tarHeader.Mode = int64(extractInfo.Mode)
			}
			err := fsutil.Create(&fsutil.CreateOptions{
				Path: targetPath,
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
