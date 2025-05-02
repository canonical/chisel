// SPDX-License-Identifier: Apache-2.0

package apacheutil_test

import (
	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/apacheutil"
)

var sliceKeyTests = []struct {
	input    string
	expected apacheutil.SliceKey
	err      string
}{{
	input:    "foo_bar",
	expected: apacheutil.SliceKey{Package: "foo", Slice: "bar"},
}, {
	input:    "fo_bar",
	expected: apacheutil.SliceKey{Package: "fo", Slice: "bar"},
}, {
	input:    "1234_bar",
	expected: apacheutil.SliceKey{Package: "1234", Slice: "bar"},
}, {
	input:    "foo1.1-2-3_bar",
	expected: apacheutil.SliceKey{Package: "foo1.1-2-3", Slice: "bar"},
}, {
	input:    "foo-pkg_dashed-slice-name",
	expected: apacheutil.SliceKey{Package: "foo-pkg", Slice: "dashed-slice-name"},
}, {
	input:    "foo+_bar",
	expected: apacheutil.SliceKey{Package: "foo+", Slice: "bar"},
}, {
	input:    "foo_slice123",
	expected: apacheutil.SliceKey{Package: "foo", Slice: "slice123"},
}, {
	input:    "g++_bins",
	expected: apacheutil.SliceKey{Package: "g++", Slice: "bins"},
}, {
	input:    "a+_bar",
	expected: apacheutil.SliceKey{Package: "a+", Slice: "bar"},
}, {
	input:    "a._bar",
	expected: apacheutil.SliceKey{Package: "a.", Slice: "bar"},
}, {
	input: "foo_ba",
	err:   `invalid slice reference: "foo_ba"`,
}, {
	input: "f_bar",
	err:   `invalid slice reference: "f_bar"`,
}, {
	input: "1234_789",
	err:   `invalid slice reference: "1234_789"`,
}, {
	input: "foo_bar.x.y",
	err:   `invalid slice reference: "foo_bar.x.y"`,
}, {
	input: "foo-_-bar",
	err:   `invalid slice reference: "foo-_-bar"`,
}, {
	input: "foo_bar-",
	err:   `invalid slice reference: "foo_bar-"`,
}, {
	input: "foo-_bar",
	err:   `invalid slice reference: "foo-_bar"`,
}, {
	input: "-foo_bar",
	err:   `invalid slice reference: "-foo_bar"`,
}, {
	input: "foo_bar_baz",
	err:   `invalid slice reference: "foo_bar_baz"`,
}, {
	input: "a-_bar",
	err:   `invalid slice reference: "a-_bar"`,
}, {
	input: "+++_bar",
	err:   `invalid slice reference: "\+\+\+_bar"`,
}, {
	input: "..._bar",
	err:   `invalid slice reference: "\.\.\._bar"`,
}, {
	input: "white space_no-whitespace",
	err:   `invalid slice reference: "white space_no-whitespace"`,
}}

func (s *S) TestParseSliceKey(c *C) {
	for _, test := range sliceKeyTests {
		key, err := apacheutil.ParseSliceKey(test.input)
		if test.err != "" {
			c.Assert(err, ErrorMatches, test.err)
			continue
		}
		c.Assert(err, IsNil)
		c.Assert(key, DeepEquals, test.expected)
	}
}
