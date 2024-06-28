package strdist_test

import (
	. "gopkg.in/check.v1"

	"strings"
	"testing"

	"github.com/canonical/chisel/internal/strdist"
)

type distanceTest struct {
	a, b string
	f    strdist.CostFunc
	r    int64
	cut  int64
}

func uniqueCost(ar, br rune) strdist.Cost {
	return strdist.Cost{SwapAB: 1, DeleteA: 3, InsertB: 5}
}

var distanceTests = []distanceTest{
	{f: uniqueCost, r: 0, a: "abc", b: "abc"},
	{f: uniqueCost, r: 1, a: "abc", b: "abd"},
	{f: uniqueCost, r: 1, a: "abc", b: "adc"},
	{f: uniqueCost, r: 1, a: "abc", b: "dbc"},
	{f: uniqueCost, r: 2, a: "abc", b: "add"},
	{f: uniqueCost, r: 2, a: "abc", b: "ddc"},
	{f: uniqueCost, r: 2, a: "abc", b: "dbd"},
	{f: uniqueCost, r: 3, a: "abc", b: "ddd"},
	{f: uniqueCost, r: 3, a: "abc", b: "ab"},
	{f: uniqueCost, r: 3, a: "abc", b: "bc"},
	{f: uniqueCost, r: 3, a: "abc", b: "ac"},
	{f: uniqueCost, r: 6, a: "abc", b: "a"},
	{f: uniqueCost, r: 6, a: "abc", b: "b"},
	{f: uniqueCost, r: 6, a: "abc", b: "c"},
	{f: uniqueCost, r: 9, a: "abc", b: ""},
	{f: uniqueCost, r: 5, a: "abc", b: "abcd"},
	{f: uniqueCost, r: 5, a: "abc", b: "dabc"},
	{f: uniqueCost, r: 10, a: "abc", b: "adbdc"},
	{f: uniqueCost, r: 10, a: "abc", b: "dabcd"},
	{f: uniqueCost, r: 40, a: "abc", b: "ddaddbddcdd"},
	{f: strdist.StandardCost, r: 3, a: "abcdefg", b: "axcdfgh"},
	{f: strdist.StandardCost, r: 2, cut: 2, a: "abcdef", b: "abc"},
	{f: strdist.StandardCost, r: 2, cut: 3, a: "abcdef", b: "abcd"},
	{f: strdist.GlobCost, r: 0, a: "abc*", b: "abcdef"},
	{f: strdist.GlobCost, r: 0, a: "ab*ef", b: "abcdef"},
	{f: strdist.GlobCost, r: 0, a: "*def", b: "abcdef"},
	{f: strdist.GlobCost, r: 0, a: "a*/def", b: "abc/def"},
	{f: strdist.GlobCost, r: 1, a: "a*/def", b: "abc/gef"},
	{f: strdist.GlobCost, r: 0, a: "a*/*f", b: "abc/def"},
	{f: strdist.GlobCost, r: 1, a: "a*/*f", b: "abc/defh"},
	{f: strdist.GlobCost, r: 1, a: "a*/*f", b: "abc/defhi"},
	{f: strdist.GlobCost, r: strdist.Inhibit, a: "a*", b: "abc/def"},
	{f: strdist.GlobCost, r: strdist.Inhibit, a: "a*/*f", b: "abc/def/hij"},
	{f: strdist.GlobCost, r: 0, a: "a**f/hij", b: "abc/def/hij"},
	{f: strdist.GlobCost, r: 1, a: "a**f/hij", b: "abc/def/hik"},
	{f: strdist.GlobCost, r: 2, a: "a**fg", b: "abc/def/hik"},
	{f: strdist.GlobCost, r: 0, a: "a**f/hij/klm", b: "abc/d**m"},
}

func (s *S) TestDistance(c *C) {
	for _, test := range distanceTests {
		c.Logf("Test: %v", test)
		if strings.Contains(test.a, "*") || strings.Contains(test.b, "*") {
			c.Assert(strdist.GlobPath(test.a, test.b), Equals, test.r == 0)
		}
		f := test.f
		if f == nil {
			f = strdist.StandardCost
		}
		test.a = strings.ReplaceAll(test.a, "**", "⁑")
		test.b = strings.ReplaceAll(test.b, "**", "⁑")
		r := strdist.Distance(test.a, test.b, f, test.cut)
		c.Assert(r, Equals, test.r)
	}
}

func BenchmarkDistance(b *testing.B) {
	const one = "abdefghijklmnopqrstuvwxyz"
	const two = "a.d.f.h.j.l.n.p.r.t.v.x.z"
	for i := 0; i < b.N; i++ {
		strdist.Distance(one, two, strdist.StandardCost, 0)
	}
}

func BenchmarkDistanceCut(b *testing.B) {
	const one = "abdefghijklmnopqrstuvwxyz"
	const two = "a.d.f.h.j.l.n.p.r.t.v.x.z"
	for i := 0; i < b.N; i++ {
		strdist.Distance(one, two, strdist.StandardCost, 1)
	}
}
