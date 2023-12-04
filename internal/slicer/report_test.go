package slicer_test

import (
	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/slicer"
	. "gopkg.in/check.v1"
	"io/fs"
)

var mySlice = &setup.Slice{
	Package:   "base-files",
	Name:      "my-slice",
	Essential: nil,
	Contents:  nil,
	Scripts:   setup.SliceScripts{},
}

var otherSlice = &setup.Slice{
	Package:   "base-files",
	Name:      "other-slice",
	Essential: nil,
	Contents:  nil,
	Scripts:   setup.SliceScripts{},
}

var sampleDir = fsutil.FileInfo{
	Path: "/root/example",
	Mode: fs.ModeDir | 0654,
	Hash: "example_hash",
	Size: 1234,
	Link: "",
}

var sampleFile = fsutil.FileInfo{
	Path: "/root/exampleFile",
	Mode: 0777,
	Hash: "exampleFile_hash",
	Size: 5678,
	Link: "",
}

var sampleLink = fsutil.FileInfo{
	Path: "/root/exampleLink",
	Mode: 0777,
	Hash: "exampleFile_hash",
	Size: 5678,
	Link: "/root/exampleFile",
}

var testFiles = []struct {
	summary  string
	info     fsutil.FileInfo
	slice    *setup.Slice
	expected slicer.FileReport
	err      string
}{{
	summary: "Regular directory",
	info:    sampleDir,
	slice:   mySlice,
	expected: slicer.FileReport{
		Path:    "/root/example",
		Mode:    fs.ModeDir | 0654,
		Hash:    "example_hash",
		Size:    1234,
		Mutable: false,
		Slices:  map[*setup.Slice]bool{mySlice: true},
		Link:    "",
	},
}, {
	summary: "Regular directory added by several slices",
	info:    sampleDir,
	slice:   otherSlice,
	expected: slicer.FileReport{
		Path:    "/root/example",
		Mode:    fs.ModeDir | 0654,
		Hash:    "example_hash",
		Size:    1234,
		Mutable: false,
		Slices:  map[*setup.Slice]bool{mySlice: true, otherSlice: true},
		Link:    "",
	},
}, {
	summary: "Regular file",
	info:    sampleFile,
	slice:   mySlice,
	expected: slicer.FileReport{
		Path:    "/root/exampleFile",
		Mode:    0777,
		Hash:    "exampleFile_hash",
		Size:    5678,
		Mutable: false,
		Slices:  map[*setup.Slice]bool{mySlice: true},
		Link:    "",
	},
}, {
	summary: "Regular file, error when created by several slices",
	info:    sampleFile,
	slice:   otherSlice,
	err:     "slices base-files_other-slice and base-files_my-slice attempted to create the same file: /root/exampleFile",
}, {
	summary: "Regular file link",
	info:    sampleLink,
	slice:   mySlice,
	expected: slicer.FileReport{
		Path:    "/root/exampleLink",
		Mode:    0777,
		Hash:    "exampleFile_hash",
		Size:    5678,
		Mutable: false,
		Slices:  map[*setup.Slice]bool{mySlice: true},
		Link:    "/root/exampleFile",
	},
}}

func (s *S) TestReportAddFile(c *C) {
	report := slicer.NewReport("/root")
	for _, test := range testFiles {
		err := report.AddFile(test.slice, test.info)
		if test.err != "" {
			c.Assert(err, ErrorMatches, test.err, Commentf(test.summary))
		} else {
			c.Assert(err, IsNil)
			c.Assert(report.Files[test.info.Path], DeepEquals, test.expected, Commentf(test.summary))
		}
	}
}
