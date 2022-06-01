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
	"strings"

	"github.com/blakesmith/ar"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
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

func Extract(pkgReader io.Reader, options *ExtractOptions) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot extract files from package %q: %w", options.Package, err)
		}
	}()

	logf("Extracting files from package %q...", options.Package)

	_, err = os.Stat(options.TargetDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("target directory does not exist")
	} else if err != nil {
		return err
	}

	arReader := ar.NewReader(pkgReader)
	for {
		arHeader, err := arReader.Next()
		if err == io.EOF {
			return fmt.Errorf("no data payload")
		}
		if err != nil {
			return err
		}
		var dataReader io.Reader
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
		if dataReader != nil {
			err = extractData(dataReader, options)
			if err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func extractData(dataReader io.Reader, options *ExtractOptions) error {

	extract := make(map[string]bool)
	for path, extractInfos := range options.Extract {
		extract[path] = true
		for _, extractInfo := range extractInfos {
			path := extractInfo.Path
			for len(path) > 1 {
				extract[path] = true
				pos := strings.LastIndexByte(path[:len(path)-1], filepath.Separator)
				if pos == -1 {
					break
				}
				path = path[:pos+1]
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
		if !extract[sourcePath] {
			continue
		}
		sourceIsDir := sourcePath[len(sourcePath)-1] == '/'

		//debugf("Extracting header: %#v", tarHeader)

		extractInfos, ok := options.Extract[sourcePath]
		if !ok {
			// Base directory for extracted content. Relevant mainly to preserve
			// the metadata, since the content will also create any missing
			// directories unaccounted for in the original content.
			err := extractDir(filepath.Join(options.TargetDir, sourcePath), os.FileMode(tarHeader.Mode))
			if err != nil {
				return err
			}
			continue
		}

		var contentCache []byte
		var contentIsCached = len(extractInfos) > 1 && !sourceIsDir
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
			targetPath := filepath.Join(options.TargetDir, extractInfo.Path)
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
