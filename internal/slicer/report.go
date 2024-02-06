package slicer

import (
	"fmt"
	"io/fs"
	"slices"

	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/setup"
)

type ReportEntry struct {
	Path   string
	Mode   fs.FileMode
	Hash   string
	Size   int
	Slices []*setup.Slice
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
	return &Report{Entries: make(map[string]ReportEntry), Root: root}
}

func (r *Report) Add(slice *setup.Slice, info *fsutil.Info) error {
	if entry, ok := r.Entries[info.Path]; ok {
		if info.Mode != entry.Mode || info.Link != entry.Link || info.Size !=
			entry.Size || info.Hash != entry.Hash {
			return fmt.Errorf("internal error: cannot add conflicting data for path %s", info.Path)
		}
		if !slices.Contains(entry.Slices, slice) {
			entry.Slices = append(entry.Slices, slice)
		}
		r.Entries[info.Path] = entry
	} else {
		r.Entries[info.Path] = ReportEntry{
			Path:   info.Path,
			Mode:   info.Mode,
			Hash:   info.Hash,
			Size:   info.Size,
			Slices: []*setup.Slice{slice},
			Link:   info.Link,
		}
	}
	return nil
}
