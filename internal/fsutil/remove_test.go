package fsutil_test

import (
	"os"
	"path/filepath"
	"syscall"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/testutil"
)

type removeTest struct {
	summary string
	options fsutil.RemoveOptions
	hackopt func(c *C, dir string, opts *fsutil.RemoveOptions)
	result  map[string]string
	error   string
}

var removeTests = []removeTest{{
	summary: "Remove a file",
	options: fsutil.RemoveOptions{
		Path: "file",
	},
	hackopt: func(c *C, dir string, opts *fsutil.RemoveOptions) {
		c.Assert(os.WriteFile(filepath.Join(dir, "file"), []byte("data"), 0o644), IsNil)
	},
	result: map[string]string{},
}, {
	summary: "Remove a non-existent file",
	options: fsutil.RemoveOptions{
		Path: "file",
	},
	result: map[string]string{},
}, {
	summary: "Remove a non-existent directory",
	options: fsutil.RemoveOptions{
		Path: "foo/",
	},
	result: map[string]string{},
}, {
	summary: "Remove an empty directory",
	options: fsutil.RemoveOptions{
		Path: "foo/bar",
	},
	hackopt: func(c *C, dir string, opts *fsutil.RemoveOptions) {
		c.Assert(os.MkdirAll(filepath.Join(dir, "foo/bar"), 0o755), IsNil)
	},
	result: map[string]string{
		"/foo/": "dir 0755",
	},
}, {
	summary: "Do not remove non-empty directory",
	options: fsutil.RemoveOptions{
		Path: "foo/",
	},
	hackopt: func(c *C, dir string, opts *fsutil.RemoveOptions) {
		c.Assert(os.MkdirAll(filepath.Join(dir, "foo"), 0o755), IsNil)
		c.Assert(os.WriteFile(filepath.Join(dir, "foo/file"), []byte("data"), 0o644), IsNil)
	},
	result: map[string]string{
		"/foo/":     "dir 0755",
		"/foo/file": "file 0644 3a6eb079",
	},
}, {
	summary: "Remove a symlink and not the target",
	options: fsutil.RemoveOptions{
		Path: "bar",
	},
	hackopt: func(c *C, dir string, opts *fsutil.RemoveOptions) {
		c.Assert(os.WriteFile(filepath.Join(dir, "foo"), []byte("data"), 0o644), IsNil)
		c.Assert(os.Symlink("foo", filepath.Join(dir, "bar")), IsNil)
	},
	result: map[string]string{
		"/foo": "file 0644 3a6eb079",
	},
}, {
	summary: "Remove a hard link",
	options: fsutil.RemoveOptions{
		Path: "hardlink1",
	},
	hackopt: func(c *C, dir string, opts *fsutil.RemoveOptions) {
		c.Assert(os.WriteFile(filepath.Join(dir, "file"), []byte("data"), 0o644), IsNil)
		c.Assert(os.Link(filepath.Join(dir, "file"), filepath.Join(dir, "hardlink1")), IsNil)
		c.Assert(os.Link(filepath.Join(dir, "file"), filepath.Join(dir, "hardlink2")), IsNil)
	},
	result: map[string]string{
		"/file":      "file 0644 3a6eb079 <1>",
		"/hardlink2": "file 0644 3a6eb079 <1>",
	},
}, {
	summary: "Cannot remove file outside of Root",
	options: fsutil.RemoveOptions{
		Root: "/root",
		Path: "../file",
	},
	error: `cannot handle path /file outside of root /root/`,
}}

func (s *S) TestRemove(c *C) {
	oldUmask := syscall.Umask(0)
	defer func() {
		syscall.Umask(oldUmask)
	}()

	for _, test := range removeTests {
		c.Logf("Test: %s", test.summary)
		if test.result == nil {
			// Empty map for no files left.
			test.result = make(map[string]string)
		}
		c.Logf("Options: %v", test.options)
		dir := c.MkDir()
		options := test.options
		if options.Root == "" {
			options.Root = dir
		}
		if test.hackopt != nil {
			test.hackopt(c, dir, &options)
		}
		err := fsutil.Remove(&options)

		if test.error != "" {
			c.Assert(err, ErrorMatches, test.error)
			continue
		}

		c.Assert(err, IsNil)
		c.Assert(testutil.TreeDump(dir), DeepEquals, test.result)
	}
}

func (s *S) TestRemoveEmptyRoot(c *C) {
	options := &fsutil.RemoveOptions{Root: ""}
	err := fsutil.Remove(options)
	c.Assert(err, ErrorMatches, "internal error: RemoveOptions.Root is unset")
}

func (s *S) TestRemoveEmptyPath(c *C) {
	options := &fsutil.RemoveOptions{Root: "foo/"}
	err := fsutil.Remove(options)
	c.Assert(err, ErrorMatches, "internal error: RemoveOptions.Path is unset")
}
