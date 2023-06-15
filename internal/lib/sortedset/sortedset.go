package sortedset

import "sort"

type String []string

func (s String) AddOne(x string) (String, bool) {
	if s == nil {
		return []string{x}, true
	}
	i := sort.SearchStrings(s, x)
	if i == len(s) {
		s = append(s, x)
	} else if s[i] != x {
		s = append(s[:i], append([]string{x}, s[i:]...)...)
	} else {
		return s, false
	}
	return s, true
}

func (s String) AddMany(xs ...string) String {
	for _, x := range xs {
		s, _ = s.AddOne(x)
	}
	return s
}
