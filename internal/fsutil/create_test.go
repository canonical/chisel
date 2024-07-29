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
	summary string
	options fsutil.CreateOptions
	hackopt func(c *C, targetDir string, options *fsutil.CreateOptions)
	result  map[string]string
	error   string
}

var createTests = []createTest{{
	summary: "Create a file and its parent directory",
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
	summary: "Create a symlink",
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
	summary: "Create a directory",
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
	summary: "Create a directory with sticky bit",
	options: fsutil.CreateOptions{
		Path: "tmp",
		Mode: fs.ModeDir | fs.ModeSticky | 0775,
	},
	result: map[string]string{
		"/tmp/": "dir 01775",
	},
}, {
	summary: "Cannot create a parent directory without MakeParents set",
	options: fsutil.CreateOptions{
		Path: "foo/bar",
		Mode: fs.ModeDir | 0775,
	},
	error: `mkdir \/[^ ]*\/foo/bar: no such file or directory`,
}, {
	summary: "Re-creating an existing directory keeps the original mode",
	options: fsutil.CreateOptions{
		Path: "foo",
		Mode: fs.ModeDir | 0775,
	},
	hackopt: func(c *C, targetDir string, options *fsutil.CreateOptions) {
		c.Assert(os.Mkdir(filepath.Join(targetDir, "foo/"), fs.ModeDir|0765), IsNil)
	},
	result: map[string]string{
		// mode is not updated.
		"/foo/": "dir 0765",
	},
}, {
	summary: "Re-creating an existing file keeps the original mode",
	options: fsutil.CreateOptions{
		Path: "foo",
		// Mode should be ignored for existing entry.
		Mode: 0644,
		Data: bytes.NewBufferString("changed"),
	},
	hackopt: func(c *C, targetDir string, options *fsutil.CreateOptions) {
		c.Assert(os.WriteFile(filepath.Join(targetDir, "foo"), []byte("data"), 0666), IsNil)
	},
	result: map[string]string{
		// mode is not updated.
		"/foo": "file 0666 d67e2e94",
	},
}, {
	summary: "Create a hard link",
	options: fsutil.CreateOptions{
		Path:        "dir/hardlink",
		Link:        "file",
		Mode:        0644,
		MakeParents: true,
	},
	hackopt: func(c *C, targetDir string, options *fsutil.CreateOptions) {
		c.Assert(os.WriteFile(filepath.Join(targetDir, "file"), []byte("data"), 0644), IsNil)
		// An absolute path is required to create a hard link.
		options.Link = filepath.Join(targetDir, options.Link)
	},
	result: map[string]string{
		"/file":         "file 0644 3a6eb079",
		"/dir/":         "dir 0755",
		"/dir/hardlink": "file 0644 3a6eb079",
	},
}, {
	summary: "Cannot create a hard link if the link target does not exist",
	options: fsutil.CreateOptions{
		Path:        "dir/hardlink",
		Link:        "missing-file",
		Mode:        0644,
		MakeParents: true,
	},
	hackopt: func(c *C, targetDir string, options *fsutil.CreateOptions) {
		options.Link = filepath.Join(targetDir, options.Link)
	},
	error: `link target does not exist: \/[^ ]*\/missing-file`,
}, {
	summary: "Re-creating a duplicated hard link keeps the original link",
	options: fsutil.CreateOptions{
		Path:        "hardlink",
		Link:        "file",
		Mode:        0644,
		MakeParents: true,
	},
	hackopt: func(c *C, targetDir string, options *fsutil.CreateOptions) {
		c.Assert(os.WriteFile(filepath.Join(targetDir, "file"), []byte("data"), 0644), IsNil)
		c.Assert(os.Link(filepath.Join(targetDir, "file"), filepath.Join(targetDir, "hardlink")), IsNil)
		options.Link = filepath.Join(targetDir, options.Link)
	},
	result: map[string]string{
		"/file":     "file 0644 3a6eb079",
		"/hardlink": "file 0644 3a6eb079",
	},
}, {
	summary: "Cannot create a hard link if the link path exists and it is not a hard link to the target",
	options: fsutil.CreateOptions{
		Path:        "hardlink",
		Link:        "file",
		Mode:        0644,
		MakeParents: true,
	},
	hackopt: func(c *C, targetDir string, options *fsutil.CreateOptions) {
		c.Assert(os.WriteFile(filepath.Join(targetDir, "file"), []byte("data"), 0644), IsNil)
		c.Assert(os.WriteFile(filepath.Join(targetDir, "hardlink"), []byte("data"), 0644), IsNil)
		options.Link = filepath.Join(targetDir, options.Link)
	},
	error: `path \/[^ ]*\/hardlink already exists`,
}}

func (s *S) TestCreate(c *C) {
	oldUmask := syscall.Umask(0)
	defer func() {
		syscall.Umask(oldUmask)
	}()

	for _, test := range createTests {
		c.Logf("Test: %s", test.summary)
		if test.result == nil {
			// Empty map for no files created.
			test.result = make(map[string]string)
		}
		c.Logf("Options: %v", test.options)
		dir := c.MkDir()
		options := test.options
		options.Path = filepath.Join(dir, options.Path)
		if test.hackopt != nil {
			test.hackopt(c, dir, &options)
		}
		entry, err := fsutil.Create(&options)

		if test.error != "" {
			c.Assert(err, ErrorMatches, test.error)
			continue
		}

		c.Assert(err, IsNil)
		c.Assert(testutil.TreeDump(dir), DeepEquals, test.result)

		// [fsutil.Create] does not return information about parent directories
		// created implicitly. We only check for the requested path.
		if entry.Link != "" && entry.Mode&fs.ModeSymlink == 0 {
			// Entry is a hard link.
			pathInfo, err := os.Lstat(entry.Path)
			c.Assert(err, IsNil)
			linkInfo, err := os.Lstat(entry.Link)
			c.Assert(err, IsNil)
			os.SameFile(pathInfo, linkInfo)
		} else {
			entry.Path = strings.TrimPrefix(entry.Path, dir)
			// Add the slashes that TreeDump adds to the path.
			slashPath := "/" + test.options.Path
			if test.options.Mode.IsDir() {
				slashPath = slashPath + "/"
			}
			c.Assert(testutil.TreeDumpEntry(entry), DeepEquals, test.result[slashPath])
		}
	}
}
