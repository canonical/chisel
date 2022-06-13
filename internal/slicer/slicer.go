package slicer

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
	extract := make(map[string]map[string][]deb.ExtractInfo)
	pathInfos := make(map[string]setup.PathInfo)

	release := options.Selection.Release

	// Build information to process the selection.
	for _, slice := range options.Selection.Slices {
		extractPackage := extract[slice.Package]
		if extractPackage == nil {
			archiveName := release.Packages[slice.Package].Archive
			archive := options.Archives[archiveName]
			if archive == nil {
				return fmt.Errorf("archive %q not defined", archiveName)
			}
			if !archive.Exists(slice.Package) {
				return fmt.Errorf("slice package %q missing from archive", slice.Package)
			}
			archives[slice.Package] = archive
			extractPackage = make(map[string][]deb.ExtractInfo)
			extract[slice.Package] = extractPackage
		}
		for targetPath, pathInfo := range slice.Contents {
			if targetPath == "" {
				continue
			}
			pathInfos[targetPath] = pathInfo
			if pathInfo.Kind == setup.CopyPath || pathInfo.Kind == setup.GlobPath {
				sourcePath := pathInfo.Info
				if sourcePath == "" {
					sourcePath = targetPath
				}
				extractPackage[sourcePath] = append(extractPackage[sourcePath], deb.ExtractInfo{
					Path: targetPath,
				})
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

	// Extract all packages, also using the selection order.
	for _, slice := range options.Selection.Slices {
		reader := packages[slice.Package]
		if reader == nil {
			continue
		}
		err := deb.Extract(reader, &deb.ExtractOptions{
			Package:   slice.Package,
			Extract:   extract[slice.Package],
			TargetDir: options.TargetDir,
		})
		reader.Close()
		packages[slice.Package] = nil
		if err != nil {
			return err
		}
	}

	// Create new content not coming from packages.
	done := make(map[string]bool)
	for _, slice := range options.Selection.Slices {
		for targetPath, pathInfo := range slice.Contents {
			if done[targetPath] || pathInfo.Kind == setup.CopyPath || pathInfo.Kind == setup.GlobPath {
				continue
			}
			done[targetPath] = true
			targetPath = filepath.Join(options.TargetDir, targetPath)
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
			return fmt.Errorf("cannot write file not mutable: %s", path)
		}
		return nil
	}
	checkRead := func(path string) error {
		if _, ok := pathInfos[path]; !ok {
			return fmt.Errorf("cannot read file not selected: %s", path)
		}
		return nil
	}
	content := &scripts.ContentValue{
		RootDir:    options.TargetDir,
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

	for targetPath, pathInfo := range pathInfos {
		if pathInfo.Until == setup.UntilMutate {
			path, err := content.RealPath(targetPath, scripts.CheckRead)
			if err == nil {
				err = os.Remove(path)
			}
			if err != nil {
				return fmt.Errorf("cannot perform 'until' removal: %w", err)
			}
		}
	}

	return nil
}
