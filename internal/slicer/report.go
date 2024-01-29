package slicer

import (
	"fmt"
	"io/fs"

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
		// If two different slices attempt to create the same regular file,
		// throw out an error.
		if !info.Mode.IsDir() {
			var existingSlice *setup.Slice
			for s := range info.Slices {
				existingSlice = s
				break
			}
			return fmt.Errorf("slices %s and %s attempted to create the same entry: %s",
				slice, existingSlice, info.Path)
		}
		// If several slices create the same directory we report all of them.
		info.Slices[slice] = true
		r.Entries[entry.Path] = info
	} else {
		r.Entries[entry.Path] = ReportEntry{
			Path:   entry.Path,
			Mode:   entry.Mode,
			Hash:   entry.Hash,
			Size:   entry.Size,
			Slices: map[*setup.Slice]bool{slice: true},
			Link:   entry.Link,
		}
	}
	return nil
}
