package tarball

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/canonical/chisel/internal/deb"
	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/strdist"
)

type ExtractOptions struct {
	Package   string
	TargetDir string
	Extract   map[string][]ExtractInfo
	// Create can optionally be set to control the creation of extracted entries.
	// extractInfos is set to the matching entries in Extract, and is nil in cases where
	// the created entry is implicit and unlisted (for example, parent directories).
	Create func(extractInfos []ExtractInfo, options *fsutil.CreateOptions) error
}

type ExtractInfo struct {
	Path     string
	Mode     uint
	Optional bool
	Context  any
}

func getValidOptions(options *ExtractOptions) (*ExtractOptions, error) {
	for extractPath, extractInfos := range options.Extract {
		isGlob := strings.ContainsAny(extractPath, "*?")
		if isGlob {
			for _, extractInfo := range extractInfos {
				if extractInfo.Path != extractPath || extractInfo.Mode != 0 {
					return nil, fmt.Errorf("when using wildcards source and target paths must match: %s", extractPath)
				}
			}
		}
	}

	if options.Create == nil {
		validOpts := *options
		validOpts.Create = func(_ []ExtractInfo, o *fsutil.CreateOptions) error {
			_, err := fsutil.Create(o)
			return err
		}
		return &validOpts, nil
	}

	return options, nil
}

func Extract(pkgReader io.ReadSeeker, options *ExtractOptions) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot extract from package %q: %w", options.Package, err)
		}
	}()

	logf("Extracting files from package %q...", options.Package)

	validOpts, err := getValidOptions(options)
	if err != nil {
		return err
	}

	_, err = os.Stat(validOpts.TargetDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("target directory does not exist")
	} else if err != nil {
		return err
	}

	return extractData(pkgReader, validOpts)
}

