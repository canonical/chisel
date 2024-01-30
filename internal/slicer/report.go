package slicer

import (
	"io/fs"

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

// Report holds the information about files created when slicing packages.
type Report struct {
	Root string
	// map indexed by path.
	Entries map[string]ReportEntry
}

func NewReport(root string) *Report {
	return &Report{Entries: make(map[string]ReportEntry), Root: root}
}

func (r *Report) AddEntry(slice *setup.Slice, entry fsutil.Info) error {
	if info, ok := r.Entries[entry.Path]; ok {
		// If several slices create the same directory we report all of them.
		// Two slices cannot try to create the same regular file because of the
		// validation Chisel does against the slice definitions.
		info.Slices = append(info.Slices, slice)
		r.Entries[entry.Path] = info
	} else {
		r.Entries[entry.Path] = ReportEntry{
			Path:   entry.Path,
			Mode:   entry.Mode,
			Hash:   entry.Hash,
			Size:   entry.Size,
			Slices: []*setup.Slice{slice},
			Link:   entry.Link,
		}
	}
	return nil
}
