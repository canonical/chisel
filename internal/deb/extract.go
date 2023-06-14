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

type ConsumeData func(reader io.Reader) error

// DataCallback function is called when a tarball entry of a regular file is
// going to be extracted.
//
// The source and size parameters are set to the file's path in the tarball and
// the size of its content, respectively. When the returned ConsumeData function
// is not nil, it is called with the file's content.
//
// When the source path occurs in the tarball more than once, this function will
// be called for each occurrence.
//
// If either DataCallback or ConsumeData function returns a non-nil error, the
// Extract() function will fail with that error.
type DataCallback func(source string, size int64) (ConsumeData, error)

// The CreateCallback function is called when a tarball entry is going to be
// extracted to a target path.
//
// The target, link, and mode parameters are set to the target's path, symbolic
// link target, and mode, respectively. The target's filesystem type can be
// determined from these parameters. If the link is not empty, the target is a
// symbolic link. Otherwise, if the target's path ends with /, it is a
// directory. Otherwise, it is a regular file.
//
// When the source parameter is not empty, the target is going to be extracted
// from a tarball entry with the source path. The function may be called more
// than once with the same source when the tarball entry is extracted to
// multiple different targets.
//
// Otherwise, the mode is 0755 and the target is going to be an implicitly
// created parent directory of another target, and when a directory entry with
// that path is later encountered in the tarball with a different mode, the
// function will be called again with the same target, source equal to the
// target, and the different mode.
//
// When the source path occurs in the tarball more than once, this function will
// be called for each target path for each occurrence.
//
// If CreateCallback function returns a non-nil error, the Extract() function
// will fail with that error.
type CreateCallback func(source, target, link string, mode fs.FileMode) error

type ExtractOptions struct {
	Package   string
	TargetDir string
	Extract   map[string][]ExtractInfo
	Globbed   map[string][]string
	OnData    DataCallback
	OnCreate  CreateCallback
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
			source := dir
			if info.created {
				return nil
			} else if info.mode == 0 {
				info.mode = fs.ModeDir | 0755
				source = ""
			}
			if err := createParents(dir); err != nil {
				return err
			}
			if options.OnCreate != nil {
				if err := options.OnCreate(source, dir, "", info.mode); err != nil {
					return err
				}
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

		getReader := func() io.Reader { return tarReader }

		if sourceMode.IsRegular() {
			var consumeData ConsumeData
			if options.OnData != nil {
				var err error
				if consumeData, err = options.OnData(sourcePath, tarHeader.Size); err != nil {
					return err
				}
			}
			if consumeData != nil || (len(extractInfos) > 1 && globPath == "") {
				// Read and cache the content so it may be reused.
				// As an alternative, to avoid having an entire file in
				// memory at once this logic might open the first file
				// written and copy it every time. For now, the choice
				// is speed over memory efficiency.
				data, err := io.ReadAll(tarReader)
				if err != nil {
					return err
				}
				getReader = func() io.Reader { return bytes.NewReader(data) }
			}
			if consumeData != nil {
				if err := consumeData(getReader()); err != nil {
					return err
				}
			}
		}

		origMode := tarHeader.Mode
		for _, extractInfo := range extractInfos {
			var targetPath string
			if globPath == "" {
				targetPath = extractInfo.Path
			} else {
				targetPath = sourcePath
			}
			if err := createParents(targetPath); err != nil {
				return err
			}
			tarHeader.Mode = origMode
			if extractInfo.Mode != 0 {
				tarHeader.Mode = int64(extractInfo.Mode)
			}
			fsMode := tarHeader.FileInfo().Mode()
			if options.OnCreate != nil {
				if err := options.OnCreate(sourcePath, targetPath, tarHeader.Linkname, fsMode); err != nil {
					return err
				}
			}
			err := fsutil.Create(&fsutil.CreateOptions{
				Path: filepath.Join(options.TargetDir, targetPath),
				Mode: fsMode,
				Data: getReader(),
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
