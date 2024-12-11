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
	for bi, br := range b {
		bl++
		cost := f(-1, br)
		if cost.InsertB == Inhibit || lst[bi] == Inhibit {
			lst[bi+1] = Inhibit
		} else {
			lst[bi+1] = lst[bi] + cost.InsertB
		}
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
		_ = stop
		if cut != 0 && stop {
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
	if !wildcardPrefixMatch(a, b) {
		// Fast path.
		return false
	}
	if !wildcardSuffixMatch(a, b) {
		// Fast path.
		return false
	}

	a = strings.ReplaceAll(a, "**", "⁑")
	b = strings.ReplaceAll(b, "**", "⁑")
	return Distance(a, b, globCost, 1) == 0
}

func globCost(ar, br rune) Cost {
	if ar == '⁑' || br == '⁑' {
		return Cost{SwapAB: 0, DeleteA: 0, InsertB: 0}
	}
	if ar == '/' || br == '/' {
		return Cost{SwapAB: Inhibit, DeleteA: Inhibit, InsertB: Inhibit}
	}
	if ar == '*' || br == '*' {
		return Cost{SwapAB: 0, DeleteA: 0, InsertB: 0}
	}
	if ar == '?' || br == '?' {
		return Cost{SwapAB: 0, DeleteA: 1, InsertB: 1}
	}
	return Cost{SwapAB: 1, DeleteA: 1, InsertB: 1}
}

// wildcardPrefixMatch compares whether the prefixes of a and b are equal up
// to the shortest one. The prefix is defined as the longest substring that
// starts at index 0 and does not contain a wildcard.
func wildcardPrefixMatch(a, b string) bool {
	ai := strings.IndexAny(a, "*?")
	bi := strings.IndexAny(b, "*?")
	if ai == -1 {
		ai = len(a)
	}
	if bi == -1 {
		bi = len(b)
	}
	mini := min(ai, bi)
	return a[:mini] == b[:mini]
}

// wildcardSuffixMatch compares whether the suffixes of a and b are equal up
// to the shortest one. The suffix is defined as the longest substring that ends
// at the string length and does not contain a wildcard.
func wildcardSuffixMatch(a, b string) bool {
	ai := strings.LastIndexAny(a, "*?")
	la := 0
	if ai != -1 {
		la = len(a) - ai - 1
	}
	lb := 0
	bi := strings.LastIndexAny(b, "*?")
	if bi != -1 {
		lb = len(b) - bi - 1
	}
	minl := min(la, lb)
	return a[len(a)-minl:] == b[len(b)-minl:]
}
