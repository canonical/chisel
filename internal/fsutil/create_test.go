package fsutil_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/testutil"
)

type createTest struct {
	options fsutil.CreateOptions
	hackdir func(c *C, dir string)
	result  map[string]string
	error   string
}

var createTests = []createTest{{
	options: fsutil.CreateOptions{
		Path:        "foo/bar",
		Data:        bytes.NewBufferString("data1"),
		Mode:        0444,
		MakeParents: true,
	},
	result: map[string]string{
		"/foo/":    "dir 0755",
		"/foo/bar": "file 0444 5b41362b",
	},
}, {
	options: fsutil.CreateOptions{
		Path:        "foo/bar",
		Link:        "../baz",
		Mode:        fs.ModeSymlink,
		MakeParents: true,
	},
	result: map[string]string{
		"/foo/":    "dir 0755",
		"/foo/bar": "symlink ../baz",
	},
}, {
	options: fsutil.CreateOptions{
		Path:        "foo/bar",
		Mode:        fs.ModeDir | 0444,
		MakeParents: true,
	},
	result: map[string]string{
		"/foo/":     "dir 0755",
		"/foo/bar/": "dir 0444",
	},
}, {
	options: fsutil.CreateOptions{
		Path: "tmp",
		Mode: fs.ModeDir | fs.ModeSticky | 0775,
	},
	result: map[string]string{
		"/tmp/": "dir 01775",
	},
}, {
	options: fsutil.CreateOptions{
		Path: "foo/bar",
		Mode: fs.ModeDir | 0775,
	},
	error: `.*: no such file or directory`,
}, {
	options: fsutil.CreateOptions{
		Path: "foo",
		Mode: fs.ModeDir | 0775,
	},
	hackdir: func(c *C, dir string) {
		c.Assert(os.Mkdir(filepath.Join(dir, "foo/"), fs.ModeDir|0765), IsNil)
	},
	result: map[string]string{
		// mode is not updated.
		"/foo/": "dir 0765",
	},
}, {
	options: fsutil.CreateOptions{
		Path: "foo",
		// Mode should be ignored for existing entry.
		Mode: 0644,
		Data: bytes.NewBufferString("changed"),
	},
	hackdir: func(c *C, dir string) {
		c.Assert(os.WriteFile(filepath.Join(dir, "foo"), []byte("data"), 0666), IsNil)
	},
	result: map[string]string{
		// mode is not updated.
		"/foo": "file 0666 d67e2e94",
	},
}}

func (s *S) TestCreate(c *C) {
	oldUmask := syscall.Umask(0)
	defer func() {
		syscall.Umask(oldUmask)
	}()

	for _, test := range createTests {
		if test.result == nil {
			// Empty map for no files created.
			test.result = make(map[string]string)
		}
		c.Logf("Options: %v", test.options)
		dir := c.MkDir()
		if test.hackdir != nil {
			test.hackdir(c, dir)
		}
		options := test.options
		options.Path = filepath.Join(dir, options.Path)
		entry, err := fsutil.Create(&options)

		if test.error != "" {
			c.Assert(err, ErrorMatches, test.error)
			continue
		}

		c.Assert(err, IsNil)
		c.Assert(testutil.TreeDump(dir), DeepEquals, test.result)
		// [fsutil.Create] does not return information about parent directories
		// created implicitly. We only check for the requested path.
		entry.Path = strings.TrimPrefix(entry.Path, dir)
		// Add the slashes that TreeDump adds to the path.
		slashPath := "/" + test.options.Path
		if test.options.Mode.IsDir() {
			slashPath = slashPath + "/"
		}
		c.Assert(testutil.TreeDumpEntry(entry), DeepEquals, test.result[slashPath])
	}
}
