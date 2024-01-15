package fsutil_test

import (
	"path/filepath"
	"syscall"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/testutil"
)

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
