package testutil_test

import (
	"strings"

	"github.com/canonical/chisel/internal/testutil"
	. "gopkg.in/check.v1"
)

type reindentTest struct {
	raw, result, error string
}

var reindentTests = []reindentTest{{
	raw:    "a\nb",
	result: "a\nb",
}, {
	raw:    "\ta\n\tb",
	result: "a\nb",
}, {
	raw:    "    a\n    b",
	result: "    a\n    b",
}, {
	raw:    "a\n\tb\nc",
	result: "a\n    b\nc",
}, {
	raw:    "a\n  b\nc",
	result: "a\n  b\nc",
}, {
	raw:    "\ta\n\t\tb\n\tc",
	result: "a\n    b\nc",
}, {
	raw:    "  a\n    b\n  c",
	result: "  a\n    b\n  c",
}, {
	raw:    "    a\n    \tb\n    c",
	result: "    a\n        b\n    c",
}, {
	raw:   "\t  a",
	error: "Tabs and spaces mixed early on string:\n\t  a",
}, {
	raw:   "\t  a\n\t    b\n\t  c",
	error: "Tabs and spaces mixed early on string:\n\t  a\n\t    b\n\t  c",
}, {
	raw:   "\ta\nb",
	error: "Line not indented consistently:\nb",
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
			c.Assert(errMsg, Equals, test.error)
		}
	}()

	c.Logf("Test: %#v", test)

	if !strings.HasSuffix(test.result, "\n") {
		test.result += "\n"
	}

	reindented := testutil.Reindent(test.raw)
	if test.error != "" {
		c.Errorf("Expected panic with message '%#v'", test.error)
		return
	}
	c.Assert(string(reindented), Equals, test.result)
}
