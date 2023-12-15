package testutil_test

import (
	"strings"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/testutil"
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
	raw:    "a\n\tb\nc",
	result: "a\n    b\nc",
}, {
	raw:    "a\n  b\nc",
	result: "a\n  b\nc",
}, {
	raw:    "\ta\n\t\tb\n\tc",
	result: "a\n    b\nc",
}, {
	raw:   "\t  a",
	error: "Space used in indent early in string:\n\t  a",
}, {
	raw:   "\t  a\n\t    b\n\t  c",
	error: "Space used in indent early in string:\n\t  a\n\t    b\n\t  c",
}, {
	raw:   "    a\nb",
	error: "Space used in indent early in string:\n    a\nb",
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

type prefixEachLineTest struct {
	raw, prefix, result string
}

var prefixEachLineTests = []prefixEachLineTest{{
	raw:    "a\n\tb\n  \t\tc\td\n\t ",
	prefix: "foo",
	result: "fooa\nfoo\tb\nfoo  \t\tc\td\nfoo\t ",
}, {
	raw:    "foo",
	prefix: "pref",
	result: "preffoo",
}, {
	raw:    "",
	prefix: "p",
	result: "p",
}, {
	raw:    "\n",
	prefix: "\t",
	result: "\t\n",
}, {
	raw:    "\n\n",
	prefix: "\t",
	result: "\t\n\t\n",
}}

func (s *S) TestPrefixEachLine(c *C) {
	for _, test := range prefixEachLineTests {
		c.Logf("Test: %#v", test)

		prefixed := testutil.PrefixEachLine(test.raw, test.prefix)
		c.Assert(prefixed, Equals, test.result)
	}
}
