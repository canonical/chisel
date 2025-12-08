package strdist

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidateSpecialGlob validates that a special glob character and its pattern are valid
func ValidateSpecialGlob(char rune, pattern string) error {
	// Validate that char is not a standard glob character
	if char == '*' || char == '?' || char == '/' {
		return fmt.Errorf("special-glob %q cannot be a standard glob character", string(char))
	}
	// Validate regex compiles
	_, err := regexp.Compile("^" + pattern + "$")
	if err != nil {
		return fmt.Errorf("special-glob %q has invalid regex pattern %q: %w", string(char), pattern, err)
	}
	return nil
}

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
	return GlobPathWithSpecial(a, b, nil, nil)
}

// GlobPathWithSpecial returns true if a and b match using supported wildcards
// and optional special glob characters defined in aSpecial and bSpecial.
// Special globs are custom single-character wildcards with regex patterns.
func GlobPathWithSpecial(a, b string, aSpecial, bSpecial map[rune]string) bool {
	if !wildcardPrefixMatchWithSpecial(a, b, aSpecial, bSpecial) {
		// Fast path.
		return false
	}
	if !wildcardSuffixMatchWithSpecial(a, b, aSpecial, bSpecial) {
		// Fast path.
		return false
	}

	a = strings.ReplaceAll(a, "**", "⁑")
	b = strings.ReplaceAll(b, "**", "⁑")

	costFunc := makeGlobCostFunc(aSpecial, bSpecial)
	return Distance(a, b, costFunc, 1) == 0
}

func makeGlobCostFunc(aSpecial, bSpecial map[rune]string) CostFunc {
	// Compile regex patterns once
	aRegex := make(map[rune]*regexp.Regexp)
	bRegex := make(map[rune]*regexp.Regexp)

	for r, pattern := range aSpecial {
		re, err := regexp.Compile("^" + pattern + "$")
		if err == nil {
			aRegex[r] = re
		}
	}
	for r, pattern := range bSpecial {
		re, err := regexp.Compile("^" + pattern + "$")
		if err == nil {
			bRegex[r] = re
		}
	}

	return func(ar, br rune) Cost {
		// Handle standard wildcards first
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

		// Handle special globs
		if re, ok := aRegex[ar]; ok {
			// ar is a special glob from a
			if br == '/' {
				// Special globs cannot match / unless explicitly allowed
				if !re.MatchString("/") {
					return Cost{SwapAB: Inhibit, DeleteA: 1, InsertB: Inhibit}
				}
			}
			// Special glob matches any single character matching its pattern
			if re.MatchString(string(br)) {
				return Cost{SwapAB: 0, DeleteA: 1, InsertB: Inhibit}
			}
			return Cost{SwapAB: Inhibit, DeleteA: 1, InsertB: Inhibit}
		}
		if re, ok := bRegex[br]; ok {
			// br is a special glob from b
			if ar == '/' {
				if !re.MatchString("/") {
					return Cost{SwapAB: Inhibit, DeleteA: Inhibit, InsertB: 1}
				}
			}
			if re.MatchString(string(ar)) {
				return Cost{SwapAB: 0, DeleteA: Inhibit, InsertB: 1}
			}
			return Cost{SwapAB: Inhibit, DeleteA: Inhibit, InsertB: 1}
		}

		return Cost{SwapAB: 1, DeleteA: 1, InsertB: 1}
	}
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
	return wildcardPrefixMatchWithSpecial(a, b, nil, nil)
}

func wildcardPrefixMatchWithSpecial(a, b string, aSpecial, bSpecial map[rune]string) bool {
	ai := indexAnyWildcard(a, aSpecial)
	bi := indexAnyWildcard(b, bSpecial)
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
	return wildcardSuffixMatchWithSpecial(a, b, nil, nil)
}

func wildcardSuffixMatchWithSpecial(a, b string, aSpecial, bSpecial map[rune]string) bool {
	ai := lastIndexAnyWildcard(a, aSpecial)
	la := 0
	if ai != -1 {
		la = len(a) - ai - 1
	}
	lb := 0
	bi := lastIndexAnyWildcard(b, bSpecial)
	if bi != -1 {
		lb = len(b) - bi - 1
	}
	minl := min(la, lb)
	return a[len(a)-minl:] == b[len(b)-minl:]
}

// indexAnyWildcard returns the index of the first wildcard in s
func indexAnyWildcard(s string, special map[rune]string) int {
	for i, r := range s {
		if r == '*' || r == '?' {
			return i
		}
		if special != nil {
			if _, ok := special[r]; ok {
				return i
			}
		}
	}
	return -1
}

// lastIndexAnyWildcard returns the index of the last wildcard in s
func lastIndexAnyWildcard(s string, special map[rune]string) int {
	runes := []rune(s)
	for i := len(runes) - 1; i >= 0; i-- {
		r := runes[i]
		if r == '*' || r == '?' {
			// Calculate byte position
			return len(string(runes[:i]))
		}
		if special != nil {
			if _, ok := special[r]; ok {
				return len(string(runes[:i]))
			}
		}
	}
	return -1
}
