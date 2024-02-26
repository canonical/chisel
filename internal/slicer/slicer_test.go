package slicer_test

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/slicer"
	"github.com/canonical/chisel/internal/testutil"
)

var (
	testKey = testutil.PGPKeys["key1"]
)

type slicerTest struct {
	summary    string
	arch       string
	release    map[string]string
	slices     []setup.SliceKey
	hackopt    func(c *C, opts *slicer.RunOptions)
	filesystem map[string]string
	// TODO:
	// The results of the report do not conform to the planned implementation
	// yet. Namely:
	// * Parent directories of {text} files are reported even though they should not.
	// * We do not track removed directories or changes done in Starlark.
	report map[string]string
	error  string
}

var packageEntries = map[string][]testutil.TarEntry{
	"copyright-symlink-libssl3": {
		{Header: tar.Header{Name: "./"}},
		{Header: tar.Header{Name: "./usr/"}},
		{Header: tar.Header{Name: "./usr/lib/"}},
		{Header: tar.Header{Name: "./usr/lib/x86_64-linux-gnu/"}},
		{Header: tar.Header{Name: "./usr/lib/x86_64-linux-gnu/libssl.so.3", Mode: 00755}},
		{Header: tar.Header{Name: "./usr/share/"}},
		{Header: tar.Header{Name: "./usr/share/doc/"}},
		{Header: tar.Header{Name: "./usr/share/doc/copyright-symlink-libssl3/"}},
		{Header: tar.Header{Name: "./usr/share/doc/copyright-symlink-libssl3/copyright"}},
	},
	"copyright-symlink-openssl": {
		{Header: tar.Header{Name: "./"}},
		{Header: tar.Header{Name: "./etc/"}},
		{Header: tar.Header{Name: "./etc/ssl/"}},
		{Header: tar.Header{Name: "./etc/ssl/openssl.cnf"}},
		{Header: tar.Header{Name: "./usr/"}},
		{Header: tar.Header{Name: "./usr/bin/"}},
		{Header: tar.Header{Name: "./usr/bin/openssl", Mode: 00755}},
		{Header: tar.Header{Name: "./usr/share/"}},
		{Header: tar.Header{Name: "./usr/share/doc/"}},
		{Header: tar.Header{Name: "./usr/share/doc/copyright-symlink-openssl/"}},
		{Header: tar.Header{Name: "./usr/share/doc/copyright-symlink-openssl/copyright", Linkname: "../libssl3/copyright"}},
	},
}

// filesystem entries of copyright file from base-files package that will be
// automatically injected into every slice
var copyrightEntries = map[string]string{
	"/usr/":                               "dir 0755",
	"/usr/share/":                         "dir 0755",
	"/usr/share/doc/":                     "dir 0755",
	"/usr/share/doc/base-files/":          "dir 0755",
	"/usr/share/doc/base-files/copyright": "file 0644 cdb5461d",
}

