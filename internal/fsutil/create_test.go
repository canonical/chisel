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
}, {
	options: fsutil.CreateOptions{
		Path:         "foo",
		Mode:         fs.ModeDir | 0775,
		OverrideMode: true,
	},
	hackdir: func(c *C, dir string) {
		c.Assert(os.Mkdir(filepath.Join(dir, "foo/"), fs.ModeDir|0765), IsNil)
	},
	result: map[string]string{
		// mode is updated.
		"/foo/": "dir 0775",
	},
}, {
	options: fsutil.CreateOptions{
		Path:         "foo",
		Mode:         0775,
		Data:         bytes.NewBufferString("whatever"),
		OverrideMode: true,
	},
	hackdir: func(c *C, dir string) {
		err := os.WriteFile(filepath.Join(dir, "foo"), []byte("data"), 0666)
		c.Assert(err, IsNil)
	},
	result: map[string]string{
		// mode is updated.
		"/foo": "file 0775 85738f8f",
	},
}, {
	options: fsutil.CreateOptions{
		Path: "foo",
		Link: "./bar",
		Mode: 0666 | fs.ModeSymlink,
	},
	hackdir: func(c *C, dir string) {
		err := os.WriteFile(filepath.Join(dir, "foo"), []byte("data"), 0666)
		c.Assert(err, IsNil)
	},
	result: map[string]string{
		"/foo": "symlink ./bar",
	},
}, {
	options: fsutil.CreateOptions{
		Path:         "foo",
		Link:         "./bar",
		Mode:         0776 | fs.ModeSymlink,
		OverrideMode: true,
	},
	hackdir: func(c *C, dir string) {
		err := os.WriteFile(filepath.Join(dir, "bar"), []byte("data"), 0666)
		c.Assert(err, IsNil)
		err = os.WriteFile(filepath.Join(dir, "foo"), []byte("data"), 0666)
		c.Assert(err, IsNil)
	},
	result: map[string]string{
		"/foo": "symlink ./bar",
		// mode is not updated.
		"/bar": "file 0666 3a6eb079",
	},
}, {
	options: fsutil.CreateOptions{
		Path: "bar",
		// Existing link with different target.
		Link: "other",
		Mode: 0666 | fs.ModeSymlink,
	},
	hackdir: func(c *C, dir string) {
		err := os.Symlink("foo", filepath.Join(dir, "bar"))
		c.Assert(err, IsNil)
	},
	result: map[string]string{
		"/bar": "symlink other",
	},
}, {
	options: fsutil.CreateOptions{
		Path: "bar",
		// Existing link with same target.
		Link: "foo",
		Mode: 0666 | fs.ModeSymlink,
	},
	hackdir: func(c *C, dir string) {
		err := os.Symlink("foo", filepath.Join(dir, "bar"))
		c.Assert(err, IsNil)
	},
	result: map[string]string{
		"/bar": "symlink foo",
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

type createWriterTest struct {
	options fsutil.CreateOptions
	data    []byte
	hackdir func(c *C, dir string)
	result  map[string]string
	error   string
}

var createWriterTests = []createWriterTest{{
	options: fsutil.CreateOptions{
		Path: "foo",
		Mode: 0644,
	},
	data: []byte("foo"),
	result: map[string]string{
		"/foo": "file 0644 2c26b46b",
	},
}, {
	options: fsutil.CreateOptions{
		Path: "foo",
		Mode: 0644 | fs.ModeDir,
	},
	error: `unsupported file type: \/[a-z0-9\-\/]*foo`,
}, {
	options: fsutil.CreateOptions{
		Path: "foo",
		Mode: 0644 | fs.ModeSymlink,
	},
	error: `unsupported file type: /[a-z0-9\-\/]*/foo`,
}, {
	options: fsutil.CreateOptions{
		Path:        "foo/bar",
		Mode:        0644,
		MakeParents: true,
	},
	data: []byte("foo"),
	result: map[string]string{
		"/foo/":    "dir 0755",
		"/foo/bar": "file 0644 2c26b46b",
	},
}, {
	options: fsutil.CreateOptions{
		Path:        "foo/bar",
		Mode:        0644,
		MakeParents: false,
	},
	error: `open /[a-z0-9\-\/]*/foo/bar: no such file or directory`,
}}

func (s *S) TestCreateWriter(c *C) {
	oldUmask := syscall.Umask(0)
	defer func() {
		syscall.Umask(oldUmask)
	}()

	for _, test := range createWriterTests {
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
		writer, entry, err := fsutil.CreateWriter(&options)
		if test.error != "" {
			c.Assert(err, ErrorMatches, test.error)
			continue
		}
		c.Assert(err, IsNil)

		// Hash and Size are only set when the writer is closed.
		_, err = writer.Write(test.data)
		c.Assert(err, IsNil)
		c.Assert(entry.Path, Equals, options.Path)
		c.Assert(entry.Mode, Equals, options.Mode)
		c.Assert(entry.SHA256, Equals, "")
		c.Assert(entry.Size, Equals, 0)
		err = writer.Close()
		c.Assert(err, IsNil)

		c.Assert(testutil.TreeDump(dir), DeepEquals, test.result)
		// [fsutil.CreateWriter] does not return information about parent
		// directories created implicitly. We only check for the requested path.
		entry.Path = strings.TrimPrefix(entry.Path, dir)
		// Add the slashes that TreeDump adds to the path.
		slashPath := "/" + test.options.Path
		if test.options.Mode.IsDir() {
			slashPath = slashPath + "/"
		}
		c.Assert(testutil.TreeDumpEntry(entry), DeepEquals, test.result[slashPath])
	}
}
