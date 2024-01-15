package slicer

import (
	"fmt"
	"io/fs"
	"sync"

	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/setup"
)

type FileReport struct {
	Path    string
	Mode    fs.FileMode
	Hash    string
	Size    uint
	Mutable bool
	Slices  map[*setup.Slice]bool
	Link    string
}

// Report holds the information about files created when slicing packages.
type Report struct {
	Root string
	// map indexed by path.
	Files map[string]FileReport
	mutex *sync.Mutex
}

func NewReport(root string) *Report {
	return &Report{Files: make(map[string]FileReport), Root: root, mutex: &sync.Mutex{}}
}

func (r *Report) AddFile(slice *setup.Slice, file fsutil.FileInfo) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if fr, ok := r.Files[file.Path]; ok {
		if !fr.Mode.IsDir() {
			var existingSlice *setup.Slice
			for s := range fr.Slices {
				existingSlice = s
				break
			}
			return fmt.Errorf("slices %s and %s attempted to create the same file: %s",
				slice, existingSlice, fr.Path)
		}
		fr.Slices[slice] = true
		r.Files[file.Path] = fr
	} else {
		r.Files[file.Path] = FileReport{
			Path:    file.Path,
			Mode:    file.Mode,
			Hash:    file.Hash,
			Size:    file.Size,
			Mutable: false,
			Slices:  map[*setup.Slice]bool{slice: true},
			Link:    file.Link,
		}
	}
	return nil
}