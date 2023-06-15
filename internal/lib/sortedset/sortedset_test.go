package sortedset_test

import (
	"github.com/canonical/chisel/internal/lib/sortedset"
	. "gopkg.in/check.v1"
)

type stringStortedSetAddOneTest struct {
	set    sortedset.String
	add    string
	added  bool
	result sortedset.String
}

var stringStortedSetAddOneTests = []stringStortedSetAddOneTest{
	{[]string{"a", "b", "c"}, "", true, []string{"", "a", "b", "c"}},
	{[]string{"a", "b", "c"}, "a", false, []string{"a", "b", "c"}},
	{[]string{"b", "d"}, "a", true, []string{"a", "b", "d"}},
	{[]string{"b", "d"}, "c", true, []string{"b", "c", "d"}},
	{[]string{"b", "d"}, "e", true, []string{"b", "d", "e"}},
	{[]string{"a", "b", "b", "c"}, "b", false, []string{"a", "b", "b", "c"}},
	{[]string{}, "a", true, []string{"a"}},
	{nil, "a", true, []string{"a"}},
}

func (s *S) TestStringAddOne(c *C) {
	for _, test := range stringStortedSetAddOneTests {
		result, added := test.set.AddOne(test.add)
		c.Assert(result, DeepEquals, test.result)
		c.Assert(added, DeepEquals, test.added)
	}
}

type stringStortedSetAddManyTest struct {
	set    sortedset.String
	add    []string
	result sortedset.String
}

var stringStortedSetAddManyTests = []stringStortedSetAddManyTest{
	{[]string{"b", "d"}, []string{}, []string{"b", "d"}},
	{[]string{"b", "d"}, nil, []string{"b", "d"}},
	{[]string{}, []string{}, []string{}},
	{nil, []string{}, nil},
	{nil, nil, nil},
	{[]string{}, []string{"a", "b"}, []string{"a", "b"}},
	{nil, []string{"a", "b"}, []string{"a", "b"}},
	{[]string{"b", "d"}, []string{"c", "a"}, []string{"a", "b", "c", "d"}},
	{[]string{"b", "d"}, []string{"c", "c"}, []string{"b", "c", "d"}},
	{[]string{"b", "d"}, []string{"b", "a", "b"}, []string{"a", "b", "d"}},
}

func (s *S) TestStringAddMany(c *C) {
	for _, test := range stringStortedSetAddManyTests {
		result := test.set.AddMany(test.add...)
		c.Assert(result, DeepEquals, test.result)
	}
}
