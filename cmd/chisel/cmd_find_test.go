package main_test

import (
	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/setup"

	chisel "github.com/canonical/chisel/cmd/chisel"
)

type findTest struct {
	summary   string
	release   *setup.Release
	query     string
	expSlices []*setup.Slice
	expError  string
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
		"openjdk-8-jdk": {
			Archive: "ubuntu",
			Name:    "openjdk-8-jdk",
			Path:    "slices/openjdk-8-jdk.yaml",
			Slices: map[string]*setup.Slice{
				"bins": {
					Package: "openjdk-8-jdk",
					Name:    "bins",
				},
				"config": {
					Package: "openjdk-8-jdk",
					Name:    "config",
				},
				"core": {
					Package: "openjdk-8-jdk",
					Name:    "core",
				},
				"libs": {
					Package: "openjdk-8-jdk",
					Name:    "libs",
				},
				"utils": {
					Package: "openjdk-8-jdk",
					Name:    "utils",
				},
			},
		},
		"python3.10": {
			Archive: "ubuntu",
			Name:    "python3.10",
			Path:    "slices/python3.10.yaml",
			Slices: map[string]*setup.Slice{
				"bins": {
					Package: "python3.10",
					Name:    "bins",
				},
				"config": {
					Package: "python3.10",
					Name:    "config",
				},
				"core": {
					Package: "python3.10",
					Name:    "core",
				},
				"libs": {
					Package: "python3.10",
					Name:    "libs",
				},
				"utils": {
					Package: "python3.10",
					Name:    "utils",
				},
			},
		},
	},
}

var findTests = []findTest{{
	summary: "Ensure search with package names",
	release: sampleRelease,
	query:   "python3.10",
	expSlices: []*setup.Slice{
		sampleRelease.Packages["python3.10"].Slices["bins"],
		sampleRelease.Packages["python3.10"].Slices["config"],
		sampleRelease.Packages["python3.10"].Slices["core"],
		sampleRelease.Packages["python3.10"].Slices["libs"],
		sampleRelease.Packages["python3.10"].Slices["utils"],
	},
}, {
	summary: "Ensure search with slice names",
	release: sampleRelease,
	query:   "config",
	expSlices: []*setup.Slice{
		sampleRelease.Packages["openjdk-8-jdk"].Slices["config"],
		sampleRelease.Packages["python3.10"].Slices["config"],
	},
}, {
	summary: "Check substring matching",
	release: sampleRelease,
	query:   "ython",
	expSlices: []*setup.Slice{
		sampleRelease.Packages["python3.10"].Slices["bins"],
		sampleRelease.Packages["python3.10"].Slices["config"],
		sampleRelease.Packages["python3.10"].Slices["core"],
		sampleRelease.Packages["python3.10"].Slices["libs"],
		sampleRelease.Packages["python3.10"].Slices["utils"],
	},
}, {
	summary: "Check partial matching",
	release: sampleRelease,
	query:   "python3.1x_bins",
	expSlices: []*setup.Slice{
		sampleRelease.Packages["python3.10"].Slices["bins"],
	},
}, {
	summary:   "Check no matching slice",
	release:   sampleRelease,
	query:     "foo_bar",
	expSlices: nil,
}, {
	summary:  "Ensure error for nil release",
	query:    "foo",
	expError: ".*invalid release",
}}

func (s *ChiselSuite) TestFindSlices(c *C) {
	for _, test := range findTests {
		c.Logf("Summary: %s", test.summary)

		slices, err := chisel.FindSlices(test.release, test.query)
		if test.expError == "" {
			c.Assert(err, IsNil)
		} else {
			c.Assert(err, ErrorMatches, test.expError)
		}
		c.Assert(slices, DeepEquals, test.expSlices)
	}
}

func (s *ChiselSuite) TestFindCommandEmptyQuery(c *C) {
	_, err := chisel.Parser().ParseArgs([]string{"find", ""})
	c.Assert(err, ErrorMatches, ".*no search term specified")
}
