package slicer_test

import (
	"io/fs"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/slicer"
)

var oneSlice = &setup.Slice{
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

var sampleDir = fsutil.Info{
	Path: "/root/example",
	Mode: fs.ModeDir | 0654,
	Hash: "example_hash",
	Size: 1234,
	Link: "",
}

var sampleFile = fsutil.Info{
	Path: "/root/exampleFile",
	Mode: 0777,
	Hash: "exampleFile_hash",
	Size: 5678,
	Link: "",
}

var sampleLink = fsutil.Info{
	Path: "/root/exampleLink",
	Mode: 0777,
	Hash: "exampleFile_hash",
	Size: 5678,
	Link: "/root/exampleFile",
}

var testFiles = []struct {
	summary  string
	info     fsutil.Info
	slice    *setup.Slice
	expected slicer.ReportEntry
	err      string
}{{
	summary: "Regular directory",
	info:    sampleDir,
	slice:   oneSlice,
	expected: slicer.ReportEntry{
		Path:   "/root/example",
		Mode:   fs.ModeDir | 0654,
		Hash:   "example_hash",
		Size:   1234,
		Slices: []*setup.Slice{oneSlice},
		Link:   "",
	},
}, {
	summary: "Regular directory added by several slices",
	info:    sampleDir,
	slice:   otherSlice,
	expected: slicer.ReportEntry{
		Path:   "/root/example",
		Mode:   fs.ModeDir | 0654,
		Hash:   "example_hash",
		Size:   1234,
		Slices: []*setup.Slice{oneSlice, otherSlice},
		Link:   "",
	},
}, {
	summary: "Regular file",
	info:    sampleFile,
	slice:   oneSlice,
	expected: slicer.ReportEntry{
		Path:   "/root/exampleFile",
		Mode:   0777,
		Hash:   "exampleFile_hash",
		Size:   5678,
		Slices: []*setup.Slice{oneSlice},
		Link:   "",
	},
}, {
	summary: "Regular file link",
	info:    sampleLink,
	slice:   oneSlice,
	expected: slicer.ReportEntry{
		Path:   "/root/exampleLink",
		Mode:   0777,
		Hash:   "exampleFile_hash",
		Size:   5678,
		Slices: []*setup.Slice{oneSlice},
		Link:   "/root/exampleFile",
	},
}}

func (s *S) TestReportAddFile(c *C) {
	report := slicer.NewReport("/root")
	for _, test := range testFiles {
		err := report.AddEntry(test.slice, test.info)
		if test.err != "" {
			c.Assert(err, ErrorMatches, test.err, Commentf(test.summary))
		} else {
			c.Assert(err, IsNil)
			c.Assert(report.Entries[test.info.Path], DeepEquals, test.expected, Commentf(test.summary))
		}
	}
}
