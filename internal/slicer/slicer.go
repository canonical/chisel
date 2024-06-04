package slicer

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/deb"
	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/scripts"
	"github.com/canonical/chisel/internal/setup"
)

type RunOptions struct {
	Selection *setup.Selection
	Archives  map[string]archive.Archive
	TargetDir string
}

type pathData struct {
	until   setup.PathUntil
	mutable bool
}

type contentChecker struct {
	knownPaths map[string]pathData
}

func (cc *contentChecker) checkMutable(path string) error {
	if !cc.knownPaths[path].mutable {
		return fmt.Errorf("cannot write file which is not mutable: %s", path)
	}
	return nil
}

func (cc *contentChecker) checkKnown(path string) error {
	var err error
	if _, ok := cc.knownPaths[path]; !ok {
		// We assume that path is clean and ends with slash if it designates a directory.
		if path[len(path)-1] == '/' {
			if path == "/" {
				panic("internal error: content root (\"/\") is not selected")
			}
			if _, ok := cc.knownPaths[path[:len(path)-1]]; ok {
				err = fmt.Errorf("content is not a directory: %s", path[:len(path)-1])
			} else {
				err = fmt.Errorf("cannot list directory which is not selected: %s", path)
			}
		} else {
			if _, ok := cc.knownPaths[path+"/"]; ok {
				err = fmt.Errorf("content is not a file: %s", path)
			} else {
				err = fmt.Errorf("cannot read file which is not selected: %s", path)
			}
		}
	}
	return err
}

