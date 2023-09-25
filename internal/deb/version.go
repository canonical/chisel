// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2014-2017 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package deb

import (
	"strings"
)

func max(a, b int) int {
	if a < b {
		return b
	}
	return a
}

//go:generate go run ./chrorder/main.go -package=deb -output=chrorder.go

func cmpString(as, bs string) int {
	for i := 0; i < max(len(as), len(bs)); i++ {
		var a uint8
		var b uint8
		if i < len(as) {
			a = as[i]
		}
		if i < len(bs) {
			b = bs[i]
		}
		if chOrder[a] < chOrder[b] {
			return -1
		}
		if chOrder[a] > chOrder[b] {
			return +1
		}
	}
	return 0
}

func trimLeadingZeroes(a string) string {
	for i := 0; i < len(a); i++ {
		if a[i] != '0' {
			return a[i:]
		}
	}
	return ""
}

// a and b both match /[0-9]+/
func cmpNumeric(a, b string) int {
	a = trimLeadingZeroes(a)
	b = trimLeadingZeroes(b)

	switch d := len(a) - len(b); {
	case d > 0:
		return 1
	case d < 0:
		return -1
	}
	for i := 0; i < len(a); i++ {
		switch {
		case a[i] > b[i]:
			return 1
		case a[i] < b[i]:
			return -1
		}
	}
	return 0
}

func nextFrag(s string) (frag, rest string, numeric bool) {
	if len(s) == 0 {
		return "", "", false
	}

	var i int
	if s[0] >= '0' && s[0] <= '9' {
		// is digit
		for i = 1; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		}
		numeric = true
	} else {
		// not digit
		for i = 1; i < len(s) && (s[i] < '0' || s[i] > '9'); i++ {
		}
	}
	return s[:i], s[i:], numeric
}

func compareSubversion(va, vb string) int {
	var a, b string
	var anum, bnum bool
	var res int
	for res == 0 {
		a, va, anum = nextFrag(va)
		b, vb, bnum = nextFrag(vb)
		if a == "" && b == "" {
			break
		}
		if anum && bnum {
			res = cmpNumeric(a, b)
		} else {
			res = cmpString(a, b)
		}
	}
	return res
}

// CompareVersions compare two version strings that follow the debian
// version policy and
// Returns:
//
//	-1 if a is smaller than b
//	 0 if a equals b
//	+1 if a is bigger than b
func CompareVersions(va, vb string) int {
	var sa, sb string
	if ia := strings.IndexByte(va, '-'); ia < 0 {
		sa = "0"
	} else {
		va, sa = va[:ia], va[ia+1:]
	}
	if ib := strings.IndexByte(vb, '-'); ib < 0 {
		sb = "0"
	} else {
		vb, sb = vb[:ib], vb[ib+1:]
	}

	// the main version number (before the "-")
	res := compareSubversion(va, vb)
	if res != 0 {
		return res
	}

	// the subversion revision behind the "-"
	return compareSubversion(sa, sb)
}
