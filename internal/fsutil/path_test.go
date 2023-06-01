package fsutil_test

import (
	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/fsutil"
)

var dirTests = []struct {
	input  string
	output string
}{
	{"/a/b/c", "/a/b/"},
	{"/a/b/c/", "/a/b/"},
	{"/a/b/c//", "/a/b/"},
	{"/a/b/c/././", "/a/b/"},
	{"/a/b/c/.././", "/a/"},
	{"/a/b//c", "/a/b/"},
	{"/a/b/./c", "/a/b/"},
	{"a/b/./c", "a/b/"},
	{"./a/b/./c", "a/b/"},
	{"/", "/"},
	{"///.///", "/"},
	{".", "./"},
	{"", "./"},
}

var cleanTests = []struct {
	input  string
	output string
}{
	{"/a/b/c", "/a/b/c"},
	{"/a/b/c/", "/a/b/c/"},
	{"/a/b/c/.//", "/a/b/c/"},
	{"/a/b//./c", "/a/b/c"},
	{"/a/b//./c/", "/a/b/c/"},
	{"/a/b/.././c/", "/a/c/"},
}

func (s *S) TestDir(c *C) {
	for _, t := range dirTests {
		c.Logf("%s => %s", t.input, t.output)
		c.Assert(fsutil.Dir(t.input), Equals, t.output)
	}
}

func (s *S) TestClean(c *C) {
	for _, t := range cleanTests {
		c.Logf("%s => %s", t.input, t.output)
		c.Assert(fsutil.Clean(t.input), Equals, t.output)
	}
}
