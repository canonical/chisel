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
	Path   string
	Mode   fs.FileMode
	Hash   string
	Size   int
	Slices map[*setup.Slice]bool
	Link   string
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
		Root:    root,
		Entries: make(map[string]ReportEntry),
	}
}

func (r *Report) Add(slice *setup.Slice, fsEntry *fsutil.Entry) error {
	if !strings.HasPrefix(fsEntry.Path, r.Root) {
		return fmt.Errorf("cannot add path %q outside of root %q", fsEntry.Path, r.Root)
	}
	relPath := filepath.Clean("/" + strings.TrimPrefix(fsEntry.Path, r.Root))

	if entry, ok := r.Entries[relPath]; ok {
		if fsEntry.Mode != entry.Mode {
			return fmt.Errorf("path %q reported twice with diverging mode: %q != %q", relPath, fsEntry.Mode, entry.Mode)
		} else if fsEntry.Link != entry.Link {
			return fmt.Errorf("path %q reported twice with diverging link: %q != %q", relPath, fsEntry.Link, entry.Link)
		} else if fsEntry.Size != entry.Size {
			return fmt.Errorf("path %q reported twice with diverging size: %d != %d", relPath, fsEntry.Size, entry.Size)
		} else if fsEntry.Hash != entry.Hash {
			return fmt.Errorf("path %q reported twice with diverging hash: %q != %q", relPath, fsEntry.Hash, entry.Hash)
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
