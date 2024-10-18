package main_test

import (
	"os"
	"path/filepath"
	"strings"

	. "gopkg.in/check.v1"

	chisel "github.com/canonical/chisel/cmd/chisel"
	"github.com/canonical/chisel/internal/testutil"
)

type infoTest struct {
	summary string
	input   map[string]string
	query   []string
	err     string
	stdout  string
}

var infoTests = []infoTest{{
	summary: "A single slice inspection",
	input:   infoRelease,
	query:   []string{"mypkg1_myslice1"},
	stdout: `
		package: mypkg1
		slices:
			myslice1:
				contents:
					/dir/file: {}
	`,
}, {
	summary: "A single package inspection",
	input:   infoRelease,
	query:   []string{"mypkg2"},
	stdout: `
		package: mypkg2
		slices:
			myslice:
				contents:
					/dir/another-file: {}
	`,
}, {
	summary: "Multiple slices within the same package",
	input:   infoRelease,
	query:   []string{"mypkg1_myslice2", "mypkg1_myslice1"},
	stdout: `
		package: mypkg1
		slices:
			myslice1:
				contents:
					/dir/file: {}
			myslice2:
				essential:
					- mypkg1_myslice1
					- mypkg2_myslice
	`,
}, {
	summary: "Packages and slices",
	input:   infoRelease,
	query:   []string{"mypkg1_myslice1", "mypkg2", "mypkg1_myslice2"},
	stdout: `
		package: mypkg1
		slices:
			myslice1:
				contents:
					/dir/file: {}
			myslice2:
				essential:
					- mypkg1_myslice1
					- mypkg2_myslice
		---
		package: mypkg2
		slices:
			myslice:
				contents:
					/dir/another-file: {}
	`,
}, {
	summary: "Package and its slices",
	input:   infoRelease,
	query:   []string{"mypkg1_myslice1", "mypkg1"},
	stdout: `
		package: mypkg1
		slices:
			myslice1:
				contents:
					/dir/file: {}
			myslice2:
				essential:
					- mypkg1_myslice1
					- mypkg2_myslice
	`,
}, {
	summary: "Same slice appearing multiple times",
	input:   infoRelease,
	query:   []string{"mypkg1_myslice1", "mypkg1_myslice1", "mypkg1_myslice1"},
	stdout: `
		package: mypkg1
		slices:
			myslice1:
				contents:
					/dir/file: {}
	`,
}, {
	summary: "No slices found",
	input:   infoRelease,
	query:   []string{"foo", "bar_foo"},
	err:     `no slice definitions found for: "foo", "bar_foo"`,
}, {
	summary: "Some slices found, others not found",
	input:   infoRelease,
	query:   []string{"foo", "mypkg1_myslice1", "bar_foo"},
	stdout: `
		package: mypkg1
		slices:
			myslice1:
				contents:
					/dir/file: {}
					/dir/sub-dir/: {make: true, mode: 0644}
	`,
	err: `no slice definitions found for: "foo", "bar_foo"`,
}, {
	summary: "No args",
	input:   infoRelease,
	err:     "the required argument `<pkg|slice> (at least 1 argument)` was not provided",
}, {
	summary: "Empty, whitespace args",
	input:   infoRelease,
	query:   []string{"", "    "},
	err:     `no slice definitions found for: "", "    "`,
}, {
	summary: "Ignore invalid slice names",
	input:   infoRelease,
	query:   []string{"foo_bar_foo", "a_b", "7_c", "a_b c", "a_b x_y"},
	err:     `no slice definitions found for: "foo_bar_foo", "a_b", "7_c", "a_b c", "a_b x_y"`,
}}

var testKey = testutil.PGPKeys["key1"]

var defaultChiselYaml = `
	format: v1
	archives:
		ubuntu:
			version: 22.04
			components: [main, universe]
			suites: [jammy]
			public-keys: [test-key]
	public-keys:
		test-key:
			id: ` + testKey.ID + `
			armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t")

var infoRelease = map[string]string{
	"chisel.yaml": string(defaultChiselYaml),
	"slices/mypkg1.yaml": `
		package: mypkg1
		essential:
			- mypkg1_myslice1
		slices:
			myslice1:
				contents:
					/dir/file:
			myslice2:
				essential:
					- mypkg2_myslice
	`,
	"slices/mypkg2.yaml": `
		package: mypkg2
		slices:
			myslice:
				contents:
					/dir/another-file:
	`,
	"slices/mypkg3.yaml": `
		package: mypkg3
		essential:
			- mypkg1_myslice1
		slices:
			myslice:
				essential:
					- mypkg2_myslice
				contents:
					/dir/other-file:
					/dir/glob*:
					/dir/sub-dir/:       {make: true, mode: 0644}
					/dir/copy:           {copy: /dir/file}
					/dir/symlink:        {symlink: /dir/file}
					/dir/mutable:        {text: TODO, mutable: true, arch: riscv64}
					/dir/arch-specific*: {arch: [amd64,arm64,i386]}
					/dir/until:          {until: mutate}
					/dir/unfolded:
						copy: /dir/file
						mode: 0644
				mutate: |
					# Test multi-line string.
					content.write("/dir/mutable", foo)
	`,
}

func (s *ChiselSuite) TestInfoCommand(c *C) {
	for _, test := range infoTests {
		c.Logf("Summary: %s", test.summary)

		s.ResetStdStreams()

		dir := c.MkDir()
		for path, data := range test.input {
			fpath := filepath.Join(dir, path)
			err := os.MkdirAll(filepath.Dir(fpath), 0755)
			c.Assert(err, IsNil)
			err = os.WriteFile(fpath, testutil.Reindent(data), 0644)
			c.Assert(err, IsNil)
		}
		test.query = append([]string{"info", "--release", dir}, test.query...)

		_, err := chisel.Parser().ParseArgs(test.query)
		if test.err != "" {
			c.Assert(err, ErrorMatches, test.err)
			continue
		}
		c.Assert(err, IsNil)
		test.stdout = string(testutil.Reindent(test.stdout))
		c.Assert(s.Stdout(), Equals, strings.TrimSpace(test.stdout)+"\n")
	}
}
