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
	Mutated   bool
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
func NewReport(root string) *Report {
	return &Report{
		Root:    filepath.Clean(root) + "/",
		Entries: make(map[string]ReportEntry),
	}
}

func (r *Report) Add(slice *setup.Slice, fsEntry *fsutil.Entry) error {
	path, err := r.sanitizePath(fsEntry.Path, fsEntry.Mode.IsDir())
	if err != nil {
		return fmt.Errorf("cannot add path: %s", err)
	}

	if entry, ok := r.Entries[path]; ok {
		if fsEntry.Mode != entry.Mode {
			return fmt.Errorf("path %q reported twice with diverging mode: %q != %q", path, fsEntry.Mode, entry.Mode)
		} else if fsEntry.Link != entry.Link {
			return fmt.Errorf("path %q reported twice with diverging link: %q != %q", path, fsEntry.Link, entry.Link)
		} else if fsEntry.Size != entry.Size {
			return fmt.Errorf("path %q reported twice with diverging size: %d != %d", path, fsEntry.Size, entry.Size)
		} else if fsEntry.Hash != entry.Hash {
			return fmt.Errorf("path %q reported twice with diverging hash: %q != %q", path, fsEntry.Hash, entry.Hash)
		}
		entry.Slices[slice] = true
		r.Entries[path] = entry
	} else {
		r.Entries[path] = ReportEntry{
			Path:   path,
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
	path, err := r.sanitizePath(fsEntry.Path, fsEntry.Mode.IsDir())
	if err != nil {
		return fmt.Errorf("cannot mutate path: %w", err)
	}

	entry, ok := r.Entries[path]
	if !ok {
		return fmt.Errorf("cannot mutate path %q: no entry in report", path)
	}
	if entry.Mode.IsDir() {
		return fmt.Errorf("cannot mutate directory %q", path)
	}
	entry.Mutated = true
	entry.FinalHash = fsEntry.Hash
	entry.Size = fsEntry.Size
	r.Entries[path] = entry
	return nil
}

func (r *Report) sanitizePath(path string, isDir bool) (string, error) {
	if !strings.HasPrefix(path, r.Root) {
		return "", fmt.Errorf("%q outside of root %q", path, r.Root)
	}
	relPath := filepath.Clean("/" + strings.TrimPrefix(path, r.Root))
	if isDir {
		relPath = relPath + "/"
	}
	return relPath, nil
}
