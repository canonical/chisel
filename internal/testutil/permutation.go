package testutil

func Permutations[S ~[]E, E any](s S) []S {
	var output []S
	// Heap's algorithm: https://en.wikipedia.org/wiki/Heap%27s_algorithm.
	var generate func(k int, s S)
	generate = func(k int, s S) {
		if k <= 1 {
			r := make([]E, len(s))
			copy(r, s)
			output = append(output, r)
			return
		}
		// Generate permutations with k-th unaltered.
		// Initially k = length(A).
		generate(k-1, s)

		// Generate permutations for k-th swapped with each k-1 initial.
		for i := 0; i < k-1; i += 1 {
			// Swap choice dependent on parity of k (even or odd).
			if k%2 == 0 {
				s[i], s[k-1] = s[k-1], s[i]
			} else {
				s[0], s[k-1] = s[k-1], s[0]
			}
			generate(k-1, s)
		}
	}

	sCpy := make([]E, len(s))
	copy(sCpy, s)
	generate(len(sCpy), sCpy)
	return output
}