func extractData(pkgReader io.ReadSeeker, options *ExtractOptions) error {
	dataReader, err := deb.DataReader(pkgReader)
	if err != nil {
		return err
	}
	defer dataReader.Close()

	oldUmask := syscall.Umask(0)
	defer func() {
		syscall.Umask(oldUmask)
	}()

	pendingPaths := make(map[string]bool)
	for extractPath, extractInfos := range options.Extract {
		for _, extractInfo := range extractInfos {
			if !extractInfo.Optional {
				pendingPaths[extractPath] = true
				break
			}
		}
	}

	// Store the hard links that we cannot extract when we first iterate over
	// the tarball.
	//
	// This happens because the tarball only stores the contents once in the
	// first entry and the rest of them point to the first one. Therefore, we
	// cannot tell whether we need to extract the content until after we get to
	// a hard link. In this case, we need a second pass.
	pendingHardLinks := make(map[string][]pendingHardLink)

	// When creating a file we will iterate through its parent directories and
	// create them with the permissions defined in the tarball.
	//
	// The assumption is that the tar entries of the parent directories appear
	// before the entry for the file itself. This is the case for .deb files but
	// not for all tarballs.
	tarDirMode := make(map[string]fs.FileMode)
	// Entries extracted by this package at their own path. A hard link is
	// only created against one of these: anything else at the link target
	// came from another package.
	extracted := make(map[string]bool)
	// Where each hard link entry of the tarball points, so that a link that
	// could not be created can find the content at the end of a chain of
	// links once the whole tarball has been seen.
	linkTargets := make(map[string]string)
	tarReader := tar.NewReader(dataReader)
	for {
		tarHeader, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		sourcePath, ok := sanitizeTarPath(tarHeader.Name)
		if !ok {
			continue
		}

		sourceIsDir := sourcePath[len(sourcePath)-1] == '/'
		if sourceIsDir {
			tarDirMode[sourcePath] = tarHeader.FileInfo().Mode()
		}
		if tarHeader.Typeflag == tar.TypeLink {
			if target, ok := sanitizeLinkTarget(tarHeader.Linkname); ok {
				linkTargets[sourcePath] = target
			}
		}

		// Find all globs and copies that require this source, and map them by
		// their target paths on disk.
		targetPaths := map[string][]ExtractInfo{}
		for extractPath, extractInfos := range options.Extract {
			if extractPath == "" {
				continue
			}
			if strings.ContainsAny(extractPath, "*?") {
				if strdist.GlobPath(extractPath, sourcePath) {
					targetPaths[sourcePath] = append(targetPaths[sourcePath], extractInfos...)
					delete(pendingPaths, extractPath)
				}
			} else if extractPath == sourcePath {
				for _, extractInfo := range extractInfos {
					targetPaths[extractInfo.Path] = append(targetPaths[extractInfo.Path], extractInfo)
				}
				delete(pendingPaths, extractPath)
			}
		}
		if len(targetPaths) == 0 {
			// Nothing to do.
			continue
		}

		var contentCache []byte
		var contentIsCached = len(targetPaths) > 1 && !sourceIsDir
		if contentIsCached {
			// Read and cache the content so it may be reused.
			// As an alternative, to avoid having an entire file in
			// memory at once this logic might open the first file
			// written and copy it every time. For now, the choice
			// is speed over memory efficiency.
			data, err := io.ReadAll(tarReader)
			if err != nil {
				return err
			}
			contentCache = data
		}

		var pathReader io.Reader = tarReader
		for targetPath, extractInfos := range targetPaths {
			if contentIsCached {
				pathReader = bytes.NewReader(contentCache)
			}
			mode := extractInfos[0].Mode
			for _, extractInfo := range extractInfos {
				if extractInfo.Mode != mode {
					if mode < extractInfo.Mode {
						mode, extractInfo.Mode = extractInfo.Mode, mode
					}
					return fmt.Errorf("path %s requested twice with diverging mode: 0%03o != 0%03o", targetPath, mode, extractInfo.Mode)
				}
			}
			if mode != 0 {
				tarHeader.Mode = int64(mode)
			}
			// Create the parent directories using the permissions from the tarball.
			parents := parentDirs(targetPath)
			for _, path := range parents {
				if path == "/" {
					continue
				}
				mode, ok := tarDirMode[path]
				if !ok {
					continue
				}
				delete(tarDirMode, path)

				createOptions := &fsutil.CreateOptions{
					Root:        options.TargetDir,
					Path:        path,
					Mode:        mode,
					MakeParents: true,
				}
				err := options.Create(nil, createOptions)
				if err != nil {
					return err
				}
			}
			link := tarHeader.Linkname
			if tarHeader.Typeflag == tar.TypeLink {
				relLinkPath, ok := sanitizeLinkTarget(tarHeader.Linkname)
				if !ok {
					return fmt.Errorf("invalid link target %s", tarHeader.Linkname)
				}
				if !extracted[relLinkPath] {
					// The content was not extracted previously, so the
					// hard link entry is dealt with once the whole
					// tarball has been seen.
					info := pendingHardLink{
						path:         targetPath,
						mode:         tarHeader.FileInfo().Mode(),
						extractInfos: extractInfos,
					}
					pendingHardLinks[relLinkPath] = append(pendingHardLinks[relLinkPath], info)
					continue
				}
				// A hard link requires the real path of the target file.
				link = filepath.Join(options.TargetDir, relLinkPath)
			}

			// Create the entry itself.
			createOptions := &fsutil.CreateOptions{
				Root:         options.TargetDir,
				Path:         targetPath,
				Mode:         tarHeader.FileInfo().Mode(),
				Data:         pathReader,
				Link:         link,
				MakeParents:  true,
				OverrideMode: true,
			}
			err := options.Create(extractInfos, createOptions)
			if err != nil {
				return err
			}
			if targetPath == sourcePath {
				extracted[targetPath] = true
			}
		}
	}

	// A pending link waits for the content at the end of its chain of links.
	// When that content was extracted by this package the link is created
	// against it now; otherwise the second pass extracts the content.
	for linkPath, links := range pendingHardLinks {
		endPath := linkPath
		for i := 0; i < len(linkTargets); i++ {
			next, ok := linkTargets[endPath]
			if !ok {
				break
			}
			endPath = next
		}
		if extracted[endPath] {
			for _, link := range links {
				createOptions := &fsutil.CreateOptions{
					Root:        options.TargetDir,
					Path:        link.path,
					Mode:        link.mode,
					Link:        filepath.Join(options.TargetDir, endPath),
					MakeParents: true,
				}
				err := options.Create(link.extractInfos, createOptions)
				if err != nil {
					return err
				}
			}
			delete(pendingHardLinks, linkPath)
		} else if endPath != linkPath {
			delete(pendingHardLinks, linkPath)
			pendingHardLinks[endPath] = append(pendingHardLinks[endPath], links...)
		}
	}

	if len(pendingHardLinks) > 0 {
		// Go over the tarball again to textract the pending hard links.
		extractHardLinkOptions := &extractHardLinkOptions{
			ExtractOptions: options,
			pendingLinks:   pendingHardLinks,
		}
		_, err := pkgReader.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}
		err = extractHardLinks(pkgReader, extractHardLinkOptions)
		if err != nil {
			return err
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

type pendingHardLink struct {
	path         string
	mode         fs.FileMode
	extractInfos []ExtractInfo
}

type extractHardLinkOptions struct {
	*ExtractOptions
	pendingLinks map[string][]pendingHardLink
}

// extractHardLinks iterates through the tarball a second time to extract the
// hard links that were not extracted in the first pass.
func extractHardLinks(pkgReader io.ReadSeeker, opts *extractHardLinkOptions) error {
	dataReader, err := deb.DataReader(pkgReader)
	if err != nil {
		return err
	}
	defer dataReader.Close()

	tarReader := tar.NewReader(dataReader)
	for {
		tarHeader, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		sourcePath, ok := sanitizeTarPath(tarHeader.Name)
		if !ok {
			continue
		}

		links := opts.pendingLinks[sourcePath]
		if len(links) == 0 {
			continue
		}
		if tarHeader.Typeflag == tar.TypeLink {
			// Only a chain of links leading nowhere ends here.
			return fmt.Errorf("cannot create hard link %s: %s is a hard link", links[0].path, sourcePath)
		}

		// For a target path, the first hard link will be created as a file with
		// the content of the target path. If there are more pending hard links,
		// the remaining ones will be created as hard links with the newly
		// created file as their target.
		absLink := filepath.Join(opts.TargetDir, links[0].path)
		// Extract the content to the first hard link path. A hard link to a
		// symlink becomes a symlink with the same target.
		createOptions := &fsutil.CreateOptions{
			Root:        opts.TargetDir,
			Path:        links[0].path,
			Mode:        tarHeader.FileInfo().Mode(),
			Data:        tarReader,
			MakeParents: true,
		}
		if tarHeader.Typeflag == tar.TypeSymlink {
			createOptions.Link = tarHeader.Linkname
		}
		err = opts.Create(links[0].extractInfos, createOptions)
		if err != nil {
			return err
		}

		// Create the remaining hard links.
		for _, link := range links[1:] {
			createOptions := &fsutil.CreateOptions{
				Root: opts.TargetDir,
				Path: link.path,
				Mode: tarHeader.FileInfo().Mode(),
				// Link to the first file extracted for the hard links.
				Link:        absLink,
				MakeParents: true,
			}
			err := opts.Create(link.extractInfos, createOptions)
			if err != nil {
				return err
			}
		}
		delete(opts.pendingLinks, sourcePath)
	}

	// If there are pending links, that means the link targets do not come from
	// this package.
	if len(opts.pendingLinks) > 0 {
		var targets []string
		for target := range opts.pendingLinks {
			targets = append(targets, target)
		}
		sort.Strings(targets)
		link := opts.pendingLinks[targets[0]][0]
		return fmt.Errorf("cannot create hard link %s: no content at %s", link.path, targets[0])
	}

	return nil
}

func parentDirs(path string) []string {
	path = filepath.Clean(path)
	parents := make([]string, strings.Count(path, "/"))
	count := 0
	for i, c := range path {
		if c == '/' {
			parents[count] = path[:i+1]
			count++
		}
	}
	return parents
}

// sanitizeLinkTarget returns the clean path a hard link entry points at, which
// the tarball may spell with or without the leading "./".
func sanitizeLinkTarget(link string) (string, bool) {
	if link == "" || link[0] == '/' {
		return "", false
	}
	return filepath.Clean("/" + strings.TrimPrefix(link, "./")), true
}

// sanitizeTarPath removes the leading "./" from the source path in the tarball,
// and verifies that the path is not empty.
func sanitizeTarPath(path string) (string, bool) {
	if len(path) < 3 || path[0] != '.' || path[1] != '/' {
		return "", false
	}
	return path[1:], true
}
