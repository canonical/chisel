package slicer

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"

	"github.com/klauspost/compress/zstd"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/deb"
	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/manifest"
	"github.com/canonical/chisel/internal/scripts"
	"github.com/canonical/chisel/internal/setup"
)

const manifestMode fs.FileMode = 0644

type RunOptions struct {
	Selection *setup.Selection
	Archives  map[string]archive.Archive
	TargetDir string
}

type pathData struct {
	until    setup.PathUntil
	mutable  bool
	hardLink bool
}

type contentChecker struct {
	knownPaths map[string]pathData
}

func (cc *contentChecker) checkMutable(path string) error {
	if !cc.knownPaths[path].mutable {
		return fmt.Errorf("cannot write file which is not mutable: %s", path)
	}
	if cc.knownPaths[path].hardLink {
		return fmt.Errorf("cannot mutate a hard link: %s", path)
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

func Run(options *RunOptions) error {
	oldUmask := syscall.Umask(0)
	defer func() {
		syscall.Umask(oldUmask)
	}()

	targetDir := filepath.Clean(options.TargetDir)
	if !filepath.IsAbs(targetDir) {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot obtain current directory: %w", err)
		}
		targetDir = filepath.Join(dir, targetDir)
	}

	pkgArchive, err := selectPkgArchives(options.Archives, options.Selection)
	if err != nil {
		return err
	}

	// Build information to process the selection.
	extract := make(map[string]map[string][]deb.ExtractInfo)
	for _, slice := range options.Selection.Slices {
		extractPackage := extract[slice.Package]
		if extractPackage == nil {
			extractPackage = make(map[string][]deb.ExtractInfo)
			extract[slice.Package] = extractPackage
		}
		arch := pkgArchive[slice.Package].Options().Arch
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
	}

	// Fetch all packages, using the selection order.
	packages := make(map[string]io.ReadSeekCloser)
	var pkgInfos []*archive.PackageInfo
	for _, slice := range options.Selection.Slices {
		if packages[slice.Package] != nil {
			continue
		}
		reader, info, err := pkgArchive[slice.Package].Fetch(slice.Package)
		if err != nil {
			return err
		}
		defer reader.Close()
		packages[slice.Package] = reader
		pkgInfos = append(pkgInfos, info)
	}

	// When creating content, record if a path is known and whether they are
	// listed as until: mutate in all the slices that reference them.
	knownPaths := map[string]pathData{}
	addKnownPath(knownPaths, "/", pathData{})
	report, err := manifest.NewReport(targetDir)
	if err != nil {
		return fmt.Errorf("internal error: cannot create report: %w", err)
	}

	// Creates the filesystem entry and adds it to the report. It also updates
	// knownPaths with the files created.
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

		relPath := filepath.Clean("/" + strings.TrimPrefix(o.Path, targetDir))
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
			// Do not add paths with "until: mutate".
			if pathInfo.Until != setup.UntilMutate {
				err := report.Add(slice, entry)
				if err != nil {
					return err
				}
			}
		}

		if inSliceContents {
			data := pathData{
				mutable:  mutable,
				until:    until,
				hardLink: entry.HardLink,
			}
			addKnownPath(knownPaths, relPath, data)
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
			return err
		}
	}

	// Create new content not extracted from packages, e.g. TextPath or DirPath
	// with {make: true}. The only exception is the manifest which will be created
	// later.
	// First group them by their relative path. Then create them and attribute
	// them to the appropriate slices.
	relPaths := map[string][]*setup.Slice{}
	for _, slice := range options.Selection.Slices {
		arch := pkgArchive[slice.Package].Options().Arch
		for relPath, pathInfo := range slice.Contents {
			if len(pathInfo.Arch) > 0 && !slices.Contains(pathInfo.Arch, arch) {
				continue
			}
			if pathInfo.Kind == setup.CopyPath || pathInfo.Kind == setup.GlobPath ||
				pathInfo.Kind == setup.GeneratePath {
				continue
			}
			relPaths[relPath] = append(relPaths[relPath], slice)
		}
	}
	for relPath, slices := range relPaths {
		until := setup.UntilMutate
		for _, slice := range slices {
			if slice.Contents[relPath].Until == setup.UntilNone {
				until = setup.UntilNone
				break
			}
		}
		// It is okay to take the first pathInfo because the release has been
		// validated when read and there are no conflicts. The only field that
		// was not checked was until because it is not used for conflict
		// validation.
		pathInfo := slices[0].Contents[relPath]
		pathInfo.Until = until
		data := pathData{
			until:   pathInfo.Until,
			mutable: pathInfo.Mutable,
		}
		addKnownPath(knownPaths, relPath, data)
		targetPath := filepath.Join(targetDir, relPath)
		entry, err := createFile(targetPath, pathInfo)
		if err != nil {
			return err
		}

		// Do not add paths with "until: mutate".
		if pathInfo.Until != setup.UntilMutate {
			for _, slice := range slices {
				err = report.Add(slice, entry)
				if err != nil {
					return err
				}
			}
		}
	}

	// Run mutation scripts. Order is fundamental here as
	// dependencies must run before dependents.
	checker := contentChecker{knownPaths}
	content := &scripts.ContentValue{
		RootDir:    targetDir,
		CheckWrite: checker.checkMutable,
		CheckRead:  checker.checkKnown,
		OnWrite:    report.Mutate,
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

	err = removeAfterMutate(targetDir, knownPaths)
	if err != nil {
		return err
	}

	return generateManifests(targetDir, options.Selection, report, pkgInfos)
}

