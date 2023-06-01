package fsutil_test

import (
	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/fsutil"
)

var cleanAndDirTestCases = []struct {
	inputPath   string
	resultClean string
	resultDir   string
}{
	{"/a/b/c", "/a/b/c", "/a/b/"},
	{"/a/b/c/", "/a/b/c/", "/a/b/"},
	{"/a/b/c//", "/a/b/c/", "/a/b/"},
	{"/a/b//c", "/a/b/c", "/a/b/"},
	{"/a/b/c/.", "/a/b/c/", "/a/b/"},
	{"/a/b/c/.///.", "/a/b/c/", "/a/b/"},
	{"/a/b/./c/", "/a/b/c/", "/a/b/"},
	{"/a/b/.///./c", "/a/b/c", "/a/b/"},
	{"/a/b/c/..", "/a/b/", "/a/"},
	{"/a/b/c/..///./", "/a/b/", "/a/"},
	{"/a/b/c/../.", "/a/b/", "/a/"},
	{"/a/b/../c/", "/a/c/", "/a/"},
	{"/a/b/..///./c", "/a/c", "/a/"},
	{"a/b/./c", "a/b/c", "a/b/"},
	{"./a/b/./c", "a/b/c", "a/b/"},
	{"/", "/", "/"},
	{"///", "/", "/"},
	{"///.///", "/", "/"},
	{"/././.", "/", "/"},
	{".", "./", "./"},
	{".///", "./", "./"},
	{"..", "../", "./"},
	{"..///.", "../", "./"},
	{"../../..", "../../../", "../../"},
	{"..///.///../..", "../../../", "../../"},
	{"", "./", "./"},
}

func (s *S) TestSlashedPathClean(c *C) {
	for _, t := range cleanAndDirTestCases {
		c.Logf("%s => %s", t.inputPath, t.resultClean)
		c.Assert(fsutil.SlashedPathClean(t.inputPath), Equals, t.resultClean)
	}
}

func (s *S) TestSlashedPathDir(c *C) {
	for _, t := range cleanAndDirTestCases {
		c.Logf("%s => %s", t.inputPath, t.resultDir)
		c.Assert(fsutil.SlashedPathDir(t.inputPath), Equals, t.resultDir)
	}
}
