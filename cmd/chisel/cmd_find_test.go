package main_test

import (
	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/testutil"

	chisel "github.com/canonical/chisel/cmd/chisel"
)

type findTest struct {
	summary string
	release *setup.Release
	query   []string
	result  []*setup.Slice
}

func makeSamplePackage(pkg string, slices []string) *setup.Package {
	slicesMap := map[string]*setup.Slice{}
	for _, slice := range slices {
		slicesMap[slice] = &setup.Slice{
			Package: pkg,
			Name:    slice,
		}
	}
	return &setup.Package{
		Name:    pkg,
		Path:    "slices/" + pkg,
		Archive: "ubuntu",
		Slices:  slicesMap,
	}
}

var sampleRelease = &setup.Release{
	DefaultArchive: "ubuntu",

	Archives: map[string]*setup.Archive{
		"ubuntu": {
			Name:       "ubuntu",
			Version:    "22.04",
			Suites:     []string{"jammy", "jammy-security"},
			Components: []string{"main", "other"},
		},
	},
	Packages: map[string]*setup.Package{
		"openjdk-8-jdk": makeSamplePackage("openjdk-8-jdk", []string{"bins", "config", "core", "libs", "utils"}),
		"python3.10":    makeSamplePackage("python3.10", []string{"bins", "config", "core", "libs", "utils"}),
	},
}

var findTests = []findTest{{
	summary: "Search by package name",
	release: sampleRelease,
	query:   []string{"python3.10"},
	result: []*setup.Slice{
		sampleRelease.Packages["python3.10"].Slices["bins"],
		sampleRelease.Packages["python3.10"].Slices["config"],
		sampleRelease.Packages["python3.10"].Slices["core"],
		sampleRelease.Packages["python3.10"].Slices["libs"],
		sampleRelease.Packages["python3.10"].Slices["utils"],
	},
}, {
	summary: "Search by slice name",
	release: sampleRelease,
	query:   []string{"_config"},
	result: []*setup.Slice{
		sampleRelease.Packages["openjdk-8-jdk"].Slices["config"],
		sampleRelease.Packages["python3.10"].Slices["config"],
	},
}, {
	summary: "Slice search without leading underscore",
	release: sampleRelease,
	query:   []string{"config"},
	result:  []*setup.Slice{},
}, {
	summary: "Check distance greater than one",
	release: sampleRelease,
	query:   []string{"python3."},
	result:  []*setup.Slice{},
}, {
	summary: "Check glob matching (*)",
	release: sampleRelease,
	query:   []string{"python3.*_bins"},
	result: []*setup.Slice{
		sampleRelease.Packages["python3.10"].Slices["bins"],
	},
}, {
	summary: "Check glob matching (?)",
	release: sampleRelease,
	query:   []string{"python3.1?_co*"},
	result: []*setup.Slice{
		sampleRelease.Packages["python3.10"].Slices["config"],
		sampleRelease.Packages["python3.10"].Slices["core"],
	},
}, {
	summary: "Check no matching slice",
	release: sampleRelease,
	query:   []string{"foo_bar"},
	result:  []*setup.Slice{},
}, {
	summary: "Several terms all match",
	release: sampleRelease,
	query:   []string{"python*", "_co*"},
	result: []*setup.Slice{
		sampleRelease.Packages["python3.10"].Slices["config"],
		sampleRelease.Packages["python3.10"].Slices["core"],
	},
}, {
	summary: "Distance of one in each term",
	release: sampleRelease,
	query:   []string{"python3.1", "_lib"},
	result: []*setup.Slice{
		sampleRelease.Packages["python3.10"].Slices["libs"],
	},
}, {
	summary: "Query with underscore is matched against full name",
	release: sampleRelease,
	query:   []string{"python3.1_libs"},
	result: []*setup.Slice{
		sampleRelease.Packages["python3.10"].Slices["libs"],
	},
}, {
	summary: "Several terms, one does not match",
	release: sampleRelease,
	query:   []string{"python", "slice"},
	result:  []*setup.Slice{},
}}

func (s *ChiselSuite) TestFindSlices(c *C) {
	for _, test := range findTests {
		c.Logf("Summary: %s", test.summary)

		for _, query := range testutil.Permutations(test.query) {
			slices, err := chisel.FindSlices(test.release, query)
			c.Assert(err, IsNil)
			c.Assert(slices, DeepEquals, test.result)
		}
	}
}
