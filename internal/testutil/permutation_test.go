package testutil_test

import (
	"gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/testutil"
)

type permutationSuite struct{}

var _ = check.Suite(&permutationSuite{})

var permutationTests = []struct {
	slice []any
	res   [][]any
}{
	{
		slice: []any{1},
		res:   [][]any{{1}},
	},
	{
		slice: []any{1, 2},
		res:   [][]any{{1, 2}, {2, 1}},
	},
	{
		slice: []any{1, 2, 3},
		res:   [][]any{{1, 2, 3}, {2, 1, 3}, {3, 1, 2}, {1, 3, 2}, {2, 3, 1}, {3, 2, 1}},
	},
}

func (*permutationSuite) TestPermutations(c *check.C) {
	for _, test := range permutationTests {
		c.Assert(testutil.Permutations(test.slice), check.DeepEquals, test.res)
	}
}
