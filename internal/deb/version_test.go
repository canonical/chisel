// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2014-2015 Canonical Ltd
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

package deb_test

import (
	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/deb"
)

type VersionTestSuite struct{}

var _ = Suite(&VersionTestSuite{})

func (s *VersionTestSuite) TestVersionCompare(c *C) {
	for _, t := range []struct {
		A, B string
		res  int
	}{
		{"20000000000000000000", "020000000000000000000", 0},
		{"1.0", "2.0", -1},
		{"1.3", "1.2.2.2", 1},
		{"1.3", "1.3.1", -1},
		{"1.0", "1.0~", 1},
		{"7.2p2", "7.2", 1},
		{"0.4a6", "0.4", 1},
		{"0pre", "0pre", 0},
		{"0pree", "0pre", 1},
		{"1.18.36:5.4", "1.18.36:5.5", -1},
		{"1.18.36:5.4", "1.18.37:1.1", -1},
		{"2.0.7pre1", "2.0.7r", -1},
		{"0.10.0", "0.8.7", 1},
		// subrev
		{"1.0-1", "1.0-2", -1},
		{"1.0-1.1", "1.0-1", 1},
		{"1.0-1.1", "1.0-1.1", 0},
		// do we like strange versions? Yes we like strange versions…
		{"0", "0", 0},
		{"0", "00", 0},
		{"", "", 0},
		{"", "0", -1},
		{"0", "", 1},
		{"", "~", 1},
		{"~", "", -1},
		// from the apt suite
		{"0-pre", "0-pre", 0},
		{"0-pre", "0-pree", -1},
		{"1.1.6r2-2", "1.1.6r-1", 1},
		{"2.6b2-1", "2.6b-2", 1},
		{"0.4a6-2", "0.4-1", 1},
		{"3.0~rc1-1", "3.0-1", -1},
		{"1.0", "1.0-0", 0},
		{"0.2", "1.0-0", -1},
		{"1.0", "1.0-0+b1", -1},
		{"1.0", "1.0-0~", 1},
		// from the old perl cupt
		{"1.2.3", "1.2.3", 0},                   // identical
		{"4.4.3-2", "4.4.3-2", 0},               // identical
		{"1.2.3", "1.2.3-0", 0},                 // zero revision
		{"009", "9", 0},                         // zeroes…
		{"009ab5", "9ab5", 0},                   // there as well
		{"1.2.3", "1.2.3-1", -1},                // added non-zero revision
		{"1.2.3", "1.2.4", -1},                  // just bigger
		{"1.2.4", "1.2.3", 1},                   // order doesn't matter
		{"1.2.24", "1.2.3", 1},                  // bigger, eh?
		{"0.10.0", "0.8.7", 1},                  // bigger, eh?
		{"3.2", "2.3", 1},                       // major number rocks
		{"1.3.2a", "1.3.2", 1},                  // letters rock
		{"0.5.0~git", "0.5.0~git2", -1},         // numbers rock
		{"2a", "21", -1},                        // but not in all places
		{"1.2a+~bCd3", "1.2a++", -1},            // tilde doesn't rock
		{"1.2a+~bCd3", "1.2a+~", 1},             // but first is longer!
		{"5.10.0", "5.005", 1},                  // preceding zeroes don't matters
		{"3a9.8", "3.10.2", -1},                 // letters are before all letter symbols
		{"3a9.8", "3~10", 1},                    // but after the tilde
		{"1.4+OOo3.0.0~", "1.4+OOo3.0.0-4", -1}, // another tilde check
		{"2.4.7-1", "2.4.7-z", -1},              // revision comparing
		{"1.002-1+b2", "1.00", 1},               // whatever...
		{"12-20220319-1ubuntu1", "12-20220319-1ubuntu2", -1}, // libgcc-s1
		{"1:13.0.1-2ubuntu2", "1:13.0.1-2ubuntu3", -1},
	} {
		res := deb.CompareVersions(t.A, t.B)
		c.Assert(res, Equals, t.res, Commentf("%#v %#v: %v but got %v", t.A, t.B, res, t.res))
	}
}
