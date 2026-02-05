package fsutil_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/testutil"
)

type moveTest struct {
	summary string
	options fsutil.MoveOptions
	hackopt func(c *C, dir string, opts *fsutil.MoveOptions)
	result  map[string]string
	error   string
}

var moveTests = []moveTest{{
	summary: "Move a file and create its parent directory",
	options: fsutil.MoveOptions{
		Path:        "bar",
		Mode:        0o644,
		MakeParents: true,
	},
	hackopt: func(c *C, dir string, opts *fsutil.MoveOptions) {
		c.Assert(os.WriteFile(filepath.Join(dir, "src/bar"), []byte("data"), 0o644), IsNil)
	},
	result: map[string]string{
		"/src/":    "dir 0755",
		"/dst/":    "dir 0755",
		"/dst/bar": "file 0644 3a6eb079",
	},
}, {
	summary: "Move a file when parent directory exists",
	options: fsutil.MoveOptions{
		Path: "bar",
		Mode: 0o644,
	},
	hackopt: func(c *C, dir string, opts *fsutil.MoveOptions) {
		c.Assert(os.WriteFile(filepath.Join(dir, "src/bar"), []byte("data"), 0o644), IsNil)
	},
	result: map[string]string{
		"/src/":    "dir 0755",
		"/dst/":    "dir 0755",
		"/dst/bar": "file 0644 3a6eb079",
	},
}, {
	summary: "Move an empty file",
	options: fsutil.MoveOptions{
		Path:        "empty",
		Mode:        0o644,
		MakeParents: true,
	},
	hackopt: func(c *C, dir string, opts *fsutil.MoveOptions) {
		c.Assert(os.WriteFile(filepath.Join(dir, "src/empty"), []byte(""), 0o644), IsNil)
	},
	result: map[string]string{
		"/src/":      "dir 0755",
		"/dst/":      "dir 0755",
		"/dst/empty": "file 0644 empty",
	},
}, {
	summary: "Move a symlink",
	options: fsutil.MoveOptions{
		Path:        "foo",
		Mode:        fs.ModeSymlink,
		MakeParents: true,
	},
	hackopt: func(c *C, dir string, opts *fsutil.MoveOptions) {
		c.Assert(os.WriteFile(filepath.Join(dir, "src/baz"), []byte("data"), 0o644), IsNil)
		c.Assert(os.Symlink("baz", filepath.Join(dir, "src/foo")), IsNil)
	},
	result: map[string]string{
		"/src/":    "dir 0755",
		"/src/baz": "file 0644 3a6eb079",
		"/dst/":    "dir 0755",
		"/dst/foo": "symlink baz",
	},
}, {
	summary: "Move (create) a directory",
	options: fsutil.MoveOptions{
		Path: "foo/",
		Mode: fs.ModeDir | 0o765,
	},
	result: map[string]string{
		"/src/":     "dir 0755",
		"/dst/":     "dir 0755",
		"/dst/foo/": "dir 0765",
	},
}, {
	summary: "Move (create) a directory with sticky bit",
	options: fsutil.MoveOptions{
		Path: "foo",
		Mode: fs.ModeDir | fs.ModeSticky | 0o775,
	},
	result: map[string]string{
		"/src/":     "dir 0755",
		"/dst/":     "dir 0755",
		"/dst/foo/": "dir 01775",
	},
}, {
	summary: "Cannot move (create) a parent directory without MakeParents set",
	options: fsutil.MoveOptions{
		Path: "foo/bar",
		Mode: fs.ModeDir | 0o775,
	},
	error: `mkdir /[^ ]*/foo/bar: no such file or directory`,
}, {
	summary: "Do not override mode of existing",
	options: fsutil.MoveOptions{
		Path:         "foo",
		Mode:         fs.ModeDir | 0o775,
	},
	hackopt: func(c *C, dir string, opts *fsutil.MoveOptions) {
		c.Assert(os.Mkdir(filepath.Join(dir, "dst/foo/"), fs.ModeDir|0o765), IsNil)
	},
	result: map[string]string{
		"/src/": "dir 0755",
		"/dst/": "dir 0755",
		// mode is not updated.
		"/dst/foo/": "dir 0765",
	},
}, {
	summary: "Moving to an existing file overrides the original mode",
	options: fsutil.MoveOptions{
		Path: "foo",
	},
	hackopt: func(c *C, dir string, opts *fsutil.MoveOptions) {
		c.Assert(os.WriteFile(filepath.Join(dir, "src/foo"), []byte("data"), 0o644), IsNil)
		c.Assert(os.WriteFile(filepath.Join(dir, "dst/foo"), []byte("data"), 0o666), IsNil)
	},
	result: map[string]string{
		"/src/":    "dir 0755",
		"/dst/":    "dir 0755",
		"/dst/foo": "file 0644 3a6eb079",
	},
}, {
	summary: "Move a hard link",
	options: fsutil.MoveOptions{
		Path:        "hardlink",
		Mode:        0o644,
		MakeParents: true,
	},
	hackopt: func(c *C, dir string, opts *fsutil.MoveOptions) {
		c.Assert(os.WriteFile(filepath.Join(dir, "src/file"), []byte("data"), 0o644), IsNil)
		c.Assert(os.Link(filepath.Join(dir, "src/file"), filepath.Join(dir, "src/hardlink")), IsNil)
	},
	result: map[string]string{
		"/src/":         "dir 0755",
		"/dst/":         "dir 0755",
		"/src/file":     "file 0644 3a6eb079 <1>",
		"/dst/hardlink": "file 0644 3a6eb079 <1>",
	},
}, {
	summary: "No error if hard link already exists",
	options: fsutil.MoveOptions{
		Path: "hardlink",
		Mode: 0o644,
	},
	hackopt: func(c *C, dir string, opts *fsutil.MoveOptions) {
		c.Assert(os.WriteFile(filepath.Join(dir, "src/file"), []byte("data"), 0o644), IsNil)
		c.Assert(os.WriteFile(filepath.Join(dir, "dst/foo"), []byte("data"), 0o644), IsNil)
		c.Assert(os.Link(filepath.Join(dir, "src/file"), filepath.Join(dir, "src/hardlink")), IsNil)
		c.Assert(os.Link(filepath.Join(dir, "dst/foo"), filepath.Join(dir, "dst/hardlink")), IsNil)
	},
	result: map[string]string{
		"/src/":         "dir 0755",
		"/dst/":         "dir 0755",
		"/src/file":     "file 0644 3a6eb079 <1>",
		"/dst/foo":      "file 0644 3a6eb079",
		"/dst/hardlink": "file 0644 3a6eb079 <1>",
	},
}, {
	summary: "Override a symlink",
	options: fsutil.MoveOptions{
		Path: "foo",
		Mode: 0o666 | fs.ModeSymlink,
	},
	hackopt: func(c *C, dir string, opts *fsutil.MoveOptions) {
		c.Assert(os.WriteFile(filepath.Join(dir, "src/baz"), []byte("data"), 0o666), IsNil)
		c.Assert(os.Symlink("baz", filepath.Join(dir, "src/foo")), IsNil)
		c.Assert(os.WriteFile(filepath.Join(dir, "dst/bar"), []byte("data"), 0o644), IsNil)
		c.Assert(os.Symlink("bar", filepath.Join(dir, "dst/foo")), IsNil)
	},
	result: map[string]string{
		"/src/":    "dir 0755",
		"/src/baz": "file 0666 3a6eb079",
		"/dst/":    "dir 0755",
		"/dst/bar": "file 0644 3a6eb079",
		"/dst/foo": "symlink baz",
	},
}, {
	summary: "Cannot move file from outside of Root",
	options: fsutil.MoveOptions{
		SrcRoot: "/rootsrc",
		DstRoot: "/rootdst",
		Path:    "../file",
		Mode:    0o666,
	},
	error: `cannot handle path /file outside of root /rootsrc`,
}, {
	summary: "Path with ./ component",
	options: fsutil.MoveOptions{
		Path:        "./file",
		Mode:        0o644,
		MakeParents: true,
	},
	hackopt: func(c *C, dir string, opts *fsutil.MoveOptions) {
		c.Assert(os.WriteFile(filepath.Join(dir, "src/file"), []byte("data"), 0o644), IsNil)
	},
	result: map[string]string{
		"/src/":     "dir 0755",
		"/dst/":     "dir 0755",
		"/dst/file": "file 0644 3a6eb079",
	},
}, {
	summary: "Path with ../ component normalizes correctly",
	options: fsutil.MoveOptions{
		Path:        "foo/../bar",
		Mode:        0o644,
		MakeParents: true,
	},
	hackopt: func(c *C, dir string, opts *fsutil.MoveOptions) {
		c.Assert(os.WriteFile(filepath.Join(dir, "src/bar"), []byte("data"), 0o644), IsNil)
	},
	result: map[string]string{
		"/src/":    "dir 0755",
		"/dst/":    "dir 0755",
		"/dst/bar": "file 0644 3a6eb079",
	},
}, {
	summary: "Cannot move to a path where parent is a file",
	options: fsutil.MoveOptions{
		Path:        "file/subpath",
		Mode:        0o644,
		MakeParents: true,
	},
	hackopt: func(c *C, dir string, opts *fsutil.MoveOptions) {
		c.Assert(os.WriteFile(filepath.Join(dir, "src/file"), []byte("data"), 0o644), IsNil)
		c.Assert(os.WriteFile(filepath.Join(dir, "dst/file"), []byte("data"), 0o644), IsNil)
	},
	error: `mkdir .*/dst/file: not a directory`,
}, {
	summary: "Cannot move a file to overwrite a directory",
	options: fsutil.MoveOptions{
		Path: "target",
		Mode: 0o644,
	},
	hackopt: func(c *C, dir string, opts *fsutil.MoveOptions) {
		c.Assert(os.WriteFile(filepath.Join(dir, "src/target"), []byte("data"), 0o644), IsNil)
		c.Assert(os.Mkdir(filepath.Join(dir, "dst/target"), 0o755), IsNil)
	},
	error: `rename .*/src/target .*/dst/target: file exists`,
}, {
	summary: "Cannot move inexistent file",
	options: fsutil.MoveOptions{
		Path: "file",
		Mode: 0o666,
	},
	error: `rename .*/src/file .*/dst/file: no such file or directory`,
}, {
	summary: "Cannot move file to outside of Root",
	options: fsutil.MoveOptions{
		SrcRoot: "/rootsrc",
		DstRoot: "/rootdst",
		Path:    "../rootsrc/file",
		Mode:    0o666,
	},
	error: `cannot handle path /rootsrc/file outside of root /rootdst`,
}}

