package control_test

import (
	"github.com/canonical/chisel/internal/control"

	. "gopkg.in/check.v1"
)

type parsePathInfoTest struct {
	table  string
	path   string
	size   int
	digest string
}

var parsePathInfoTests = []parsePathInfoTest{{
	table: `
		before 1 /one/path
		0123456789abcdef0123456789abcdef 2 /the/path
		after 2 /two/path
	`,
	path:   "/the/path",
	size:   2,
	digest: "0123456789abcdef0123456789abcdef",
}, {
	table: `
		0123456789abcdef0123456789abcdef 1 /the/path
		after 2 /two/path
	`,
	path:   "/the/path",
	size:   1,
	digest: "0123456789abcdef0123456789abcdef",
}, {
	table: `
		before 1 /two/path
		0123456789abcdef0123456789abcdef 2 /the/path
	`,
	path:   "/the/path",
	size:   2,
	digest: "0123456789abcdef0123456789abcdef",
}, {
	table: `0123456789abcdef0123456789abcdef 0 /the/path`,
	path:   "/the/path",
	size:   0,
	digest: "0123456789abcdef0123456789abcdef",
}, {
	table: `0123456789abcdef0123456789abcdef    555    /the/path`,
	path:   "/the/path",
	size:   555,
	digest: "0123456789abcdef0123456789abcdef",
}, {
	table: `deadbeef 0 /the/path`,
	path:   "/the/path",
	digest: "",
}, {
	table: `bad-data 0 /the/path`,
	path:   "/the/path",
	digest: "",
}}

func (s *S) TestParsePathInfo(c *C) {
	for _, test := range parsePathInfoTests {
		c.Logf("Path is %q, expecting digest %q and size %d.", test.path, test.digest, test.size)
		digest, size, ok := control.ParsePathInfo(test.table, test.path)
		c.Logf("Got digest %q, size %d, ok %v.", digest, size, ok)
		if test.digest == "" {
			c.Assert(digest, Equals, "")
			c.Assert(size, Equals, -1)
			c.Assert(ok, Equals, false)
		} else {
			c.Assert(digest, Equals, test.digest)
			c.Assert(size, Equals, test.size)
			c.Assert(ok, Equals, true)
		}
	}
}
