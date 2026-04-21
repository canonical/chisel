package strdist

import (
	"fmt"
	"strings"
)

type CostInt int64

func (cv CostInt) String() string {
	if cv == Inhibit {
		return "-"
	}
	return fmt.Sprint(int64(cv))
}

const Inhibit = 1<<63 - 1

type Cost struct {
	SwapAB  CostInt
	DeleteA CostInt
	InsertB CostInt
}

type CostFunc func(ar, br rune) Cost

func StandardCost(ar, br rune) Cost {
	return Cost{SwapAB: 1, DeleteA: 1, InsertB: 1}
}

func Distance(a, b string, f CostFunc, cut int64) int64 {
	if a == b {
		return 0
	}
	lst := make([]CostInt, len(b)+1)
	bl := 0
	for _, br := range b {
		cost := f(-1, br)
		if cost.InsertB == Inhibit || lst[bl] == Inhibit {
			lst[bl+1] = Inhibit
		} else {
			lst[bl+1] = lst[bl] + cost.InsertB
		}
		bl++
	}
	lst = lst[:bl+1]
	// Not required, but caching means preventing the fast path
	// below from calling the function and locking every time.
	debug := IsDebugOn()
	if debug {
		debugf(">>> %v", lst)
	}
	for _, ar := range a {
		last := lst[0]
		cost := f(ar, -1)
		if cost.DeleteA == Inhibit || last == Inhibit {
			lst[0] = Inhibit
		} else {
			lst[0] = last + cost.DeleteA
		}
		stop := true
		i := 0
		for _, br := range b {
			i++
			cost := f(ar, br)
			min := CostInt(Inhibit)
			if ar == br {
				min = last
			} else if cost.SwapAB != Inhibit && last != Inhibit {
				min = last + cost.SwapAB
			}
			if cost.InsertB != Inhibit && lst[i-1] != Inhibit {
				if n := lst[i-1] + cost.InsertB; n < min {
					min = n
				}
			}
			if cost.DeleteA != Inhibit && lst[i] != Inhibit {
				if n := lst[i] + cost.DeleteA; n < min {
					min = n
				}
			}
			last, lst[i] = lst[i], min
			if min < CostInt(cut) {
				stop = false
			}
		}
		if debug {
			debugf("... %v", lst)
		}
		if cut != 0 && len(b) > 0 && stop {
			break
		}
	}
	return int64(lst[len(lst)-1])
}

// GlobPath returns true if a and b match using supported wildcards.
// Note that both a and b main contain wildcards, and it's up to the
// call site to constrain the string content if that's not desirable.
//
// Supported wildcards:
//
//	?  - Any one character, except for /
//	*  - Any zero or more characters, except for /
//	** - Any zero or more characters, including /
func GlobPath(a, b string) bool {
	// Computing the actual distance is slow as its complexity is
	// O(len(a) * len(b)). If the paths contain globs there is no way around
	// it, but we can be clever about creating segments from the paths and only
	// calling the distance function when necessary. If there is no glob the
	// complexity becomes O(len(a) + len(b)).
	//
	// The algorithm works by separating the path into segments delimited by
	// '/'. We compare the segments in order for a and b, we have three cases:
	// 1) No segment uses globs, comparison is memcmp of both strings.
	// 2) One of the strings uses a single "*" or "?". We call the distance
	//    function only with the segment which reduces the algorithmic
	//    complexity by reducing the length.
	// 3) One of the strings uses "**". We need to call distance on the
	//    the rest of both strings and we can no longer rely on segments.
	//
	// Crucially, this optimization works because:
	// * There are few paths in the releases that use "**", because it results
	//   in conflict easily. In fact, it is usually used to extract a whole
	//   directory, meaning the prefix segements should be unique and we can
	//   avoid computing "**" completely.
	distance := func(a, b string) bool {
		a = strings.ReplaceAll(a, "**", "⁑")
		b = strings.ReplaceAll(b, "**", "⁑")
		return Distance(a, b, globCost, 1) == 0
	}

	// Returns the index where the segment ends (next '/' or the end of the
	// string) and whether the string has a "*" or "?". This function will only
	// traverse the string once.
	segmentEnd := func(s string) (end int, hasGlob bool) {
		end = strings.IndexAny(s, "*?/")
		if end == -1 {
			end = len(s) - 1
		} else if s[end] == '*' || s[end] == '?' {
			slash := strings.IndexRune(s[end:], '/')
			if slash != -1 {
				end = end + slash
			} else {
				end = len(s) - 1
			}
			hasGlob = true
		}
		return end, hasGlob
	}

	for len(a) > 0 && len(b) > 0 {
		endA, globA := segmentEnd(a)
		endB, globB := segmentEnd(b)

		segmentA := a[:endA+1]
		segmentB := b[:endB+1]
		if strings.Contains(segmentA, "**") || strings.Contains(segmentB, "**") {
			// We need to match the rest of the string with the slow path, no
			// other way around it.
			return distance(a, b)
		} else if globA || globB {
			if !distance(segmentA, segmentB) {
				return false
			}
		} else {
			if segmentA != segmentB {
				return false
			}
		}

		a = a[endA+1:]
		b = b[endB+1:]
	}

	// One string is empty, this call is linear.
	return distance(a, b)
}

func globCost(ar, br rune) Cost {
	if ar == '⁑' || br == '⁑' {
		return Cost{SwapAB: 0, DeleteA: 0, InsertB: 0}
	}
	if ar == '*' || br == '*' {
		if ar == '*' && br == '/' {
			return Cost{SwapAB: Inhibit, DeleteA: 0, InsertB: Inhibit}
		} else if ar == '/' && br == '*' {
			return Cost{SwapAB: Inhibit, DeleteA: Inhibit, InsertB: 0}
		}
		return Cost{SwapAB: 0, DeleteA: 0, InsertB: 0}
	}
	if ar == '/' || br == '/' {
		return Cost{SwapAB: Inhibit, DeleteA: Inhibit, InsertB: Inhibit}
	}
	if ar == '?' || br == '?' {
		return Cost{SwapAB: 0, DeleteA: 1, InsertB: 1}
	}
	return Cost{SwapAB: 1, DeleteA: 1, InsertB: 1}
}