func Run(options *RunOptions) (*Report, error) {
	report := NewReport(options.TargetDir)

	oldUmask := syscall.Umask(0)
	defer func() {
		syscall.Umask(oldUmask)
	}()

	targetDir := filepath.Clean(options.TargetDir)
	targetDirAbs := targetDir
	if !filepath.IsAbs(targetDirAbs) {
		dir, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("cannot obtain current directory: %w", err)
		}
		targetDirAbs = filepath.Join(dir, targetDir)
	}

	packages, err := fetchPackages(options)
	if err != nil {
		return nil, err
	}
	for _, reader := range packages {
		defer reader.Close()
	}

	// Build information to process the selection.
	extract := make(map[string]map[string][]deb.ExtractInfo)
	for _, slice := range options.Selection.Slices {
		extractPackage := extract[slice.Package]
		if extractPackage == nil {
			extractPackage = make(map[string][]deb.ExtractInfo)
			extract[slice.Package] = extractPackage
		}
		archiveName := options.Selection.Release.Packages[slice.Package].Archive
		arch := options.Archives[archiveName].Options().Arch
		copyrightPath := "/usr/share/doc/" + slice.Package + "/copyright"
		hasCopyright := false
		for targetPath, pathInfo := range slice.Contents {
			if targetPath == "" {
				continue
			}
			if len(pathInfo.Arch) > 0 && !slices.Contains(pathInfo.Arch, arch) {
				continue
			}

			if pathInfo.Kind == setup.CopyPath || pathInfo.Kind == setup.GlobPath {
				sourcePath := pathInfo.Info
				if sourcePath == "" {
					sourcePath = targetPath
				}
				extractPackage[sourcePath] = append(extractPackage[sourcePath], deb.ExtractInfo{
					Path:    targetPath,
					Context: slice,
				})
				if sourcePath == copyrightPath && targetPath == copyrightPath {
					hasCopyright = true
				}
			} else {
				// When the content is not extracted from the package (i.e. path is
				// not glob or copy), we add a ExtractInfo for the parent directory
				// to preserve the permissions from the tarball where possible.
				targetDir := filepath.Dir(strings.TrimRight(targetPath, "/")) + "/"
				if targetDir == "" || targetDir == "/" {
					continue
				}
				extractPackage[targetDir] = append(extractPackage[targetDir], deb.ExtractInfo{
					Path:     targetDir,
					Optional: true,
				})
			}
		}
		if !hasCopyright {
			extractPackage[copyrightPath] = append(extractPackage[copyrightPath], deb.ExtractInfo{
				Path:     copyrightPath,
				Optional: true,
			})
		}
	}

	// When creating content, record if a path is known and whether they are
	// listed as until: mutate in all the slices that reference them.
	knownPaths := map[string]pathData{}
	addParents(knownPaths, "/")

	// Creates the filesystem entry and adds it to the report.
	create := func(extractInfos []deb.ExtractInfo, o *fsutil.CreateOptions) error {
		entry, err := fsutil.Create(o)
		if err != nil {
			return err
		}
		// Content created was not listed in a slice contents because extractInfo
		// is empty.
		if len(extractInfos) == 0 {
			return nil
		}

		relPath := filepath.Clean("/" + strings.TrimLeft(o.Path, targetDir))
		if o.Mode.IsDir() {
			relPath = relPath + "/"
		}
		inSliceContents := false
		until := setup.UntilMutate
		mutable := false
		for _, extractInfo := range extractInfos {
			if extractInfo.Context == nil {
				continue
			}
			slice, ok := extractInfo.Context.(*setup.Slice)
			if !ok {
				return fmt.Errorf("internal error: invalid Context of type %T in extractInfo", extractInfo.Context)
			}
			pathInfo, ok := slice.Contents[extractInfo.Path]
			if !ok {
				return fmt.Errorf("internal error: path %q not listed in slice contents", extractInfo.Path)
			}
			inSliceContents = true
			mutable = mutable || pathInfo.Mutable
			if pathInfo.Until == setup.UntilNone {
				until = setup.UntilNone
			}
			err := report.Add(slice, entry)
			if err != nil {
				return err
			}
		}

		if inSliceContents {
			knownPaths[relPath] = pathData{mutable: mutable, until: until}
			addParents(knownPaths, relPath)
		}
		return nil
	}

	// Extract all packages, also using the selection order.
	for _, slice := range options.Selection.Slices {
		reader := packages[slice.Package]
		if reader == nil {
			continue
		}
		err := deb.Extract(reader, &deb.ExtractOptions{
			Package:   slice.Package,
			Extract:   extract[slice.Package],
			TargetDir: targetDir,
			Create:    create,
		})
		reader.Close()
		packages[slice.Package] = nil
		if err != nil {
			return nil, err
		}
	}

	// Create new content not coming from packages.
	done := make(map[string]bool)
	for _, slice := range options.Selection.Slices {
		archiveName := options.Selection.Release.Packages[slice.Package].Archive
		arch := options.Archives[archiveName].Options().Arch
		for relPath, pathInfo := range slice.Contents {
			if len(pathInfo.Arch) > 0 && !slices.Contains(pathInfo.Arch, arch) {
				continue
			}
			if done[relPath] || pathInfo.Kind == setup.CopyPath || pathInfo.Kind == setup.GlobPath {
				continue
			}
			done[relPath] = true
			addParents(knownPaths, relPath)
			targetPath := filepath.Join(targetDir, relPath)
			entry, err := createFile(targetPath, pathInfo)
			if err != nil {
				return nil, err
			}
			err = report.Add(slice, entry)
			if err != nil {
				return nil, err
			}
			if pathInfo.Until == setup.UntilMutate || pathInfo.Mutable {
				knownPaths[relPath] = pathData{
					until:   pathInfo.Until,
					mutable: pathInfo.Mutable,
				}
			}
		}
	}

	// Run mutation scripts. Order is fundamental here as
	// dependencies must run before dependents.
	checker := contentChecker{knownPaths}
	content := &scripts.ContentValue{
		RootDir:    targetDirAbs,
		CheckWrite: checker.checkMutable,
		CheckRead:  checker.checkKnown,
	}
	for _, slice := range options.Selection.Slices {
		opts := scripts.RunOptions{
			Label:  "mutate",
			Script: slice.Scripts.Mutate,
			Namespace: map[string]scripts.Value{
				"content": content,
			},
		}
		err := scripts.Run(&opts)
		if err != nil {
			return nil, fmt.Errorf("slice %s: %w", slice, err)
		}
	}

	err = removeUntilMutate(targetDirAbs, knownPaths)
	if err != nil {
		return nil, err
	}

	return report, nil
}

