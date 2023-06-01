package slicer

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

func Run(options *RunOptions) error {

	archives := make(map[string]archive.Archive)
	pathInfos := make(map[string]setup.PathInfo)
	knownPaths := make(map[string]bool)

	knownPaths["/"] = true

	addKnownPath := func(path string) {
		if path[0] != '/' {
			panic("bug: tried to add relative path to known paths")
		}
		path = fsutil.Clean(path)
		for {
			if _, ok := knownPaths[path]; ok {
				break
			}
			knownPaths[path] = true
			if path = fsutil.Dir(path); path == "/" {
				break
			}
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
			return fmt.Errorf("cannot obtain current directory: %w", err)
		}
		targetDirAbs = filepath.Join(dir, targetDir)
	}

	type targetPathInfo struct {
		sourcePath string
		optional   bool
	}
	targetInfos := make(map[string]map[string]*targetPathInfo)

	// Build information to process the selection.
	for _, slice := range options.Selection.Slices {
		packageTargetInfos := targetInfos[slice.Package]
		if packageTargetInfos == nil {
			archiveName := release.Packages[slice.Package].Archive
			archive := options.Archives[archiveName]
			if archive == nil {
				return fmt.Errorf("archive %q not defined", archiveName)
			}
			if !archive.Exists(slice.Package) {
				return fmt.Errorf("slice package %q missing from archive", slice.Package)
			}
			archives[slice.Package] = archive
			packageTargetInfos = make(map[string]*targetPathInfo)
			targetInfos[slice.Package] = packageTargetInfos
		}
		arch := archives[slice.Package].Options().Arch
		for targetPath, pathInfo := range slice.Contents {
			if targetPath == "" {
				continue
			}
			if len(pathInfo.Arch) > 0 && !contains(pathInfo.Arch, arch) {
				continue
			}
			if pathInfo.Kind != setup.GlobPath {
				addKnownPath(targetPath)
			}
			pathInfos[targetPath] = pathInfo
			if pathInfo.Kind == setup.CopyPath || pathInfo.Kind == setup.GlobPath {
				sourcePath := pathInfo.Info
				if sourcePath == "" {
					sourcePath = targetPath
				}
				targetInfo := packageTargetInfos[targetPath]
				if targetInfo != nil {
					if targetInfo.sourcePath != sourcePath {
						panic("internal error: target path with multiple distinct sources")
					}
					if targetInfo.optional {
						targetInfo.optional = false
					}
				} else {
					packageTargetInfos[targetPath] = &targetPathInfo{
						sourcePath: sourcePath,
					}
				}
			}
			if pathInfo.Kind != setup.GlobPath {
				parent := fsutil.Dir(targetPath)
				for ; parent != "/"; parent = fsutil.Dir(parent) {
					if _, ok := packageTargetInfos[parent]; ok {
						break
					}
					packageTargetInfos[parent] = &targetPathInfo{
						sourcePath: parent,
						optional:   true,
					}
				}
			}
		}
		copyrightPath := "/usr/share/doc/" + slice.Package + "/copyright"
		if _, ok := packageTargetInfos[copyrightPath]; !ok {
			packageTargetInfos[copyrightPath] = &targetPathInfo{
				sourcePath: copyrightPath,
				optional:   true,
			}
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
			return err
		}
		defer reader.Close()
		packages[slice.Package] = reader
	}

	globbedPaths := make(map[string][]string)

	// Extract all packages, also using the selection order.
	for _, slice := range options.Selection.Slices {
		extractPackage := make(map[string][]deb.ExtractInfo)
		packageTargetInfo := targetInfos[slice.Package]
		for targetPath, targetInfo := range packageTargetInfo {
			extractPackage[targetInfo.sourcePath] = append(extractPackage[targetInfo.sourcePath], deb.ExtractInfo{
				Path:     targetPath,
				Optional: targetInfo.optional,
			})
		}
		reader := packages[slice.Package]
		if reader == nil {
			continue
		}
		err := deb.Extract(reader, &deb.ExtractOptions{
			Package:   slice.Package,
			Extract:   extractPackage,
			TargetDir: targetDir,
			Globbed:   globbedPaths,
		})
		reader.Close()
		packages[slice.Package] = nil
		if err != nil {
			return err
		}
	}

	for _, expandedPaths := range globbedPaths {
		for _, path := range expandedPaths {
			addKnownPath(path)
		}
	}

	// Create new content not coming from packages.
	done := make(map[string]bool)
	for _, slice := range options.Selection.Slices {
		arch := archives[slice.Package].Options().Arch
		for targetPath, pathInfo := range slice.Contents {
			if len(pathInfo.Arch) > 0 && !contains(pathInfo.Arch, arch) {
				continue
			}
			if done[targetPath] || pathInfo.Kind == setup.CopyPath || pathInfo.Kind == setup.GlobPath {
				continue
			}
			done[targetPath] = true
			targetPath = filepath.Join(targetDir, targetPath)
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
				return fmt.Errorf("internal error: cannot extract path of kind %q", pathInfo.Kind)
			}

			err := fsutil.Create(&fsutil.CreateOptions{
				Path: targetPath,
				Mode: tarHeader.FileInfo().Mode(),
				Data: fileContent,
				Link: linkTarget,
			})
			if err != nil {
				return err
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
			return fmt.Errorf("slice %s: %w", slice, err)
		}
	}

	var untilDirs []string
	for targetPath, pathInfo := range pathInfos {
		if pathInfo.Until == setup.UntilMutate {
			var targetPaths []string
			if pathInfo.Kind == setup.GlobPath {
				targetPaths = globbedPaths[targetPath]
			} else {
				targetPaths = []string{targetPath}
			}
			for _, targetPath := range targetPaths {
				realPath, err := content.RealPath(targetPath, scripts.CheckRead)
				if err == nil {
					if strings.HasSuffix(targetPath, "/") {
						untilDirs = append(untilDirs, realPath)
					} else {
						err = os.Remove(realPath)
					}
				}
				if err != nil {
					return fmt.Errorf("cannot perform 'until' removal: %w", err)
				}
			}
		}
	}
	for _, realPath := range untilDirs {
		err := os.Remove(realPath)
		// The non-empty directory error is caught by IsExist as well.
		if err != nil && !os.IsExist(err) {
			return fmt.Errorf("cannot perform 'until' removal: %#v", err)
		}
	}

	return nil
}

func contains(l []string, s string) bool {
	for _, si := range l {
		if si == s {
			return true
		}
	}
	return false
}