func generateManifests(targetDir string, selection *setup.Selection,
	report *manifest.Report, pkgInfos []*archive.PackageInfo) error {
	manifestSlices := manifest.FindPaths(selection.Slices)
	if len(manifestSlices) == 0 {
		// Nothing to do.
		return nil
	}
	var writers []io.Writer
	for relPath, slices := range manifestSlices {
		logf("Generating manifest at %s...", relPath)
		absPath := filepath.Join(targetDir, relPath)
		createOptions := &fsutil.CreateOptions{
			Path:        absPath,
			Mode:        manifestMode,
			MakeParents: true,
		}
		writer, info, err := fsutil.CreateWriter(createOptions)
		if err != nil {
			return err
		}
		defer writer.Close()
		writers = append(writers, writer)
		for _, slice := range slices {
			err := report.Add(slice, info)
			if err != nil {
				return err
			}
		}
	}
	w, err := zstd.NewWriter(io.MultiWriter(writers...))
	if err != nil {
		return err
	}
	defer w.Close()
	writeOptions := &manifest.WriteOptions{
		PackageInfo: pkgInfos,
		Selection:   selection.Slices,
		Report:      report,
	}
	err = manifest.Write(writeOptions, w)
	return err
}

// removeAfterMutate removes entries marked with until: mutate. A path is marked
// only when all slices that refer to the path mark it with until: mutate.
func removeAfterMutate(rootDir string, knownPaths map[string]pathData) error {
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

// addKnownPath adds a path with its data to the list of known paths. Then it
// records that the parent directories of the path are also known.
func addKnownPath(knownPaths map[string]pathData, path string, data pathData) {
	if !strings.HasPrefix(path, "/") {
		panic("bug: tried to add relative path to known paths")
	}
	cleanPath := filepath.Clean(path)
	slashPath := cleanPath
	if strings.HasSuffix(path, "/") && cleanPath != "/" {
		slashPath += "/"
	}
	for {
		if _, ok := knownPaths[slashPath]; ok {
			break
		}
		knownPaths[slashPath] = data
		// The parents have empty data.
		data = pathData{}
		cleanPath = filepath.Dir(cleanPath)
		if cleanPath == "/" {
			break
		}
		slashPath = cleanPath + "/"
	}
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

// selectPkgArchives selects the highest priority archive containing the package
// unless a particular archive is pinned within the slice definition file. It
// returns a map of archives indexed by package names.
func selectPkgArchives(archives map[string]archive.Archive, selection *setup.Selection) (map[string]archive.Archive, error) {
	sortedArchives := make([]*setup.Archive, 0, len(selection.Release.Archives))
	for _, archive := range selection.Release.Archives {
		if archive.Priority < 0 {
			// Ignore negative priority archives unless a package specifically
			// asks for it with the "archive" field.
			continue
		}
		sortedArchives = append(sortedArchives, archive)
	}
	slices.SortFunc(sortedArchives, func(a, b *setup.Archive) int {
		return b.Priority - a.Priority
	})

	pkgArchive := make(map[string]archive.Archive)
	for _, s := range selection.Slices {
		if _, ok := pkgArchive[s.Package]; ok {
			continue
		}
		pkg := selection.Release.Packages[s.Package]

		var candidates []*setup.Archive
		if pkg.Archive == "" {
			// If the package has not pinned any archive, choose the highest
			// priority archive in which the package exists.
			candidates = sortedArchives
		} else {
			candidates = []*setup.Archive{selection.Release.Archives[pkg.Archive]}
		}

		var chosen archive.Archive
		for _, archiveInfo := range candidates {
			archive := archives[archiveInfo.Name]
			if archive != nil && archive.Exists(pkg.Name) {
				chosen = archive
				break
			}
		}
		if chosen == nil {
			return nil, fmt.Errorf("cannot find package %q in archive(s)", pkg.Name)
		}
		pkgArchive[pkg.Name] = chosen
	}
	return pkgArchive, nil
}
