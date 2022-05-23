package slicer

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/deb"
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
			if pathInfo.Kind == setup.CopyPath {
				sourcePath := pathInfo.Info
				if sourcePath == "" {
					sourcePath = targetPath
				}
				extractPackage[sourcePath] = append(extractPackage[sourcePath], deb.ExtractInfo{
					Path: targetPath,
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
			if done[targetPath] || pathInfo.Kind == setup.CopyPath {
				continue
			}
			done[targetPath] = true
			targetPath = filepath.Join(options.TargetDir, targetPath)
			targetMode := os.FileMode(pathInfo.Mode)
			if targetMode == 0 {
				if pathInfo.Kind == setup.DirPath {
					targetMode = 0755
				} else {
					targetMode = 0644
				}
			}
			switch pathInfo.Kind {
			case setup.DirPath:
				extractDir(targetPath, targetMode)
			case setup.TextPath:
				extractFile([]byte(pathInfo.Info), targetPath, targetMode)
			case setup.SymlinkPath:
				extractSymlink(pathInfo.Info, targetPath, targetMode)
			default:
				return fmt.Errorf("internal error: cannot extract path of kind %q", pathInfo.Kind)
			}
		}
	}

	return nil
}

func extractDir(targetPath string, mode os.FileMode) error {
	return os.MkdirAll(targetPath, mode)
}

func extractFile(data []byte, targetPath string, mode os.FileMode) error {
	err := os.MkdirAll(filepath.Dir(targetPath), 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}
	return ioutil.WriteFile(targetPath, data, mode)
}

func extractSymlink(symlinkPath string, targetPath string, mode os.FileMode) error {
	err := os.MkdirAll(filepath.Dir(targetPath), 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}
	return os.Symlink(symlinkPath, targetPath)
}
