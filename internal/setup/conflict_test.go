package setup_test

import (
	"slices"
	"strings"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/setup"
)

func (s *S) TestPathToSegments(c *C) {
	tests := []struct {
		path     string
		segments []setup.PathSegment
		err      string
	}{{
		path: "/foo/bar",
		segments: []setup.PathSegment{
			{Text: "/"},
			{Text: "foo/"},
			{Text: "bar"},
			{Text: ""},
		},
	}, {
		path: "/foo/",
		segments: []setup.PathSegment{
			{Text: "/"},
			{Text: "foo/"},
			{Text: ""},
		},
	}, {
		path: "/",
		segments: []setup.PathSegment{
			{Text: "/"},
			{Text: ""},
		},
	}, {
		path: "/*",
		segments: []setup.PathSegment{
			{Text: "/"},
			{Text: "*", HasGlob: true},
			{Text: ""},
		},
	}, {
		path: "/*/",
		segments: []setup.PathSegment{
			{Text: "/"},
			{Text: "*/", HasGlob: true},
			{Text: ""},
		},
	}, {
		path: "/**",
		segments: []setup.PathSegment{
			{Text: "/"},
			{Text: "**", HasGlob: true, HasDoubleGlob: true},
			{Text: ""},
		},
	}, {
		path: "/**/bar",
		segments: []setup.PathSegment{
			{Text: "/"},
			{Text: "**/bar", HasGlob: true, HasDoubleGlob: true},
			{Text: ""},
		},
	}, {
		path: "/foo*/bar",
		segments: []setup.PathSegment{
			{Text: "/"},
			{Text: "foo*/", HasGlob: true},
			{Text: "bar"},
			{Text: ""},
		},
	}, {
		path: "/foo?/bar",
		segments: []setup.PathSegment{
			{Text: "/"},
			{Text: "foo?/", HasGlob: true},
			{Text: "bar"},
			{Text: ""},
		},
	}, {
		path: "/f*oo/f**/bar",
		segments: []setup.PathSegment{
			{Text: "/"},
			{Text: "f*oo/", HasGlob: true},
			{Text: "f**/bar", HasGlob: true, HasDoubleGlob: true},
			{Text: ""},
		},
	}, {
		path: "/foo**/bar/baz",
		segments: []setup.PathSegment{
			{Text: "/"},
			{Text: "foo**/bar/baz", HasGlob: true, HasDoubleGlob: true},
			{Text: ""},
		},
	}, {
		path: "foo/bar",
		err:  `internal error: path does not start with '/'`,
	}}

	for _, test := range tests {
		segments, err := setup.PathToSegments(test.path)
		if test.err != "" {
			c.Assert(err, ErrorMatches, test.err)
			continue
		}
		c.Assert(err, IsNil)
		c.Assert(segments, DeepEquals, test.segments)
	}
}

func (s *S) TestConflictTree(c *C) {
	slicePath := &setup.Slice{
		Package: "pkg1",
		Name:    "path",
		Contents: map[string]setup.PathInfo{
			"/a/*/b": {Kind: setup.CopyPath},
		},
	}
	sliceGlob := &setup.Slice{
		Package: "pkg2",
		Name:    "glob",
		Contents: map[string]setup.PathInfo{
			"/a/*": {Kind: setup.GlobPath},
		},
	}

	pathInfo := setup.PathSegmentSlice{
		Slice:     slicePath,
		PathInfo:  setup.PathInfo{Kind: setup.CopyPath},
		WholePath: "/a/*/b",
	}
	globInfo := setup.PathSegmentSlice{
		Slice:     sliceGlob,
		PathInfo:  setup.PathInfo{Kind: setup.GlobPath},
		WholePath: "/a/*",
	}

	tree := setup.NewConflictTree(map[string][]*setup.Slice{
		"/a/*/b": {slicePath},
		"/a/*":   {sliceGlob},
	})
	err := tree.HasConflict()
	c.Assert(err, IsNil)

	expected := &setup.PathNode{
		Segment: setup.PathSegment{Text: "/"},
		Children: map[string]*setup.PathNode{
			"a/": {
				Segment: setup.PathSegment{Text: "a/"},
				Slices:  []*setup.PathSegmentSlice{&pathInfo, &globInfo},
				Children: map[string]*setup.PathNode{
					"*": {
						Segment: setup.PathSegment{Text: "*", HasGlob: true},
						Slices:  []*setup.PathSegmentSlice{&globInfo},
						Children: map[string]*setup.PathNode{
							"": {
								Segment: setup.PathSegment{Text: ""},
								Slices:  []*setup.PathSegmentSlice{&globInfo},
							},
						},
					},
					"*/": {
						Segment: setup.PathSegment{Text: "*/", HasGlob: true},
						Slices:  []*setup.PathSegmentSlice{&pathInfo},
						Children: map[string]*setup.PathNode{
							"b": {
								Segment: setup.PathSegment{Text: "b"},
								Slices:  []*setup.PathSegmentSlice{&pathInfo},
								Children: map[string]*setup.PathNode{
									"": {
										Segment: setup.PathSegment{Text: ""},
										Slices:  []*setup.PathSegmentSlice{&pathInfo},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	assertTreeEquals(c, tree.Root, expected)
}

func assertTreeEquals(c *C, obtained, expected *setup.PathNode) {
	c.Assert(obtained.Segment, DeepEquals, expected.Segment)

	slices.SortFunc(obtained.Slices, func(a, b *setup.PathSegmentSlice) int {
		return strings.Compare(a.Slice.String(), b.Slice.String())
	})
	slices.SortFunc(expected.Slices, func(a, b *setup.PathSegmentSlice) int {
		return strings.Compare(a.Slice.String(), b.Slice.String())
	})
	c.Assert(obtained.Slices, DeepEquals, expected.Slices)

	c.Assert(len(obtained.Children), Equals, len(expected.Children))
	for name, expectedChild := range expected.Children {
		obtainedChild, ok := obtained.Children[name]
		c.Assert(ok, Equals, true)
		assertTreeEquals(c, obtainedChild, expectedChild)
	}
}
