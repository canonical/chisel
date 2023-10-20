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
	input:   sampleRelease,
	query:   []string{"mypkg_foo"},
	stdout: `
package: mypkg
archive: ubuntu
slices:
    foo:
        contents:
            /etc/foo: {}
            /etc/foo-dir/:
                make: true
                mode: 0644
`,
}, {
	summary: "A single package inspection",
	input:   sampleRelease,
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
	input:   sampleRelease,
	query:   []string{"mypkg_foo", "mypkg_baz"},
	stdout: `
package: mypkg
archive: ubuntu
slices:
    baz:
        essential:
            - libpkg_libs
            - mypkg_foo
            - mypkg_bar
    foo:
        contents:
            /etc/foo: {}
            /etc/foo-dir/:
                make: true
                mode: 0644
`,
}, {
	summary: "Different packages, multiple slices of same packages",
	input:   sampleRelease,
	query:   []string{"mypkg_foo", "libpkg", "mypkg_baz"},
	stdout: `
package: mypkg
archive: ubuntu
slices:
    baz:
        essential:
            - libpkg_libs
            - mypkg_foo
            - mypkg_bar
    foo:
        contents:
            /etc/foo: {}
            /etc/foo-dir/:
                make: true
                mode: 0644
---
package: libpkg
archive: ubuntu
slices:
    libs:
        contents:
            /usr/lib/libpkg.so*: {}
`,
}, {
	summary: "Same package, multiple slices",
	input:   sampleRelease,
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
            /etc/bar.conf:
                text: TODO
                mutable: true
                arch: riscv64
            /lib/*-linux-*/bar.so:
                arch:
                    - amd64
                    - arm64
                    - i386
            /usr/bin/bar:
                symlink: /bin/bar
            /usr/bin/baz:
                copy: /bin/bar
            /usr/lib/bar*.so: {}
            /usr/share/bar/*.conf:
                until: mutate
        mutate: |
            dir = "/usr/share/bar/"
            conf = [content.read(dir + path) for path in content.list(dir)]
            content.write("/etc/bar.conf", "".join(conf))
    baz:
        essential:
            - libpkg_libs
            - mypkg_foo
            - mypkg_bar
    foo:
        contents:
            /etc/foo: {}
            /etc/foo-dir/:
                make: true
                mode: 0644
`,
}, {
	summary: "Same slice, appearing multiple times",
	input:   sampleRelease,
	query:   []string{"mypkg_foo", "mypkg_foo", "mypkg_foo"},
	stdout: `
package: mypkg
archive: ubuntu
slices:
    foo:
        contents:
            /etc/foo: {}
            /etc/foo-dir/:
                make: true
                mode: 0644
`,
}, {
	summary: "No slices found",
	input:   sampleRelease,
	query:   []string{"foo", "bar_foo"},
	err:     ".*no slice definitions found for: .*foo.*bar_foo.*",
}, {
	summary: "Some slices found, others not found",
	input:   sampleRelease,
	query:   []string{"foo", "mypkg_foo", "bar_foo"},
	stdout: `
package: mypkg
archive: ubuntu
slices:
    foo:
        contents:
            /etc/foo: {}
            /etc/foo-dir/:
                make: true
                mode: 0644
`,
	err: ".*no slice definitions found for: .*foo.*bar_foo.*",
}, {
	summary: "No args",
	input:   sampleRelease,
	err:     ".*required argument.*not provided.*",
}, {
	summary: "Empty, whitespace args",
	input:   sampleRelease,
	query:   []string{"", "    "},
	err:     `no slice definitions found for: "", "    "`,
}, {
	summary: "Bad format slices",
	input:   sampleRelease,
	query:   []string{"foo_bar_foo", "a_b", "7_c", "a_b c", "a_b x_y"},
	err:     ".*no slice definitions found for:.*",
}}

const defaultChiselYaml = `
	format: chisel-v1
	archives:
		ubuntu:
			version: 22.04
			components: [main, universe]
`

var sampleRelease = map[string]string{
	"chisel.yaml": string(defaultChiselYaml),
	"slices/mypkg.yaml": `
		package: mypkg

		slices:
			foo:
				contents:
					/etc/foo:
					/etc/foo-dir/: {make: true, mode: 0644}
			bar:
				essential:
					- mypkg_foo
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
					- mypkg_foo
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

		if _, ok := test.input["chisel.yaml"]; !ok {
			test.input["chisel.yaml"] = string(defaultChiselYaml)
		}

		dir := c.MkDir()
		for path, data := range test.input {
			fpath := filepath.Join(dir, path)
			err := os.MkdirAll(filepath.Dir(fpath), 0755)
			c.Assert(err, IsNil)
			err = os.WriteFile(fpath, testutil.Reindent(data), 0644)
			c.Assert(err, IsNil)
		}

		prefix := []string{"info"}
		if len(test.query) > 0 {
			prefix = append(prefix, "--release", dir)
		}
		test.query = append(prefix, test.query...)
		test.stdout = strings.TrimPrefix(test.stdout, "\n")

		_, err := chisel.Parser().ParseArgs(test.query)
		if test.err != "" {
			c.Assert(err, ErrorMatches, test.err)
		} else {
			c.Assert(err, IsNil)
		}
		c.Assert(s.Stdout(), Equals, test.stdout)
		s.ResetStdStreams()
	}
}
