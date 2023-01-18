package testutil_test

import (
	"strings"

	"github.com/canonical/chisel/internal/testutil"
	. "gopkg.in/check.v1"
)

type reindentTest struct {
	src, expect, err string
}

var reindentTests = []reindentTest{{
	src:    "a\nb",
	expect: "a\nb",
}, {
	src:    "\ta\n\tb",
	expect: "a\nb",
}, {
	src:    "    a\n    b",
	expect: "    a\n    b",
}, {
	src:    "a\n\tb\nc",
	expect: "a\n    b\nc",
}, {
	src:    "a\n  b\nc",
	expect: "a\n  b\nc",
}, {
	src:    "\ta\n\t\tb\n\tc",
	expect: "a\n    b\nc",
}, {
	src:    "  a\n    b\n  c",
	expect: "  a\n    b\n  c",
}, {
	src:    "    a\n    \tb\n    c",
	expect: "    a\n        b\n    c",
}, {
	src: "\t  a",
	err: "Tabs and spaces mixed early on string:\n\t  a",
}, {
	src: "\t  a\n\t    b\n\t  c",
	err: "Tabs and spaces mixed early on string:\n\t  a\n\t    b\n\t  c",
}, {
	src: "\ta\nb",
	err: "Line not indented consistently:\nb",
}}

func (s *S) TestReindent(c *C) {
	for _, test := range reindentTests {
		s.testReindent(c, test)
	}
}

func (*S) testReindent(c *C, test reindentTest) {
	defer func() {
		if err := recover(); err != nil {
			errMsg, ok := err.(string)
			if !ok {
				panic(err)
			}
			c.Assert(errMsg, Equals, test.err)
		}
	}()

	c.Logf("Test: %#v", test)

	if !strings.HasSuffix(test.expect, "\n") {
		test.expect += "\n"
	}

	reindented := testutil.Reindent(test.src)
	if test.err != "" {
		c.Errorf("Expected panic with message '%#v'", test.err)
		return
	}
	c.Assert(string(reindented), Equals, test.expect)
}
