package slicer

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/setup"
)

type ReportEntry struct {
	Path      string
	Mode      fs.FileMode
	Hash      string
	Size      int
	Slices    map[*setup.Slice]bool
	Link      string
	FinalHash string
}

// Report holds the information about files and directories created when slicing
// packages.
type Report struct {
	// Root is the filesystem path where the all reported content is based.
	Root string
	// Entries holds all reported content, indexed by their path.
	Entries map[string]ReportEntry
}

// NewReport returns an empty report for content that will be based at the
// provided root path.
func NewReport(root string) (*Report, error) {
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("cannot use relative path for report root: %q", root)
	}
	report := &Report{
		Root:    filepath.Clean(root) + "/",
		Entries: make(map[string]ReportEntry),
	}
	return report, nil
}

func (r *Report) Add(slice *setup.Slice, fsEntry *fsutil.Entry) error {
	relPath, err := r.sanitizeAbsPath(fsEntry.Path, fsEntry.Mode.IsDir())
	if err != nil {
		return fmt.Errorf("cannot add path to report: %s", err)
	}

	if entry, ok := r.Entries[relPath]; ok {
		if fsEntry.Mode != entry.Mode {
			return fmt.Errorf("path %s reported twice with diverging mode: 0%03o != 0%03o", relPath, fsEntry.Mode, entry.Mode)
		} else if fsEntry.Link != entry.Link {
			return fmt.Errorf("path %s reported twice with diverging link: %q != %q", relPath, fsEntry.Link, entry.Link)
		} else if fsEntry.Size != entry.Size {
			return fmt.Errorf("path %s reported twice with diverging size: %d != %d", relPath, fsEntry.Size, entry.Size)
		} else if fsEntry.Hash != entry.Hash {
			return fmt.Errorf("path %s reported twice with diverging hash: %q != %q", relPath, fsEntry.Hash, entry.Hash)
		}
		entry.Slices[slice] = true
		r.Entries[relPath] = entry
	} else {
		r.Entries[relPath] = ReportEntry{
			Path:   relPath,
			Mode:   fsEntry.Mode,
			Hash:   fsEntry.Hash,
			Size:   fsEntry.Size,
			Slices: map[*setup.Slice]bool{slice: true},
			Link:   fsEntry.Link,
		}
	}
	return nil
}

// Mutate updates the FinalHash and Size of an existing path entry.
func (r *Report) Mutate(fsEntry *fsutil.Entry) error {
	relPath, err := r.sanitizeAbsPath(fsEntry.Path, fsEntry.Mode.IsDir())
	if err != nil {
		return fmt.Errorf("cannot mutate path in report: %s", err)
	}

	entry, ok := r.Entries[relPath]
	if !ok {
		return fmt.Errorf("cannot mutate path in report: %s not previously added", relPath)
	}
	if entry.Mode.IsDir() {
		return fmt.Errorf("cannot mutate path in report: %s is a directory", relPath)
	}
	if entry.Hash == fsEntry.Hash {
		// Content has not changed, nothing to do.
		return nil
	}
	entry.FinalHash = fsEntry.Hash
	entry.Size = fsEntry.Size
	r.Entries[relPath] = entry
	return nil
}

func (r *Report) sanitizeAbsPath(path string, isDir bool) (relPath string, err error) {
	if !strings.HasPrefix(path, r.Root) {
		return "", fmt.Errorf("%s outside of root %s", path, r.Root)
	}
	relPath = filepath.Clean("/" + strings.TrimPrefix(path, r.Root))
	if isDir {
		relPath = relPath + "/"
	}
	return relPath, nil
}
