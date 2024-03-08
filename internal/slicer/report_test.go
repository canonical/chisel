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

var sampleDir = fsutil.Entry{
	Path: "/base/exampleDir",
	Mode: fs.ModeDir | 0654,
	Link: "",
}

var sampleFile = fsutil.Entry{
	Path: "/base/exampleFile",
	Mode: 0777,
	Hash: "exampleFile_hash",
	Size: 5678,
	Link: "",
}

var sampleLink = fsutil.Entry{
	Path: "/base/exampleLink",
	Mode: 0777,
	Hash: "exampleFile_hash",
	Size: 5678,
	Link: "/base/exampleFile",
}

type sliceAndEntry struct {
	entry fsutil.Entry
	slice *setup.Slice
}

var reportTests = []struct {
	summary string
	add     []sliceAndEntry
	// indexed by path.
	expected map[string]slicer.ReportEntry
	// error after adding the last [sliceAndEntry].
	err string
}{{
	summary: "Regular directory",
	add:     []sliceAndEntry{{entry: sampleDir, slice: oneSlice}},
	expected: map[string]slicer.ReportEntry{
		"/exampleDir": {
			Path:   "/exampleDir",
			Mode:   fs.ModeDir | 0654,
			Slices: map[*setup.Slice]bool{oneSlice: true},
			Link:   "",
		}},
}, {
	summary: "Regular directory added by several slices",
	add: []sliceAndEntry{
		{entry: sampleDir, slice: oneSlice},
		{entry: sampleDir, slice: otherSlice},
	},
	expected: map[string]slicer.ReportEntry{
		"/exampleDir": {
			Path:   "/exampleDir",
			Mode:   fs.ModeDir | 0654,
			Slices: map[*setup.Slice]bool{oneSlice: true, otherSlice: true},
			Link:   "",
		}},
}, {
	summary: "Regular file",
	add:     []sliceAndEntry{{entry: sampleFile, slice: oneSlice}},
	expected: map[string]slicer.ReportEntry{
		"/exampleFile": {
			Path:   "/exampleFile",
			Mode:   0777,
			Hash:   "exampleFile_hash",
			Size:   5678,
			Slices: map[*setup.Slice]bool{oneSlice: true},
			Link:   "",
		}},
}, {
	summary: "Regular file link",
	add:     []sliceAndEntry{{entry: sampleLink, slice: oneSlice}},
	expected: map[string]slicer.ReportEntry{
		"/exampleLink": {
			Path:   "/exampleLink",
			Mode:   0777,
			Hash:   "exampleFile_hash",
			Size:   5678,
			Slices: map[*setup.Slice]bool{oneSlice: true},
			Link:   "/base/exampleFile",
		}},
}, {
	summary: "Several entries",
	add: []sliceAndEntry{
		{entry: sampleDir, slice: oneSlice},
		{entry: sampleFile, slice: otherSlice},
	},
	expected: map[string]slicer.ReportEntry{
		"/exampleDir": {
			Path:   "/exampleDir",
			Mode:   fs.ModeDir | 0654,
			Slices: map[*setup.Slice]bool{oneSlice: true},
			Link:   "",
		},
		"/exampleFile": {
			Path:   "/exampleFile",
			Mode:   0777,
			Hash:   "exampleFile_hash",
			Size:   5678,
			Slices: map[*setup.Slice]bool{otherSlice: true},
			Link:   "",
		}},
}, {
	summary: "Same path, identical files",
	add: []sliceAndEntry{
		{entry: sampleFile, slice: oneSlice},
		{entry: sampleFile, slice: oneSlice},
	},
	expected: map[string]slicer.ReportEntry{
		"/exampleFile": {
			Path:   "/exampleFile",
			Mode:   0777,
			Hash:   "exampleFile_hash",
			Size:   5678,
			Slices: map[*setup.Slice]bool{oneSlice: true},
			Link:   "",
		}},
}, {
	summary: "Error for same path distinct mode",
	add: []sliceAndEntry{
		{entry: sampleFile, slice: oneSlice},
		{entry: fsutil.Entry{
			Path: sampleFile.Path,
			Mode: 0,
			Hash: sampleFile.Hash,
			Size: sampleFile.Size,
			Link: sampleFile.Link,
		}, slice: oneSlice},
	},
	err: `path "/exampleFile" reported twice with diverging mode: "----------" != "-rwxrwxrwx"`,
}, {
	summary: "Error for same path distinct hash",
	add: []sliceAndEntry{
		{entry: sampleFile, slice: oneSlice},
		{entry: fsutil.Entry{
			Path: sampleFile.Path,
			Mode: sampleFile.Mode,
			Hash: "distinct hash",
			Size: sampleFile.Size,
			Link: sampleFile.Link,
		}, slice: oneSlice},
	},
	err: `path "/exampleFile" reported twice with diverging hash: "distinct hash" != "exampleFile_hash"`,
}, {
	summary: "Error for same path distinct size",
	add: []sliceAndEntry{
		{entry: sampleFile, slice: oneSlice},
		{entry: fsutil.Entry{
			Path: sampleFile.Path,
			Mode: sampleFile.Mode,
			Hash: sampleFile.Hash,
			Size: 0,
			Link: sampleFile.Link,
		}, slice: oneSlice},
	},
	err: `path "/exampleFile" reported twice with diverging size: 0 != 5678`,
}, {
	summary: "Error for same path distinct link",
	add: []sliceAndEntry{
		{entry: sampleFile, slice: oneSlice},
		{entry: fsutil.Entry{
			Path: sampleFile.Path,
			Mode: sampleFile.Mode,
			Hash: sampleFile.Hash,
			Size: sampleFile.Size,
			Link: "distinct link",
		}, slice: oneSlice},
	},
	err: `path "/exampleFile" reported twice with diverging link: "distinct link" != ""`,
}}

func (s *S) TestReportAdd(c *C) {
	for _, test := range reportTests {
		report := slicer.NewReport("/base/")
		var err error
		for _, si := range test.add {
			err = report.Add(si.slice, &si.entry)
		}
		if test.err != "" {
			c.Assert(err, ErrorMatches, test.err)
			continue
		}
		c.Assert(err, IsNil)
		c.Assert(report.Entries, DeepEquals, test.expected, Commentf(test.summary))
	}
}