var slicerTests = []slicerTest{{
	summary: "Basic slicing",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/usr/bin/hello:
						/usr/bin/hallo: {copy: /usr/bin/hello}
						/bin/hallo:     {symlink: ../usr/bin/hallo}
						/etc/passwd:    {text: data1}
						/etc/dir/sub/:  {make: true, mode: 01777}
		`,
	},
	filesystem: map[string]string{
		"/usr/":          "dir 0755",
		"/usr/bin/":      "dir 0755",
		"/usr/bin/hello": "file 0775 eaf29575",
		"/usr/bin/hallo": "file 0775 eaf29575",
		"/bin/":          "dir 0755",
		"/bin/hallo":     "symlink ../usr/bin/hallo",
		"/etc/":          "dir 0755",
		"/etc/dir/":      "dir 0755",
		"/etc/dir/sub/":  "dir 01777",
		"/etc/passwd":    "file 0644 5b41362b",
	},
	report: map[string]string{
		"/bin/":          "dir 0755 {base-files_myslice}",
		"/bin/hallo":     "symlink ../usr/bin/hallo {base-files_myslice}",
		"/etc/":          "dir 0755 {base-files_myslice}",
		"/etc/dir/sub/":  "dir 01777 {base-files_myslice}",
		"/etc/passwd":    "file 0644 5b41362b {base-files_myslice}",
		"/usr/":          "dir 0755 {base-files_myslice}",
		"/usr/bin/":      "dir 0755 {base-files_myslice}",
		"/usr/bin/hallo": "file 0775 eaf29575 {base-files_myslice}",
		"/usr/bin/hello": "file 0775 eaf29575 {base-files_myslice}",
	},
}, {
	summary: "Glob extraction",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/**/he*o:
		`,
	},
	filesystem: map[string]string{
		"/usr/":          "dir 0755",
		"/usr/bin/":      "dir 0755",
		"/usr/bin/hello": "file 0775 eaf29575",
	},
	report: map[string]string{
		"/usr/":          "dir 0755 {base-files_myslice}",
		"/usr/bin/hello": "file 0775 eaf29575 {base-files_myslice}",
	},
}, {
	summary: "Create new file under extracted directory",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						# Note the missing /tmp/ here.
						/tmp/new: {text: data1}
		`,
	},
	filesystem: map[string]string{
		"/tmp/":    "dir 01777", // This is the magic.
		"/tmp/new": "file 0644 5b41362b",
	},
	report: map[string]string{
		"/tmp/":    "dir 01777 {base-files_myslice}",
		"/tmp/new": "file 0644 5b41362b {base-files_myslice}",
	},
}, {
	summary: "Create new nested file under extracted directory",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						# Note the missing /tmp/ here.
						/tmp/new/sub: {text: data1}
		`,
	},
	filesystem: map[string]string{
		"/tmp/":        "dir 01777", // This is the magic.
		"/tmp/new/":    "dir 0755",
		"/tmp/new/sub": "file 0644 5b41362b",
	},
	report: map[string]string{
		"/tmp/":        "dir 01777 {base-files_myslice}",
		"/tmp/new/sub": "file 0644 5b41362b {base-files_myslice}",
	},
}, {
	summary: "Create new directory under extracted directory",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						# Note the missing /tmp/ here.
						/tmp/new/: {make: true}
		`,
	},
	filesystem: map[string]string{
		"/tmp/":     "dir 01777", // This is the magic.
		"/tmp/new/": "dir 0755",
	},
	report: map[string]string{
		"/tmp/":     "dir 01777 {base-files_myslice}",
		"/tmp/new/": "dir 0755 {base-files_myslice}",
	},
}, {
	summary: "Conditional architecture",
	arch:    "amd64",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/tmp/file1: {text: data1, arch: amd64}
						/tmp/file2: {text: data1, arch: i386}
						/tmp/file3: {text: data1, arch: [i386, amd64]}
						/usr/bin/hello1: {copy: /usr/bin/hello, arch: amd64}
						/usr/bin/hello2: {copy: /usr/bin/hello, arch: i386}
						/usr/bin/hello3: {copy: /usr/bin/hello, arch: [i386, amd64]}
		`,
	},
	filesystem: map[string]string{
		"/tmp/":           "dir 01777",
		"/tmp/file1":      "file 0644 5b41362b",
		"/tmp/file3":      "file 0644 5b41362b",
		"/usr/":           "dir 0755",
		"/usr/bin/":       "dir 0755",
		"/usr/bin/hello1": "file 0775 eaf29575",
		"/usr/bin/hello3": "file 0775 eaf29575",
	},
	report: map[string]string{
		"/tmp/":           "dir 01777 {base-files_myslice}",
		"/tmp/file1":      "file 0644 5b41362b {base-files_myslice}",
		"/tmp/file3":      "file 0644 5b41362b {base-files_myslice}",
		"/usr/":           "dir 0755 {base-files_myslice}",
		"/usr/bin/":       "dir 0755 {base-files_myslice}",
		"/usr/bin/hello1": "file 0775 eaf29575 {base-files_myslice}",
		"/usr/bin/hello3": "file 0775 eaf29575 {base-files_myslice}",
	},
}, {
	summary: "Script: write a file",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/tmp/file1: {text: data1, mutable: true}
					mutate: |
						content.write("/tmp/file1", "data2")
		`,
	},
	filesystem: map[string]string{
		"/tmp/":      "dir 01777",
		"/tmp/file1": "file 0644 d98cf53e",
	},
	report: map[string]string{
		"/tmp/":      "dir 01777 {base-files_myslice}",
		"/tmp/file1": "file 0644 5b41362b {base-files_myslice}",
	},
}, {
	summary: "Script: read a file",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/tmp/file1: {text: data1}
						/foo/file2: {text: data2, mutable: true}
					mutate: |
						data = content.read("/tmp/file1")
						content.write("/foo/file2", data)
		`,
	},
	filesystem: map[string]string{
		"/tmp/":      "dir 01777",
		"/tmp/file1": "file 0644 5b41362b",
		"/foo/":      "dir 0755",
		"/foo/file2": "file 0644 5b41362b",
	},
	report: map[string]string{
		"/foo/file2": "file 0644 d98cf53e {base-files_myslice}",
		"/tmp/":      "dir 01777 {base-files_myslice}",
		"/tmp/file1": "file 0644 5b41362b {base-files_myslice}",
	},
}, {
	summary: "Script: use 'until' to remove file after mutate",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/tmp/file1: {text: data1, until: mutate}
						/foo/file2: {text: data2, mutable: true}
					mutate: |
						data = content.read("/tmp/file1")
						content.write("/foo/file2", data)
		`,
	},
	filesystem: map[string]string{
		"/tmp/":      "dir 01777",
		"/foo/":      "dir 0755",
		"/foo/file2": "file 0644 5b41362b",
	},
	report: map[string]string{
		"/foo/file2": "file 0644 d98cf53e {base-files_myslice}",
		"/tmp/":      "dir 01777 {base-files_myslice}",
		"/tmp/file1": "file 0644 5b41362b {base-files_myslice}",
	},
}, {
	summary: "Script: use 'until' to remove wildcard after mutate",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/usr/bin**:  {until: mutate}
						/etc/passwd: {until: mutate, text: data1}
		`,
	},
	filesystem: map[string]string{
		"/usr/": "dir 0755",
		"/etc/": "dir 0755",
	},
	report: map[string]string{
		"/etc/":          "dir 0755 {base-files_myslice}",
		"/etc/passwd":    "file 0644 5b41362b {base-files_myslice}",
		"/usr/":          "dir 0755 {base-files_myslice}",
		"/usr/bin/":      "dir 0755 {base-files_myslice}",
		"/usr/bin/hello": "file 0775 eaf29575 {base-files_myslice}",
	},
}, {
	summary: "Script: 'until' does not remove non-empty directories",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/usr/bin/: {until: mutate}
						/usr/bin/hallo: {copy: /usr/bin/hello}
		`,
	},
	filesystem: map[string]string{
		"/usr/":          "dir 0755",
		"/usr/bin/":      "dir 0755",
		"/usr/bin/hallo": "file 0775 eaf29575",
	},
	report: map[string]string{
		"/usr/":          "dir 0755 {base-files_myslice}",
		"/usr/bin/":      "dir 0755 {base-files_myslice}",
		"/usr/bin/hallo": "file 0775 eaf29575 {base-files_myslice}",
	},
}, {
	summary: "Script: cannot write non-mutable files",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/tmp/file1: {text: data1}
					mutate: |
						content.write("/tmp/file1", "data2")
		`,
	},
	error: `slice base-files_myslice: cannot write file which is not mutable: /tmp/file1`,
}, {
	summary: "Script: cannot read unlisted content",
	slices:  []setup.SliceKey{{"base-files", "myslice2"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice1:
					contents:
						/tmp/file1: {text: data1}
				myslice2:
					mutate: |
						content.read("/tmp/file1")
		`,
	},
	error: `slice base-files_myslice2: cannot read file which is not selected: /tmp/file1`,
}, {
	summary: "Script: can read globbed content",
	slices:  []setup.SliceKey{{"base-files", "myslice1"}, {"base-files", "myslice2"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice1:
					contents:
						/usr/bin/*:
				myslice2:
					mutate: |
						content.read("/usr/bin/hello")
		`,
	},
}, {
	summary: "Relative content root directory must not error",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/tmp/file1: {text: data1}
					mutate: |
						content.read("/tmp/file1")
		`,
	},
	hackopt: func(c *C, opts *slicer.RunOptions) {
		dir, err := os.Getwd()
		c.Assert(err, IsNil)
		opts.TargetDir, err = filepath.Rel(dir, opts.TargetDir)
		c.Assert(err, IsNil)
	},
}, {
	summary: "Can list parent directories of normal paths",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/a/b/c: {text: foo}
						/x/y/: {make: true}
					mutate: |
						content.list("/")
						content.list("/a")
						content.list("/a/b")
						content.list("/x")
						content.list("/x/y")
		`,
	},
}, {
	summary: "Cannot list unselected directory",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/a/b/c: {text: foo}
					mutate: |
						content.list("/a/d")
		`,
	},
	error: `slice base-files_myslice: cannot list directory which is not selected: /a/d/`,
}, {
	summary: "Cannot list file path as a directory",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/a/b/c: {text: foo}
					mutate: |
						content.list("/a/b/c")
		`,
	},
	error: `slice base-files_myslice: content is not a directory: /a/b/c`,
}, {
	summary: "Can list parent directories of globs",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/**/bin/h?llo:
					mutate: |
						content.list("/usr/bin")
		`,
	},
}, {
	summary: "Cannot list directories not matched by glob",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/**/bin/h?llo:
					mutate: |
						content.list("/etc")
		`,
	},
	error: `slice base-files_myslice: cannot list directory which is not selected: /etc/`,
}, {
	summary: "Duplicate copyright symlink is ignored",
	slices:  []setup.SliceKey{{"copyright-symlink-openssl", "bins"}},
	release: map[string]string{
		"slices/mydir/copyright-symlink-libssl3.yaml": `
			package: copyright-symlink-libssl3
			slices:
				libs:
					contents:
						/usr/lib/x86_64-linux-gnu/libssl.so.3:
		`,
		"slices/mydir/copyright-symlink-openssl.yaml": `
			package: copyright-symlink-openssl
			slices:
				bins:
					essential:
						- copyright-symlink-libssl3_libs
						- copyright-symlink-openssl_config
					contents:
						/usr/bin/openssl:
				config:
					contents:
						/etc/ssl/openssl.cnf:
		`,
	},
}, {
	summary: "Can list unclean directory paths",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/a/b/c: {text: foo}
						/x/y/: {make: true}
					mutate: |
						content.list("/////")
						content.list("/a/")
						content.list("/a/b/../b/")
						content.list("/x///")
						content.list("/x/./././y")
		`,
	},
}, {
	summary: "Cannot read directories",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/x/y/: {make: true}
					mutate: |
						content.read("/x/y")
		`,
	},
	error: `slice base-files_myslice: content is not a file: /x/y`,
}, {
	summary: "Non-default archive",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"chisel.yaml": `
			format: chisel-v1
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					default: true
					v1-public-keys: [test-key]
				bar:
					version: 22.04
					components: [main]
					v1-public-keys: [test-key]
			v1-public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/base-files.yaml": `
			package: base-files
			archive: bar
			slices:
				myslice:
					contents:
						/usr/bin/hello:
		`,
	},
	filesystem: map[string]string{
		"/usr/":          "dir 0755",
		"/usr/bin/":      "dir 0755",
		"/usr/bin/hello": "file 0775 eaf29575",
	},
	report: map[string]string{
		"/usr/":          "dir 0755 {base-files_myslice}",
		"/usr/bin/":      "dir 0755 {base-files_myslice}",
		"/usr/bin/hello": "file 0775 eaf29575 {base-files_myslice}",
	},
}}

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

type testArchive struct {
	options archive.Options
	pkgs    map[string][]byte
}

func (a *testArchive) Options() *archive.Options {
	return &a.options
}

func (a *testArchive) Fetch(pkg string) (io.ReadCloser, error) {
	if data, ok := a.pkgs[pkg]; ok {
		return io.NopCloser(bytes.NewBuffer(data)), nil
	}
	return nil, fmt.Errorf("attempted to open %q package", pkg)
}

func (a *testArchive) Exists(pkg string) bool {
	_, ok := a.pkgs[pkg]
	return ok
}

func (s *S) TestRun(c *C) {
	// Run tests for format chisel-v1.
	runSlicerTests(c, slicerTests)

	// Run tests for format v1.
	v1SlicerTests := make([]slicerTest, len(slicerTests))
	for i, t := range slicerTests {
		t.error = strings.Replace(t.error, "chisel-v1", "v1", -1)
		t.error = strings.Replace(t.error, "v1-public-keys", "public-keys", -1)
		m := map[string]string{}
		for k, v := range t.release {
			v = strings.Replace(v, "chisel-v1", "v1", -1)
			v = strings.Replace(v, "v1-public-keys", "public-keys", -1)
			m[k] = v
		}
		t.release = m
		v1SlicerTests[i] = t
	}
	runSlicerTests(c, v1SlicerTests)
}

func runSlicerTests(c *C, tests []slicerTest) {
	for _, test := range tests {
		c.Logf("Summary: %s", test.summary)

		if _, ok := test.release["chisel.yaml"]; !ok {
			test.release["chisel.yaml"] = string(defaultChiselYaml)
		}

		releaseDir := c.MkDir()
		for path, data := range test.release {
			fpath := filepath.Join(releaseDir, path)
			err := os.MkdirAll(filepath.Dir(fpath), 0755)
			c.Assert(err, IsNil)
			err = os.WriteFile(fpath, testutil.Reindent(data), 0644)
			c.Assert(err, IsNil)
		}

		release, err := setup.ReadRelease(releaseDir)
		c.Assert(err, IsNil)

		selection, err := setup.Select(release, test.slices)
		c.Assert(err, IsNil)

		pkgs := map[string][]byte{
			"base-files": testutil.PackageData["base-files"],
		}
		for name, entries := range packageEntries {
			deb, err := testutil.MakeDeb(entries)
			c.Assert(err, IsNil)
			pkgs[name] = deb
		}
		archives := map[string]archive.Archive{}
		for name, setupArchive := range release.Archives {
			archive := &testArchive{
				options: archive.Options{
					Label:      setupArchive.Name,
					Version:    setupArchive.Version,
					Suites:     setupArchive.Suites,
					Components: setupArchive.Components,
					Arch:       test.arch,
				},
				pkgs: pkgs,
			}
			archives[name] = archive
		}

		targetDir := c.MkDir()
		options := slicer.RunOptions{
			Selection: selection,
			Archives:  archives,
			TargetDir: targetDir,
		}
		if test.hackopt != nil {
			test.hackopt(c, &options)
		}
		report, err := slicer.Run(&options)
		if test.error == "" {
			c.Assert(err, IsNil)
		} else {
			c.Assert(err, ErrorMatches, test.error)
			continue
		}

		if test.filesystem != nil {
			result := make(map[string]string, len(copyrightEntries)+len(test.filesystem))
			for k, v := range copyrightEntries {
				result[k] = v
			}
			for k, v := range test.filesystem {
				result[k] = v
			}
			c.Assert(testutil.TreeDump(targetDir), DeepEquals, result)
		}

		if test.report != nil {
			result := make(map[string]string, len(copyrightEntries)+len(test.filesystem))
			for k, v := range copyrightEntries {
				result[k] = v + " {base-files_myslice}"
			}
			for k, v := range test.report {
				result[k] = v
			}
			c.Assert(treeDumpReport(report), DeepEquals, result)
		}
	}
}

// treeDumpReport returns the file information in the same format as
// [testutil.TreeDump] with the added slices that have installed each path.
func treeDumpReport(report *slicer.Report) map[string]string {
	result := make(map[string]string)
	for _, entry := range report.Entries {
		fperm := entry.Mode.Perm()
		if entry.Mode&fs.ModeSticky != 0 {
			fperm |= 01000
		}
		var fsDump string
		switch entry.Mode.Type() {
		case fs.ModeDir:
			entry.Path = entry.Path + "/"
			fsDump = fmt.Sprintf("dir %#o", fperm)
		case fs.ModeSymlink:
			fsDump = fmt.Sprintf("symlink %s", entry.Link)
		case 0: // Regular
			if entry.Size == 0 {
				fsDump = fmt.Sprintf("file %#o empty", entry.Mode.Perm())
			} else {
				fsDump = fmt.Sprintf("file %#o %s", fperm, entry.Hash[:8])
			}
		default:
			panic(fmt.Errorf("unknown file type %d: %s", entry.Mode.Type(), entry.Path))
		}

		// append {slice1, ..., sliceN} to the end of the entry dump.
		slicesStr := make([]string, 0, len(entry.Slices))
		for slice := range entry.Slices {
			slicesStr = append(slicesStr, slice.String())
		}
		result[entry.Path] = fmt.Sprintf("%s {%s}", fsDump, strings.Join(slicesStr, ","))
	}
	return result
}
