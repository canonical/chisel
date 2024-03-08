package fsutil_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"syscall"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/testutil"
)

type createTest struct {
	options fsutil.CreateOptions
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
		c.Assert(dumpFSEntry(entry, dir)[test.options.Path], DeepEquals, test.result[test.options.Path])
	}
}

// dumpFSEntry returns the file entry in the same format as [testutil.TreeDump].
func dumpFSEntry(fsEntry *fsutil.Entry, root string) map[string]string {
	result := make(map[string]string)
	path := strings.TrimPrefix(fsEntry.Path, root)
	fperm := fsEntry.Mode.Perm()
	if fsEntry.Mode&fs.ModeSticky != 0 {
		fperm |= 01000
	}
	switch fsEntry.Mode.Type() {
	case fs.ModeDir:
		result[path+"/"] = fmt.Sprintf("dir %#o", fperm)
	case fs.ModeSymlink:
		result[path] = fmt.Sprintf("symlink %s", fsEntry.Link)
	case 0: // Regular
		var entry string
		if fsEntry.Size == 0 {
			entry = fmt.Sprintf("file %#o empty", fsEntry.Mode.Perm())
		} else {
			entry = fmt.Sprintf("file %#o %s", fperm, fsEntry.Hash[:8])
		}
		result[path] = entry
	default:
		panic(fmt.Errorf("unknown file type %d: %s", fsEntry.Mode.Type(), path))
	}
	return result
}
