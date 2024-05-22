package testutil_test

import (
	"sort"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/testutil"
)

type permutationSuite struct{}

var _ = Suite(&permutationSuite{})

var permutationTests = []struct {
	slice []any
	res   [][]any
}{
	{
		slice: []any{},
		res:   [][]any{{}},
	},
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

func (*permutationSuite) TestPermutations(c *C) {
	for _, test := range permutationTests {
		c.Assert(testutil.Permutations(test.slice), DeepEquals, test.res)
	}
}

func (*permutationSuite) TestFuzzPermutations(c *C) {
	for sLen := 0; sLen <= 10; sLen++ {
		s := make([]byte, sLen)
		for i := 0; i < sLen; i++ {
			s[i] = byte(i)
		}
		permutations := testutil.Permutations(s)

		// Factorial.
		expectedLen := 1
		for i := 2; i <= len(s); i++ {
			expectedLen *= i
		}
		c.Assert(len(permutations), Equals, expectedLen)

		duplicated := map[string]bool{}
		for _, perm := range permutations {
			// []byte is not comparable.
			permStr := string(perm)
			if _, ok := duplicated[permStr]; ok {
				c.Fatalf("duplicated permutation: %v", perm)
			}
			duplicated[permStr] = true
			// Check that the elements are the same.
			sort.Slice(perm, func(i, j int) bool {
				return perm[i] < perm[j]
			})
			c.Assert(perm, DeepEquals, s, Commentf("invalid elements in permutation %v of base slice %v", perm, s))
		}
	}
}
