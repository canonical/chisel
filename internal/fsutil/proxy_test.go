package fsutil_test

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"syscall"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/testutil"
)

func treeDumpFileCreator(fc *fsutil.FileCreator, root string) map[string]string {
	result := make(map[string]string)
	for _, file := range fc.Files {
		path := strings.TrimPrefix(file.Path, root)
		fperm := file.Mode.Perm()
		if file.Mode&fs.ModeSticky != 0 {
			fperm |= 01000
		}
		switch file.Mode.Type() {
		case fs.ModeDir:
			result[path+"/"] = fmt.Sprintf("dir %#o", fperm)
		case fs.ModeSymlink:
			result[path] = fmt.Sprintf("symlink %s", file.Link)
		case 0: // Regular
			var entry string
			if file.Size == 0 {
				entry = fmt.Sprintf("file %#o empty", file.Mode.Perm())
			} else {
				entry = fmt.Sprintf("file %#o %s", fperm, file.Hash[:8])
			}
			result[path] = entry
		default:
			panic(fmt.Errorf("unknown file type %d: %s", file.Mode.Type(), path))
		}
	}
	return result
}

func (s *S) TestProxy(c *C) {
	oldUmask := syscall.Umask(0)
	defer func() {
		syscall.Umask(oldUmask)
	}()

	for _, test := range createTests() {
		c.Logf("Options: %v", test.options)
		dir := c.MkDir()
		options := test.options
		options.Path = filepath.Join(dir, options.Path)
		fileCreator := fsutil.NewFileCreator()
		err := fileCreator.Create(&options)
		if test.error != "" {
			c.Assert(err, ErrorMatches, test.error)
			continue
		} else {
			c.Assert(err, IsNil)
		}
		c.Assert(testutil.TreeDump(dir), DeepEquals, test.result)
		if test.options.MakeParents {
			// The fileCreator does not record the parent directories created
			// implicitly.
			for path, info := range treeDumpFileCreator(fileCreator, dir) {
				c.Assert(info, Equals, test.result[path])
			}
		} else {
			c.Assert(treeDumpFileCreator(fileCreator, dir), DeepEquals, test.result)
		}
	}
}
