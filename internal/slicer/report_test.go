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

type reportTestEntry struct {
	info  fsutil.Info
	slice *setup.Slice
}

var reportTests = []struct {
	summary  string
	entries  []reportTestEntry
	expected []slicer.ReportEntry
}{{
	summary: "Regular directory",
	entries: []reportTestEntry{{info: sampleDir, slice: oneSlice}},
	expected: []slicer.ReportEntry{{
		Path:   "/root/example",
		Mode:   fs.ModeDir | 0654,
		Hash:   "example_hash",
		Size:   1234,
		Slices: []*setup.Slice{oneSlice},
		Link:   "",
	}},
}, {
	summary: "Regular directory added by several slices",
	entries: []reportTestEntry{
		{info: sampleDir, slice: oneSlice},
		{info: sampleDir, slice: otherSlice},
	},
	expected: []slicer.ReportEntry{{
		Path:   "/root/example",
		Mode:   fs.ModeDir | 0654,
		Hash:   "example_hash",
		Size:   1234,
		Slices: []*setup.Slice{oneSlice, otherSlice},
		Link:   "",
	}},
}, {
	summary: "Regular file",
	entries: []reportTestEntry{{info: sampleFile, slice: oneSlice}},
	expected: []slicer.ReportEntry{{
		Path:   "/root/exampleFile",
		Mode:   0777,
		Hash:   "exampleFile_hash",
		Size:   5678,
		Slices: []*setup.Slice{oneSlice},
		Link:   "",
	}},
}, {
	summary: "Regular file link",
	entries: []reportTestEntry{{info: sampleLink, slice: oneSlice}},
	expected: []slicer.ReportEntry{{
		Path:   "/root/exampleLink",
		Mode:   0777,
		Hash:   "exampleFile_hash",
		Size:   5678,
		Slices: []*setup.Slice{oneSlice},
		Link:   "/root/exampleFile",
	}},
}, {
	summary: "Several entries",
	entries: []reportTestEntry{
		{info: sampleDir, slice: oneSlice},
		{info: sampleFile, slice: otherSlice},
	},
	expected: []slicer.ReportEntry{{
		Path:   "/root/example",
		Mode:   fs.ModeDir | 0654,
		Hash:   "example_hash",
		Size:   1234,
		Slices: []*setup.Slice{oneSlice},
		Link:   "",
	}, {
		Path:   "/root/exampleFile",
		Mode:   0777,
		Hash:   "exampleFile_hash",
		Size:   5678,
		Slices: []*setup.Slice{otherSlice},
		Link:   "",
	}},
}}

func (s *S) TestReportAddFile(c *C) {
	for _, test := range reportTests {
		report := slicer.NewReport("/root")
		for _, entry := range test.entries {
			report.AddEntry(entry.slice, entry.info)
		}
		reportEntries := []slicer.ReportEntry{}
		for _, entry := range report.Entries {
			reportEntries = append(reportEntries, entry)
		}
		c.Assert(reportEntries, DeepEquals, test.expected, Commentf(test.summary))
	}
}
