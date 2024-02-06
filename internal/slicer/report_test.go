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
	summary   string
	sliceInfo []sliceAndInfo
	// indexed by path.
	expected map[string]slicer.ReportEntry
	// error after processing the last sliceInfo item.
	err string
}{{
	summary:   "Regular directory",
	sliceInfo: []sliceAndInfo{{info: sampleDir, slice: oneSlice}},
	expected: map[string]slicer.ReportEntry{
		"/root/example": {
			Path:   "/root/example",
			Mode:   fs.ModeDir | 0654,
			Slices: []*setup.Slice{oneSlice},
			Link:   "",
		}},
}, {
	summary: "Regular directory added by several slices",
	sliceInfo: []sliceAndInfo{
		{info: sampleDir, slice: oneSlice},
		{info: sampleDir, slice: otherSlice},
	},
	expected: map[string]slicer.ReportEntry{
		"/root/example": {
			Path:   "/root/example",
			Mode:   fs.ModeDir | 0654,
			Slices: []*setup.Slice{oneSlice, otherSlice},
			Link:   "",
		}},
}, {
	summary:   "Regular file",
	sliceInfo: []sliceAndInfo{{info: sampleFile, slice: oneSlice}},
	expected: map[string]slicer.ReportEntry{
		"/root/exampleFile": {
			Path:   "/root/exampleFile",
			Mode:   0777,
			Hash:   "exampleFile_hash",
			Size:   5678,
			Slices: []*setup.Slice{oneSlice},
			Link:   "",
		}},
}, {
	summary:   "Regular file link",
	sliceInfo: []sliceAndInfo{{info: sampleLink, slice: oneSlice}},
	expected: map[string]slicer.ReportEntry{
		"/root/exampleLink": {
			Path:   "/root/exampleLink",
			Mode:   0777,
			Hash:   "exampleFile_hash",
			Size:   5678,
			Slices: []*setup.Slice{oneSlice},
			Link:   "/root/exampleFile",
		}},
}, {
	summary: "Several entries",
	sliceInfo: []sliceAndInfo{
		{info: sampleDir, slice: oneSlice},
		{info: sampleFile, slice: otherSlice},
	},
	expected: map[string]slicer.ReportEntry{
		"/root/example": {
			Path:   "/root/example",
			Mode:   fs.ModeDir | 0654,
			Slices: []*setup.Slice{oneSlice},
			Link:   "",
		},
		"/root/exampleFile": {
			Path:   "/root/exampleFile",
			Mode:   0777,
			Hash:   "exampleFile_hash",
			Size:   5678,
			Slices: []*setup.Slice{otherSlice},
			Link:   "",
		}},
}, {
	summary: "Same path, identical files",
	sliceInfo: []sliceAndInfo{
		{info: sampleFile, slice: oneSlice},
		{info: sampleFile, slice: oneSlice},
	},
	expected: map[string]slicer.ReportEntry{
		"/root/exampleFile": {
			Path:   "/root/exampleFile",
			Mode:   0777,
			Hash:   "exampleFile_hash",
			Size:   5678,
			Slices: []*setup.Slice{oneSlice},
			Link:   "",
		}},
}, {
	summary: "Error for same path distinct files",
	sliceInfo: []sliceAndInfo{
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
}}

func (s *S) TestReportAdd(c *C) {
	for _, test := range reportTests {
		report := slicer.NewReport("/root")
		var err error
		for _, si := range test.sliceInfo {
			err = report.Add(si.slice, &si.info)
		}
		if test.err != "" {
			c.Assert(err, ErrorMatches, test.err)
			continue
		}
		c.Assert(report.Entries, DeepEquals, test.expected, Commentf(test.summary))
	}
}
