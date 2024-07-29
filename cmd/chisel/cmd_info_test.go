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
	query:   []string{"mypkg_foo"},
	stdout: `
		package: mypkg
		archive: ubuntu
		slices:
			foo:
				contents:
					/etc/foo: {}
					/etc/foo-dir/: {make: true, mode: 0644}
	`,
}, {
	summary: "A single package inspection",
	input:   infoRelease,
	query:   []string{"libpkg"},
	stdout: `
		package: libpkg
		archive: ubuntu
		slices:
			libs:
				contents:
					/usr/lib/libpkg.so*: {}
	`,
}, {
	summary: "Multiple slices within the same package",
	input:   infoRelease,
	query:   []string{"mypkg_foo", "mypkg_baz"},
	stdout: `
		package: mypkg
		archive: ubuntu
		slices:
			baz:
				essential:
					- libpkg_libs
					- mypkg_bar
					- mypkg_foo
			foo:
				contents:
					/etc/foo: {}
					/etc/foo-dir/: {make: true, mode: 0644}
	`,
}, {
	summary: "Packages and slices",
	input:   infoRelease,
	query:   []string{"mypkg_foo", "libpkg", "mypkg_baz"},
	stdout: `
		package: mypkg
		archive: ubuntu
		slices:
			baz:
				essential:
					- libpkg_libs
					- mypkg_bar
					- mypkg_foo
			foo:
				contents:
					/etc/foo: {}
					/etc/foo-dir/: {make: true, mode: 0644}
		---
		package: libpkg
		archive: ubuntu
		slices:
			libs:
				contents:
					/usr/lib/libpkg.so*: {}
	`,
}, {
	summary: "Package and its slices",
	input:   infoRelease,
	query:   []string{"mypkg_foo", "mypkg", "mypkg_baz"},
	stdout: `
		package: mypkg
		archive: ubuntu
		slices:
			bar:
				essential:
					- mypkg_foo
				contents:
					/bin/bar: {}
					/etc/bar.conf: {text: TODO, mutable: true, arch: riscv64}
					/lib/*-linux-*/bar.so: {arch: [amd64, arm64, i386]}
					/usr/bin/bar: {symlink: /bin/bar}
					/usr/bin/baz: {copy: /bin/bar}
					/usr/lib/bar*.so: {}
					/usr/share/bar/*.conf: {until: mutate}
				mutate: |
					dir = "/usr/share/bar/"
					conf = [content.read(dir + path) for path in content.list(dir)]
					content.write("/etc/bar.conf", "".join(conf))
			baz:
				essential:
					- libpkg_libs
					- mypkg_bar
					- mypkg_foo
			foo:
				contents:
					/etc/foo: {}
					/etc/foo-dir/: {make: true, mode: 0644}
	`,
}, {
	summary: "Same slice appearing multiple times",
	input:   infoRelease,
	query:   []string{"mypkg_foo", "mypkg_foo", "mypkg_foo"},
	stdout: `
		package: mypkg
		archive: ubuntu
		slices:
			foo:
				contents:
					/etc/foo: {}
					/etc/foo-dir/: {make: true, mode: 0644}
	`,
}, {
	summary: "No slices found",
	input:   infoRelease,
	query:   []string{"foo", "bar_foo"},
	err:     `no slice definitions found for: "foo", "bar_foo"`,
}, {
	summary: "Some slices found, others not found",
	input:   infoRelease,
	query:   []string{"foo", "mypkg_foo", "bar_foo"},
	stdout: `
		package: mypkg
		archive: ubuntu
		slices:
			foo:
				contents:
					/etc/foo: {}
					/etc/foo-dir/: {make: true, mode: 0644}
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
	format: chisel-v1
	archives:
		ubuntu:
			version: 22.04
			components: [main, universe]
			v1-public-keys: [test-key]
	v1-public-keys:
		test-key:
			id: ` + testKey.ID + `
			armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
`

var infoRelease = map[string]string{
	"chisel.yaml": string(defaultChiselYaml),
	"slices/mypkg.yaml": `
		package: mypkg

		essential:
			- mypkg_foo

		slices:
			foo:
				contents:
					/etc/foo:
					/etc/foo-dir/: {make: true, mode: 0644}
			bar:
				contents:
					/etc/bar.conf: {text: TODO, mutable: true, arch: riscv64}
					/lib/*-linux-*/bar.so: {arch: [amd64,arm64,i386]}
					/usr/bin/bar: {symlink: /bin/bar}
					/usr/bin/baz: {copy: /bin/bar}
					/usr/lib/bar*.so:
					/usr/share/bar/*.conf: {until: mutate}
					/bin/bar:
				mutate: |
					dir = "/usr/share/bar/"
					conf = [content.read(dir + path) for path in content.list(dir)]
					content.write("/etc/bar.conf", "".join(conf))
			baz:
				essential:
					- libpkg_libs
					- mypkg_bar
	`,
	"slices/libpkg.yaml": `
		package: libpkg

		slices:
			libs:
				contents:
					/usr/lib/libpkg.so*:
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
		c.Assert(strings.TrimSpace(s.Stdout()), Equals, strings.TrimSpace(test.stdout))
	}
}
