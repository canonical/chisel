package slicer

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

func Run(options *RunOptions) (*Report, error) {

	archives := make(map[string]archive.Archive)
	extract := make(map[string]map[string][]deb.ExtractInfo)
	pathInfos := make(map[string]setup.PathInfo)
	report := NewReport(options.TargetDir)

	knownPaths := make(map[string]bool)
	knownPaths["/"] = true

	addKnownPath := func(path string) {
		if path[0] != '/' {
			panic("bug: tried to add relative path to known paths")
		}
		cleanPath := filepath.Clean(path)
		slashPath := cleanPath
		if path[len(path)-1] == '/' && cleanPath != "/" {
			slashPath += "/"
		}
		for {
			if _, ok := knownPaths[slashPath]; ok {
				break
			}
			knownPaths[slashPath] = true
			cleanPath = filepath.Dir(cleanPath)
			if cleanPath == "/" {
				break
			}
			slashPath = cleanPath + "/"
		}
	}

	oldUmask := syscall.Umask(0)
	defer func() {
		syscall.Umask(oldUmask)
	}()

	release := options.Selection.Release
	targetDir := filepath.Clean(options.TargetDir)
	targetDirAbs := targetDir
	if !filepath.IsAbs(targetDirAbs) {
		dir, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("cannot obtain current directory: %w", err)
		}
		targetDirAbs = filepath.Join(dir, targetDir)
	}

	// Build information to process the selection.
	for _, slice := range options.Selection.Slices {
		extractPackage := extract[slice.Package]
		if extractPackage == nil {
			archiveName := release.Packages[slice.Package].Archive
			archive := options.Archives[archiveName]
			if archive == nil {
				return nil, fmt.Errorf("archive %q not defined", archiveName)
			}
			if !archive.Exists(slice.Package) {
				return nil, fmt.Errorf("slice package %q missing from archive", slice.Package)
			}
			archives[slice.Package] = archive
			extractPackage = make(map[string][]deb.ExtractInfo)
			extract[slice.Package] = extractPackage
		}
		arch := archives[slice.Package].Options().Arch
		copyrightPath := "/usr/share/doc/" + slice.Package + "/copyright"
		hasCopyright := false
		for targetPath, pathInfo := range slice.Contents {
			if targetPath == "" {
				continue
			}
			if len(pathInfo.Arch) > 0 && !contains(pathInfo.Arch, arch) {
				continue
			}
			pathInfos[targetPath] = pathInfo

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

	// Fetch all packages, using the selection order.
	packages := make(map[string]io.ReadCloser)
	for _, slice := range options.Selection.Slices {
		if packages[slice.Package] != nil {
			continue
		}
		reader, err := archives[slice.Package].Fetch(slice.Package)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		packages[slice.Package] = reader
	}

	pathUntil := map[string]setup.PathUntil{}

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

			if pathInfo.Until == setup.UntilNone {
				until = setup.UntilNone
			}

			err := report.Add(slice, entry)
			if err != nil {
				return err
			}
		}
		if inSliceContents {
			pathUntil[relPath] = until
			addKnownPath(relPath)
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
		arch := archives[slice.Package].Options().Arch
		for relPath, pathInfo := range slice.Contents {
			if len(pathInfo.Arch) > 0 && !contains(pathInfo.Arch, arch) {
				continue
			}
			if done[relPath] || pathInfo.Kind == setup.CopyPath || pathInfo.Kind == setup.GlobPath {
				continue
			}
			done[relPath] = true
			addKnownPath(relPath)
			targetPath := filepath.Join(targetDir, relPath)
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

			entry, err := fsutil.Create(&fsutil.CreateOptions{
				Path:        targetPath,
				Mode:        tarHeader.FileInfo().Mode(),
				Data:        fileContent,
				Link:        linkTarget,
				MakeParents: true,
			})
			if err != nil {
				return nil, err
			}
			err = report.Add(slice, entry)
			if err != nil {
				return nil, err
			}
			if pathInfo.Until == setup.UntilMutate {
				pathUntil[relPath] = setup.UntilMutate
			}
		}
	}

	// Run mutation scripts. Order is fundamental here as
	// dependencies must run before dependents.
	checkWrite := func(path string) error {
		if !pathInfos[path].Mutable {
			return fmt.Errorf("cannot write file which is not mutable: %s", path)
		}
		return nil
	}
	checkRead := func(path string) error {
		var err error
		if !knownPaths[path] {
			// we assume that path is clean and ends with slash if it designates a directory
			if path[len(path)-1] == '/' {
				if path == "/" {
					panic("internal error: content root (\"/\") is not selected")
				}
				if knownPaths[path[:len(path)-1]] {
					err = fmt.Errorf("content is not a directory: %s", path[:len(path)-1])
				} else {
					err = fmt.Errorf("cannot list directory which is not selected: %s", path)
				}
			} else {
				if knownPaths[path+"/"] {
					err = fmt.Errorf("content is not a file: %s", path)
				} else {
					err = fmt.Errorf("cannot read file which is not selected: %s", path)
				}
			}
		}
		return err
	}
	content := &scripts.ContentValue{
		RootDir:    targetDirAbs,
		CheckWrite: checkWrite,
		CheckRead:  checkRead,
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

	var untilDirs []string
	for path, until := range pathUntil {
		if until != setup.UntilMutate {
			continue
		}
		realPath, err := content.RealPath(path, scripts.CheckRead)
		if err == nil {
			if strings.HasSuffix(path, "/") {
				untilDirs = append(untilDirs, realPath)
			} else {
				err = os.Remove(realPath)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("cannot perform 'until' removal: %w", err)
		}
	}
	// Order the directories so the deepest ones appear first, this way we can
	// check for empty directories properly.
	sort.Slice(untilDirs, func(i, j int) bool {
		return strings.Compare(untilDirs[i], untilDirs[j]) > 0
	})
	for _, realPath := range untilDirs {
		err := os.Remove(realPath)
		// The non-empty directory error is caught by IsExist as well.
		if err != nil && !os.IsExist(err) {
			return nil, fmt.Errorf("cannot perform 'until' removal: %#v", err)
		}
	}

	return report, nil
}

func contains(l []string, s string) bool {
	for _, si := range l {
		if si == s {
			return true
		}
	}
	return false
}