func (s *S) TestMove(c *C) {
	oldUmask := syscall.Umask(0)
	defer func() {
		syscall.Umask(oldUmask)
	}()

	for _, test := range moveTests {
		c.Logf("Test: %s", test.summary)
		if test.result == nil {
			// Empty map for no files moved.
			test.result = make(map[string]string)
		}
		c.Logf("Options: %v", test.options)
		dir := c.MkDir()
		options := test.options
		if options.SrcRoot == "" {
			srcRoot := filepath.Join(dir, "src")
			c.Assert(os.Mkdir(srcRoot, 0o755), IsNil)
			options.SrcRoot = srcRoot
		}
		if options.DstRoot == "" {
			dstRoot := filepath.Join(dir, "dst")
			c.Assert(os.Mkdir(dstRoot, 0o755), IsNil)
			options.DstRoot = dstRoot
		}
		if test.hackopt != nil {
			test.hackopt(c, dir, &options)
		}
		err := fsutil.Move(&options)

		if test.error != "" {
			c.Assert(err, ErrorMatches, test.error)
			continue
		}

		c.Assert(err, IsNil)
		c.Assert(testutil.TreeDump(dir), DeepEquals, test.result)
	}
}

func (s *S) TestMoveEmptyRoot(c *C) {
	options := &fsutil.MoveOptions{SrcRoot: "", DstRoot: "foo/"}
	err := fsutil.Move(options)
	c.Assert(err, ErrorMatches, "internal error: MoveOptions.SrcRoot is unset")
	options = &fsutil.MoveOptions{SrcRoot: "foo/", DstRoot: ""}
	err = fsutil.Move(options)
	c.Assert(err, ErrorMatches, "internal error: MoveOptions.DstRoot is unset")
}
