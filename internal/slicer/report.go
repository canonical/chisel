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

func (r *Report) Add(slice *setup.Slice, info *fsutil.Info) error {
	if !strings.HasPrefix(info.Path, r.Root) {
		return fmt.Errorf("internal error: cannot add path %q outside out root %q", info.Path, r.Root)
	}
	relPath := filepath.Clean("/" + strings.TrimPrefix(info.Path, r.Root))

	if entry, ok := r.Entries[relPath]; ok {
		if info.Mode != entry.Mode || info.Link != entry.Link ||
			info.Size != entry.Size || info.Hash != entry.Hash {
			return fmt.Errorf("internal error: cannot add conflicting data for path %q", relPath)
		}
		entry.Slices[slice] = true
		r.Entries[relPath] = entry
	} else {
		r.Entries[relPath] = ReportEntry{
			Path:   relPath,
			Mode:   info.Mode,
			Hash:   info.Hash,
			Size:   info.Size,
			Slices: map[*setup.Slice]bool{slice: true},
			Link:   info.Link,
		}
	}
	return nil
}
