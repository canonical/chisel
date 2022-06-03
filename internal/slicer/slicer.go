package slicer

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/deb"
	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/setup"
)

type RunOptions struct {
	Selection *setup.Selection
	Archives  map[string]archive.Archive
	TargetDir string
}

func Run(options *RunOptions) error {

	archives := make(map[string]archive.Archive)
	packages := make(map[string]io.ReadCloser)
	extract := make(map[string]map[string][]deb.ExtractInfo)

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
				debugf("Adding optional: %s", targetDir)
				extractPackage[targetDir] = append(extractPackage[targetDir], deb.ExtractInfo{
					Path:     targetDir,
					Optional: true,
				})
			}
		}
	}

	// Fetch all packages, using the selection order.
	for name := range extract {
		reader, err := archives[name].Fetch(name)
		if err != nil {
			return err
		}
		defer reader.Close()
		packages[name] = reader
	}

	// Fetch all packages, also using the selection order.
	for name, reader := range packages {
		err := deb.Extract(reader, &deb.ExtractOptions{
			Package:   name,
			Extract:   extract[name],
			TargetDir: options.TargetDir,
		})
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

	return nil
}
