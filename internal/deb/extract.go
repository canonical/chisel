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

	"github.com/blakesmith/ar"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"

	"github.com/canonical/chisel/internal/strdist"
)

type ExtractOptions struct {
	Package   string
	TargetDir string
	Extract   map[string][]ExtractInfo
}

type ExtractInfo struct {
	Path string
	Mode uint
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
				if strings.HasSuffix(extractPath, pkgPath) {
					return "", true
				}
				for _, extractInfo := range extractInfos {
					if strings.HasSuffix(extractInfo.Path, pkgPath) {
						return "", true
					}
				}
			}
		}
		return "", false
	}

	pendingPaths := make(map[string]bool)
	for extractPath, _ := range options.Extract {
		pendingPaths[extractPath] = true
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
		} else {
			extractInfos, ok = options.Extract[sourcePath]
			if ok {
				delete(pendingPaths, sourcePath)
			} else {
				// Base directory for extracted content. Relevant mainly to preserve
				// the metadata, since the extracted content itself will also create
				// any missing directories unaccounted for in the options.
				err := extractDir(filepath.Join(options.TargetDir, sourcePath), os.FileMode(tarHeader.Mode))
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
			targetMode := os.FileMode(extractInfo.Mode)
			if targetMode == 0 {
				targetMode = os.FileMode(tarHeader.Mode)
			}
			if targetMode&01000 != 0 {
				targetMode ^= 01000
				targetMode |= os.ModeSticky
			}

			switch tarHeader.Typeflag {
			case tar.TypeDir:
				err = extractDir(targetPath, targetMode)
			case tar.TypeReg:
				err = extractFile(pathReader, targetPath, targetMode)
			case tar.TypeSymlink:
				err = extractSymlink(tarHeader.Linkname, targetPath, targetMode)
			default:
				err = fmt.Errorf("unsupported file type: %s", sourcePath)
			}
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

func extractDir(targetPath string, mode os.FileMode) error {
	debugf("Extracting directory: %s (mode %#o)", targetPath, mode)
	err := os.MkdirAll(filepath.Dir(targetPath), 0755)
	if err != nil {
		return err
	}
	err = os.Mkdir(targetPath, mode)
	if err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

func extractFile(tarReader io.Reader, targetPath string, mode os.FileMode) error {
	debugf("Extracting file: %s (mode %#o)", targetPath, mode)
	err := os.MkdirAll(filepath.Dir(targetPath), 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}
	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(targetFile, tarReader)
	err = targetFile.Close()
	if copyErr != nil {
		return copyErr
	}
	return err
}

func extractSymlink(symlinkPath string, targetPath string, mode os.FileMode) error {
	debugf("Extracting symlink: %s => %s", targetPath, symlinkPath)
	err := os.MkdirAll(filepath.Dir(targetPath), 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}
	return os.Symlink(symlinkPath, targetPath)
}
