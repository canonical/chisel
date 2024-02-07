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
	Path: "/root/exampleDir",
	Mode: fs.ModeDir | 0654,
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

type sliceAndInfo struct {
	info  fsutil.Info
	slice *setup.Slice
}

var reportTests = []struct {
	summary      string
	sliceAndInfo []sliceAndInfo
	// indexed by path.
	expected map[string]slicer.ReportEntry
	// error after processing the last sliceAndInfo item.
	err string
}{{
	summary:      "Regular directory",
	sliceAndInfo: []sliceAndInfo{{info: sampleDir, slice: oneSlice}},
	expected: map[string]slicer.ReportEntry{
		"/root/exampleDir": {
			Path:   "/root/exampleDir",
			Mode:   fs.ModeDir | 0654,
			Slices: map[*setup.Slice]bool{oneSlice: true},
			Link:   "",
		}},
}, {
	summary: "Regular directory added by several slices",
	sliceAndInfo: []sliceAndInfo{
		{info: sampleDir, slice: oneSlice},
		{info: sampleDir, slice: otherSlice},
	},
	expected: map[string]slicer.ReportEntry{
		"/root/exampleDir": {
			Path:   "/root/exampleDir",
			Mode:   fs.ModeDir | 0654,
			Slices: map[*setup.Slice]bool{oneSlice: true, otherSlice: true},
			Link:   "",
		}},
}, {
	summary:      "Regular file",
	sliceAndInfo: []sliceAndInfo{{info: sampleFile, slice: oneSlice}},
	expected: map[string]slicer.ReportEntry{
		"/root/exampleFile": {
			Path:   "/root/exampleFile",
			Mode:   0777,
			Hash:   "exampleFile_hash",
			Size:   5678,
			Slices: map[*setup.Slice]bool{oneSlice: true},
			Link:   "",
		}},
}, {
	summary:      "Regular file link",
	sliceAndInfo: []sliceAndInfo{{info: sampleLink, slice: oneSlice}},
	expected: map[string]slicer.ReportEntry{
		"/root/exampleLink": {
			Path:   "/root/exampleLink",
			Mode:   0777,
			Hash:   "exampleFile_hash",
			Size:   5678,
			Slices: map[*setup.Slice]bool{oneSlice: true},
			Link:   "/root/exampleFile",
		}},
}, {
	summary: "Several entries",
	sliceAndInfo: []sliceAndInfo{
		{info: sampleDir, slice: oneSlice},
		{info: sampleFile, slice: otherSlice},
	},
	expected: map[string]slicer.ReportEntry{
		"/root/exampleDir": {
			Path:   "/root/exampleDir",
			Mode:   fs.ModeDir | 0654,
			Slices: map[*setup.Slice]bool{oneSlice: true},
			Link:   "",
		},
		"/root/exampleFile": {
			Path:   "/root/exampleFile",
			Mode:   0777,
			Hash:   "exampleFile_hash",
			Size:   5678,
			Slices: map[*setup.Slice]bool{otherSlice: true},
			Link:   "",
		}},
}, {
	summary: "Same path, identical files",
	sliceAndInfo: []sliceAndInfo{
		{info: sampleFile, slice: oneSlice},
		{info: sampleFile, slice: oneSlice},
	},
	expected: map[string]slicer.ReportEntry{
		"/root/exampleFile": {
			Path:   "/root/exampleFile",
			Mode:   0777,
			Hash:   "exampleFile_hash",
			Size:   5678,
			Slices: map[*setup.Slice]bool{oneSlice: true},
			Link:   "",
		}},
}, {
	summary: "Error for same path distinct mode",
	sliceAndInfo: []sliceAndInfo{
		{info: sampleFile, slice: oneSlice},
		{info: fsutil.Info{
			Path: sampleFile.Path,
			Mode: 0,
			Hash: sampleFile.Hash,
			Size: sampleFile.Size,
			Link: sampleFile.Link,
		}, slice: oneSlice},
	},
	err: `internal error: cannot add conflicting data for path "/root/exampleFile"`,
}, {
	summary: "Error for same path distinct hash",
	sliceAndInfo: []sliceAndInfo{
		{info: sampleFile, slice: oneSlice},
		{info: fsutil.Info{
			Path: sampleFile.Path,
			Mode: sampleFile.Mode,
			Hash: "distinct hash",
			Size: sampleFile.Size,
			Link: sampleFile.Link,
		}, slice: oneSlice},
	},
	err: `internal error: cannot add conflicting data for path "/root/exampleFile"`,
}, {
	summary: "Error for same path distinct size",
	sliceAndInfo: []sliceAndInfo{
		{info: sampleFile, slice: oneSlice},
		{info: fsutil.Info{
			Path: sampleFile.Path,
			Mode: sampleFile.Mode,
			Hash: sampleFile.Hash,
			Size: 0,
			Link: sampleFile.Link,
		}, slice: oneSlice},
	},
	err: `internal error: cannot add conflicting data for path "/root/exampleFile"`,
}, {
	summary: "Error for same path distinct link",
	sliceAndInfo: []sliceAndInfo{
		{info: sampleFile, slice: oneSlice},
		{info: fsutil.Info{
			Path: sampleFile.Path,
			Mode: sampleFile.Mode,
			Hash: sampleFile.Hash,
			Size: sampleFile.Size,
			Link: "distinct link",
		}, slice: oneSlice},
	},
	err: `internal error: cannot add conflicting data for path "/root/exampleFile"`,
}}

func (s *S) TestReportAdd(c *C) {
	for _, test := range reportTests {
		report := slicer.NewReport("/root")
		var err error
		for _, si := range test.sliceAndInfo {
			err = report.Add(si.slice, &si.info)
		}
		if test.err != "" {
			c.Assert(err, ErrorMatches, test.err)
			continue
		}
		c.Assert(report.Entries, DeepEquals, test.expected, Commentf(test.summary))
	}
}