// removeUntilMutate removes entries marked with until: mutate. A path is marked
// only when all slices that refer to the path mark it with until: mutate.
func removeUntilMutate(rootDir string, knownPaths map[string]pathData) error {
	var untilDirs []string
	for path, data := range knownPaths {
		if data.until != setup.UntilMutate {
			continue
		}
		realPath := filepath.Join(rootDir, path)
		if strings.HasSuffix(path, "/") {
			untilDirs = append(untilDirs, realPath)
		} else {
			err := os.Remove(realPath)
			if err != nil {
				return fmt.Errorf("cannot perform 'until' removal: %w", err)
			}
		}
	}
	// Order the directories so the deepest ones appear first, this way we can
	// check for empty directories properly.
	sort.Slice(untilDirs, func(i, j int) bool {
		return untilDirs[i] > untilDirs[j]
	})
	for _, realPath := range untilDirs {
		err := os.Remove(realPath)
		// The non-empty directory error is caught by IsExist as well.
		if err != nil && !os.IsExist(err) {
			return fmt.Errorf("cannot perform 'until' removal: %#v", err)
		}
	}
	return nil
}

// addParents adds all parent directories of path to the list of known paths.
// The path has to be absolute.
func addParents(knownPaths map[string]pathData, path string) {
	if !strings.HasPrefix(path, "/") {
		panic("bug: tried to add relative path to known paths")
	}
	cleanPath := filepath.Clean(path)
	slashPath := cleanPath
	if strings.HasSuffix(path, "/") && cleanPath != "/" {
		slashPath += "/"
	}
	for {
		if _, ok := knownPaths[slashPath]; !ok {
			knownPaths[slashPath] = pathData{}
		}
		cleanPath = filepath.Dir(cleanPath)
		if cleanPath == "/" {
			break
		}
		slashPath = cleanPath + "/"
	}
}

// fetchPackages fetches all packages using the selection order.
func fetchPackages(options *RunOptions) (map[string]io.ReadCloser, error) {
	packages := make(map[string]io.ReadCloser)
	// When returning an error do not leak resources.
	closeAll := func() {
		for _, pkg := range packages {
			pkg.Close()
		}
	}
	for _, slice := range options.Selection.Slices {
		if packages[slice.Package] != nil {
			continue
		}
		archiveName := options.Selection.Release.Packages[slice.Package].Archive
		archive := options.Archives[archiveName]
		if archive == nil {
			closeAll()
			return nil, fmt.Errorf("archive %q not defined", archiveName)
		}
		if !archive.Exists(slice.Package) {
			closeAll()
			return nil, fmt.Errorf("slice package %q missing from archive", slice.Package)
		}
		reader, err := archive.Fetch(slice.Package)
		if err != nil {
			closeAll()
			return nil, err
		}
		packages[slice.Package] = reader
	}
	return packages, nil
}

func createFile(targetPath string, pathInfo setup.PathInfo) (*fsutil.Entry, error) {
	targetMode := pathInfo.Mode
	if targetMode == 0 {
		if pathInfo.Kind == setup.DirPath {
			targetMode = 0755
		} else {
			targetMode = 0644
		}
	}

	// Leverage tar handling of mode bits.
	tarHeader := tar.Header{Mode: int64(targetMode)}
	var fileContent io.Reader
	var linkTarget string
	switch pathInfo.Kind {
	case setup.TextPath:
		tarHeader.Typeflag = tar.TypeReg
		fileContent = bytes.NewBufferString(pathInfo.Info)
	case setup.DirPath:
		tarHeader.Typeflag = tar.TypeDir
	case setup.SymlinkPath:
		tarHeader.Typeflag = tar.TypeSymlink
		linkTarget = pathInfo.Info
	default:
		return nil, fmt.Errorf("internal error: cannot extract path of kind %q", pathInfo.Kind)
	}

	return fsutil.Create(&fsutil.CreateOptions{
		Path:        targetPath,
		Mode:        tarHeader.FileInfo().Mode(),
		Data:        fileContent,
		Link:        linkTarget,
		MakeParents: true,
	})
}
