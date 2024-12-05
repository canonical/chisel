package slicer_test

import (
	"archive/tar"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/klauspost/compress/zstd"
	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/manifest"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/slicer"
	"github.com/canonical/chisel/internal/testutil"
)

var (
	testKey = testutil.PGPKeys["key1"]
)

type slicerTest struct {
	summary       string
	arch          string
	release       map[string]string
	pkgs          []*testutil.TestPackage
	slices        []setup.SliceKey
	hackopt       func(c *C, opts *slicer.RunOptions)
	filesystem    map[string]string
	manifestPaths map[string]string
	manifestPkgs  map[string]string
	error         string
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

var testPackageCopyrightEntries = []testutil.TarEntry{
	// Hardcoded copyright paths.
	testutil.Dir(0755, "./usr/"),
	testutil.Dir(0755, "./usr/share/"),
	testutil.Dir(0755, "./usr/share/doc/"),
	testutil.Dir(0755, "./usr/share/doc/test-package/"),
	testutil.Reg(0644, "./usr/share/doc/test-package/copyright", "copyright"),
}

var slicerTests = []slicerTest{{
	summary: "Basic slicing",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/file:
						/dir/file-copy:  {copy: /dir/file}
						/other-dir/file: {symlink: ../dir/file}
						/dir/text-file:  {text: data1}
						/dir/foo/bar/:   {make: true, mode: 01777}
		`,
	},
	filesystem: map[string]string{
		"/dir/":           "dir 0755",
		"/dir/file":       "file 0644 cc55e2ec",
		"/dir/file-copy":  "file 0644 cc55e2ec",
		"/dir/foo/":       "dir 0755",
		"/dir/foo/bar/":   "dir 01777",
		"/dir/text-file":  "file 0644 5b41362b",
		"/other-dir/":     "dir 0755",
		"/other-dir/file": "symlink ../dir/file",
	},
	manifestPaths: map[string]string{
		"/dir/file":       "file 0644 cc55e2ec {test-package_myslice}",
		"/dir/file-copy":  "file 0644 cc55e2ec {test-package_myslice}",
		"/dir/foo/bar/":   "dir 01777 {test-package_myslice}",
		"/dir/text-file":  "file 0644 5b41362b {test-package_myslice}",
		"/other-dir/file": "symlink ../dir/file {test-package_myslice}",
	},
}, {
	summary: "Glob extraction",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/**/other-f*e:
		`,
	},
	filesystem: map[string]string{
		"/dir/":                  "dir 0755",
		"/dir/nested/":           "dir 0755",
		"/dir/nested/other-file": "file 0644 6b86b273",
		"/dir/other-file":        "file 0644 63d5dd49",
	},
	manifestPaths: map[string]string{
		"/dir/nested/other-file": "file 0644 6b86b273 {test-package_myslice}",
		"/dir/other-file":        "file 0644 63d5dd49 {test-package_myslice}",
	},
}, {
	summary: "Create new file under extracted directory and preserve parent directory permissions",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						# Note the missing /parent/ here.
						/parent/new: {text: data1}
		`,
	},
	filesystem: map[string]string{
		"/parent/":    "dir 01777", // This is the magic.
		"/parent/new": "file 0644 5b41362b",
	},
	manifestPaths: map[string]string{
		"/parent/new": "file 0644 5b41362b {test-package_myslice}",
	},
}, {
	summary: "Create new nested file under extracted directory and preserve parent directory permissions",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						# Note the missing /parent/ and /parent/permissions/ here.
						/parent/permissions/new: {text: data1}
		`,
	},
	filesystem: map[string]string{
		"/parent/":                "dir 01777", // This is the magic.
		"/parent/permissions/":    "dir 0764",  // This is the magic.
		"/parent/permissions/new": "file 0644 5b41362b",
	},
	manifestPaths: map[string]string{
		"/parent/permissions/new": "file 0644 5b41362b {test-package_myslice}",
	},
}, {
	summary: "Create new directory under extracted directory and preserve parent directory permissions",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						# Note the missing /parent/ here.
						/parent/new/: {make: true}
		`,
	},
	filesystem: map[string]string{
		"/parent/":     "dir 01777", // This is the magic.
		"/parent/new/": "dir 0755",
	},
	manifestPaths: map[string]string{
		"/parent/new/": "dir 0755 {test-package_myslice}",
	},
}, {
	summary: "Create new file using glob and preserve parent directory permissions",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						# Note the missing /parent/ and /parent/permissions/ here.
						/parent/**:
		`,
	},
	filesystem: map[string]string{
		"/parent/":                 "dir 01777", // This is the magic.
		"/parent/permissions/":     "dir 0764",  // This is the magic.
		"/parent/permissions/file": "file 0755 722c14b3",
	},
	manifestPaths: map[string]string{
		"/parent/":                 "dir 01777 {test-package_myslice}",
		"/parent/permissions/":     "dir 0764 {test-package_myslice}",
		"/parent/permissions/file": "file 0755 722c14b3 {test-package_myslice}",
	},
}, {
	summary: "Conditional architecture",
	arch:    "amd64",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/text-file-1: {text: data1, arch: amd64}
						/dir/text-file-2: {text: data1, arch: i386}
						/dir/text-file-3: {text: data1, arch: [i386, amd64]}
						/dir/nested/copy-1: {copy: /dir/nested/file, arch: amd64}
						/dir/nested/copy-2: {copy: /dir/nested/file, arch: i386}
						/dir/nested/copy-3: {copy: /dir/nested/file, arch: [i386, amd64]}
		`,
	},
	filesystem: map[string]string{
		"/dir/":              "dir 0755",
		"/dir/text-file-1":   "file 0644 5b41362b",
		"/dir/text-file-3":   "file 0644 5b41362b",
		"/dir/nested/":       "dir 0755",
		"/dir/nested/copy-1": "file 0644 84237a05",
		"/dir/nested/copy-3": "file 0644 84237a05",
	},
	manifestPaths: map[string]string{
		"/dir/nested/copy-1": "file 0644 84237a05 {test-package_myslice}",
		"/dir/nested/copy-3": "file 0644 84237a05 {test-package_myslice}",
		"/dir/text-file-1":   "file 0644 5b41362b {test-package_myslice}",
		"/dir/text-file-3":   "file 0644 5b41362b {test-package_myslice}",
	},
}, {
	summary: "Copyright is not installed implicitly",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name: "test-package",
		// Add the copyright entries to the package.
		Data: testutil.MustMakeDeb(append(testutil.TestPackageEntries, testPackageCopyrightEntries...)),
	}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/file:
		`,
	},
	filesystem: map[string]string{
		"/dir/":     "dir 0755",
		"/dir/file": "file 0644 cc55e2ec",
	},
	manifestPaths: map[string]string{
		"/dir/file": "file 0644 cc55e2ec {test-package_myslice}",
	},
}, {
	summary: "Install two packages",
	slices: []setup.SliceKey{
		{"test-package", "myslice"},
		{"other-package", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name: "test-package",
		Data: testutil.PackageData["test-package"],
	}, {
		Name: "other-package",
		Data: testutil.PackageData["other-package"],
	}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/file:
						/foo/: {make: true}
		`,
		"slices/mydir/other-package.yaml": `
			package: other-package
			slices:
				myslice:
					contents:
						/file:
						/bar/: {make: true}
		`,
	},
	filesystem: map[string]string{
		"/bar/":     "dir 0755",
		"/dir/":     "dir 0755",
		"/dir/file": "file 0644 cc55e2ec",
		"/file":     "file 0644 fc02ca0e",
		"/foo/":     "dir 0755",
	},
	manifestPaths: map[string]string{
		"/foo/":     "dir 0755 {test-package_myslice}",
		"/dir/file": "file 0644 cc55e2ec {test-package_myslice}",
		"/bar/":     "dir 0755 {other-package_myslice}",
		"/file":     "file 0644 fc02ca0e {other-package_myslice}",
	},
}, {
	summary: "Install two packages, explicit path has preference over implicit parent",
	slices: []setup.SliceKey{
		{"a-implicit-parent", "myslice"},
		{"b-explicit-dir", "myslice"},
		{"c-implicit-parent", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name: "a-implicit-parent",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0755, "./dir/"),
			testutil.Reg(0644, "./dir/file-1", "random"),
		}),
	}, {
		Name: "b-explicit-dir",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(01777, "./dir/"),
		}),
	}, {
		Name: "c-implicit-parent",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0755, "./dir/"),
			testutil.Reg(0644, "./dir/file-2", "random"),
		}),
	}},
	release: map[string]string{
		"slices/mydir/a-implicit-parent.yaml": `
			package: a-implicit-parent
			slices:
				myslice:
					contents:
						/dir/file-1:
		`,
		"slices/mydir/b-explicit-dir.yaml": `
			package: b-explicit-dir
			slices:
				myslice:
					contents:
						/dir/:
		`,
		"slices/mydir/c-implicit-parent.yaml": `
			package: c-implicit-parent
			slices:
				myslice:
					contents:
						/dir/file-2:
		`,
	},
	filesystem: map[string]string{
		"/dir/":       "dir 01777",
		"/dir/file-1": "file 0644 a441b15f",
		"/dir/file-2": "file 0644 a441b15f",
	},
	manifestPaths: map[string]string{
		"/dir/":       "dir 01777 {b-explicit-dir_myslice}",
		"/dir/file-1": "file 0644 a441b15f {a-implicit-parent_myslice}",
		"/dir/file-2": "file 0644 a441b15f {c-implicit-parent_myslice}",
	},
}, {
	summary: "Valid same file in two slices in different packages",
	slices: []setup.SliceKey{
		{"test-package", "myslice"},
		{"other-package", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name: "test-package",
		Data: testutil.PackageData["test-package"],
	}, {
		Name: "other-package",
		Data: testutil.PackageData["other-package"],
	}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/textFile: {text: SAME_TEXT}
		`,
		"slices/mydir/other-package.yaml": `
			package: other-package
			slices:
				myslice:
					contents:
						/textFile: {text: SAME_TEXT}
		`,
	},
	filesystem: map[string]string{
		"/textFile": "file 0644 c6c83d10",
	},
	manifestPaths: map[string]string{
		"/textFile": "file 0644 c6c83d10 {other-package_myslice,test-package_myslice}",
	},
}, {
	summary: "Script: write a file",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/text-file: {text: data1, mutable: true}
					mutate: |
						content.write("/dir/text-file", "data2")
		`,
	},
	filesystem: map[string]string{
		"/dir/":          "dir 0755",
		"/dir/text-file": "file 0644 d98cf53e",
	},
	manifestPaths: map[string]string{
		"/dir/text-file": "file 0644 5b41362b d98cf53e {test-package_myslice}",
	},
}, {
	summary: "Script: read a file",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/text-file-1: {text: data1}
						/foo/text-file-2: {text: data2, mutable: true}
					mutate: |
						data = content.read("/dir/text-file-1")
						content.write("/foo/text-file-2", data)
		`,
	},
	filesystem: map[string]string{
		"/dir/":            "dir 0755",
		"/dir/text-file-1": "file 0644 5b41362b",
		"/foo/":            "dir 0755",
		"/foo/text-file-2": "file 0644 5b41362b",
	},
	manifestPaths: map[string]string{
		"/dir/text-file-1": "file 0644 5b41362b {test-package_myslice}",
		"/foo/text-file-2": "file 0644 d98cf53e 5b41362b {test-package_myslice}",
	},
}, {
	summary: "Script: use 'until' to remove file after mutate",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/text-file-1: {text: data1, until: mutate}
						/foo/text-file-2: {text: data2, mutable: true}
					mutate: |
						data = content.read("/dir/text-file-1")
						content.write("/foo/text-file-2", data)
		`,
	},
	filesystem: map[string]string{
		"/dir/":            "dir 0755",
		"/foo/":            "dir 0755",
		"/foo/text-file-2": "file 0644 5b41362b",
	},
	manifestPaths: map[string]string{
		"/foo/text-file-2": "file 0644 d98cf53e 5b41362b {test-package_myslice}",
	},
}, {
	summary: "Script: use 'until' to remove wildcard after mutate",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/nested**:  {until: mutate}
						/other-dir/text-file: {until: mutate, text: data1}
		`,
	},
	filesystem: map[string]string{
		"/dir/":       "dir 0755",
		"/other-dir/": "dir 0755",
	},
	manifestPaths: map[string]string{},
}, {
	summary: "Script: 'until' does not remove non-empty directories",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/nested/: {until: mutate}
						/dir/nested/file-copy: {copy: /dir/file}
		`,
	},
	filesystem: map[string]string{
		"/dir/":                 "dir 0755",
		"/dir/nested/":          "dir 0755",
		"/dir/nested/file-copy": "file 0644 cc55e2ec",
	},
	manifestPaths: map[string]string{
		"/dir/nested/file-copy": "file 0644 cc55e2ec {test-package_myslice}",
	},
}, {
	summary: "Script: writing same contents to existing file does not set the final hash in report",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/text-file: {text: data1, mutable: true}
					mutate: |
						content.write("/dir/text-file", "data1")
		`,
	},
	filesystem: map[string]string{
		"/dir/":          "dir 0755",
		"/dir/text-file": "file 0644 5b41362b",
	},
	manifestPaths: map[string]string{
		"/dir/text-file": "file 0644 5b41362b {test-package_myslice}",
	},
}, {
	summary: "Script: cannot write non-mutable files",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/text-file: {text: data1}
					mutate: |
						content.write("/dir/text-file", "data2")
		`,
	},
	error: `slice test-package_myslice: cannot write file which is not mutable: /dir/text-file`,
}, {
	summary: "Script: cannot write to unlisted file",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
					mutate: |
						content.write("/dir/text-file", "data")
		`,
	},
	error: `slice test-package_myslice: cannot write file which is not mutable: /dir/text-file`,
}, {
	summary: "Script: cannot write to directory",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/: {make: true}
					mutate: |
						content.write("/dir/", "data")
		`,
	},
	error: `slice test-package_myslice: cannot write file which is not mutable: /dir/`,
}, {
	summary: "Script: cannot read unlisted content",
	slices:  []setup.SliceKey{{"test-package", "myslice2"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice1:
					contents:
						/dir/text-file: {text: data1}
				myslice2:
					mutate: |
						content.read("/dir/text-file")
		`,
	},
	error: `slice test-package_myslice2: cannot read file which is not selected: /dir/text-file`,
}, {
	summary: "Script: can read globbed content",
	slices:  []setup.SliceKey{{"test-package", "myslice1"}, {"test-package", "myslice2"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice1:
					contents:
						/dir/nested/fil*:
				myslice2:
					mutate: |
						content.read("/dir/nested/file")
		`,
	},
}, {
	summary: "Relative content root directory must not error",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/text-file: {text: data1, mutable: true}
					mutate: |
						content.read("/dir/text-file")
						content.write("/dir/text-file", "data2")
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
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
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
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/a/b/c: {text: foo}
					mutate: |
						content.list("/a/d")
		`,
	},
	error: `slice test-package_myslice: cannot list directory which is not selected: /a/d/`,
}, {
	summary: "Cannot list file path as a directory",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/a/b/c: {text: foo}
					mutate: |
						content.list("/a/b/c")
		`,
	},
	error: `slice test-package_myslice: content is not a directory: /a/b/c`,
}, {
	summary: "Can list parent directories of globs",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/**/nested/f?le:
					mutate: |
						content.list("/dir/nested")
		`,
	},
}, {
	summary: "Cannot list directories not matched by glob",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/**/nested/f?le:
					mutate: |
						content.list("/other-dir")
		`,
	},
	error: `slice test-package_myslice: cannot list directory which is not selected: /other-dir/`,
}, {
	summary: "Duplicate copyright symlink is ignored",
	slices:  []setup.SliceKey{{"copyright-symlink-openssl", "bins"}},
	pkgs: []*testutil.TestPackage{{
		Name: "copyright-symlink-openssl",
		Data: testutil.MustMakeDeb(packageEntries["copyright-symlink-openssl"]),
	}, {
		Name: "copyright-symlink-libssl3",
		Data: testutil.MustMakeDeb(packageEntries["copyright-symlink-libssl3"]),
	}},
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
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
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
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/x/y/: {make: true}
					mutate: |
						content.read("/x/y")
		`,
	},
	error: `slice test-package_myslice: content is not a file: /x/y`,
}, {
	summary: "Multiple archives with priority",
	slices:  []setup.SliceKey{{"test-package", "myslice"}, {"other-package", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name:    "test-package",
		Hash:    "h1",
		Version: "v1",
		Arch:    "a1",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Reg(0644, "./file", "from foo"),
		}),
		Archives: []string{"foo"},
	}, {
		Name:    "test-package",
		Hash:    "h2",
		Version: "v2",
		Arch:    "a2",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Reg(0644, "./file", "from bar"),
		}),
		Archives: []string{"bar"},
	}, {
		Name:    "other-package",
		Hash:    "h3",
		Version: "v3",
		Arch:    "a3",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Reg(0644, "./other-file", "from bar"),
		}),
		Archives: []string{"bar"},
	}},
	release: map[string]string{
		"chisel.yaml": `
			format: v1
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					priority: 20
					public-keys: [test-key]
				bar:
					version: 22.04
					components: [main]
					suites: [jammy]
					priority: 10
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/file:
		`,
		"slices/mydir/other-package.yaml": `
			package: other-package
			slices:
				myslice:
					contents:
						/other-file:
		`,
	},
	filesystem: map[string]string{
		// The notion of "default" is obsolete and highest priority is selected.
		"/file": "file 0644 7a3e00f5",
		// Fetched from archive "bar" as no other archive has the package.
		"/other-file": "file 0644 fa0c9cdb",
	},
	manifestPaths: map[string]string{
		"/file":       "file 0644 7a3e00f5 {test-package_myslice}",
		"/other-file": "file 0644 fa0c9cdb {other-package_myslice}",
	},
	manifestPkgs: map[string]string{
		"test-package":  "test-package v1 a1 h1",
		"other-package": "other-package v3 a3 h3",
	},
}, {
	summary: "Pinned archive bypasses higher priority",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name:    "test-package",
		Hash:    "h1",
		Version: "v1",
		Arch:    "a1",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Reg(0644, "./file", "from foo"),
		}),
		Archives: []string{"foo"},
	}, {
		Name:    "test-package",
		Hash:    "h2",
		Version: "v2",
		Arch:    "a2",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Reg(0644, "./file", "from bar"),
		}),
		Archives: []string{"bar"},
	}},
	release: map[string]string{
		"chisel.yaml": `
			format: v1
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					priority: 20
					public-keys: [test-key]
				bar:
					version: 22.04
					components: [main]
					suites: [jammy]
					priority: 10
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/test-package.yaml": `
			package: test-package
			archive: bar
			slices:
				myslice:
					contents:
						/file:
		`,
	},
	hackopt: func(c *C, opts *slicer.RunOptions) {
		delete(opts.Archives, "foo")
	},
	filesystem: map[string]string{
		// test-package fetched from pinned archive "bar".
		"/file": "file 0644 fa0c9cdb",
	},
	manifestPaths: map[string]string{
		"/file": "file 0644 fa0c9cdb {test-package_myslice}",
	},
	manifestPkgs: map[string]string{
		"test-package": "test-package v2 a2 h2",
	},
}, {
	summary: "Pinned archive does not have the package",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name: "test-package",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Reg(0644, "./file", "from foo"),
		}),
		Archives: []string{"foo"},
	}},
	release: map[string]string{
		"chisel.yaml": `
			format: v1
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					priority: 20
					public-keys: [test-key]
				bar:
					version: 22.04
					components: [main]
					suites: [jammy]
					priority: 10
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/test-package.yaml": `
			package: test-package
			archive: bar
			slices:
				myslice:
					contents:
						/file:
		`,
	},
	// Although archive "foo" does have the package, since archive "bar" has
	// been pinned in the slice definition, no other archives will be checked.
	error: `cannot find package "test-package" in archive\(s\)`,
}, {
	summary: "No archives have the package",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	pkgs:    []*testutil.TestPackage{},
	release: map[string]string{
		"chisel.yaml": `
			format: v1
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					priority: 20
					public-keys: [test-key]
				bar:
					version: 22.04
					components: [main]
					suites: [jammy]
					priority: 10
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/file:
		`,
	},
	error: `cannot find package "test-package" in archive\(s\)`,
}, {
	summary: "Negative priority archives are ignored when not explicitly pinned in package",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name: "test-package",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Reg(0644, "./file", "from foo"),
		}),
		Archives: []string{"foo"},
	}},
	release: map[string]string{
		"chisel.yaml": `
			format: v1
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					priority: -20
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/file:
		`,
	},
	error: `cannot find package "test-package" in archive\(s\)`,
}, {
	summary: "Negative priority archive explicitly pinned in package",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name:    "test-package",
		Hash:    "h1",
		Version: "v1",
		Arch:    "a1",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Reg(0644, "./file", "from foo"),
		}),
		Archives: []string{"foo"},
	}},
	release: map[string]string{
		"chisel.yaml": `
			format: v1
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					priority: -20
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/test-package.yaml": `
			package: test-package
			archive: foo
			slices:
				myslice:
					contents:
						/file:
		`,
	},
	filesystem: map[string]string{
		"/file": "file 0644 7a3e00f5",
	},
	manifestPaths: map[string]string{
		"/file": "file 0644 7a3e00f5 {test-package_myslice}",
	},
	manifestPkgs: map[string]string{
		"test-package": "test-package v1 a1 h1",
	},
}, {
	summary: "Multiple slices of same package",
	slices: []setup.SliceKey{
		{"test-package", "myslice1"},
		{"test-package", "myslice2"},
	},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice1:
					contents:
						/dir/file:
						/dir/file-copy:  {copy: /dir/file}
						/other-dir/file: {symlink: ../dir/file}
						/dir/foo/bar/:   {make: true, mode: 01777}
				myslice2:
					contents:
						/dir/other-file:
		`,
	},
	filesystem: map[string]string{
		"/dir/":           "dir 0755",
		"/dir/file":       "file 0644 cc55e2ec",
		"/dir/file-copy":  "file 0644 cc55e2ec",
		"/dir/foo/":       "dir 0755",
		"/dir/foo/bar/":   "dir 01777",
		"/dir/other-file": "file 0644 63d5dd49",
		"/other-dir/":     "dir 0755",
		"/other-dir/file": "symlink ../dir/file",
	},
	manifestPaths: map[string]string{
		"/dir/file":       "file 0644 cc55e2ec {test-package_myslice1}",
		"/dir/file-copy":  "file 0644 cc55e2ec {test-package_myslice1}",
		"/dir/foo/bar/":   "dir 01777 {test-package_myslice1}",
		"/dir/other-file": "file 0644 63d5dd49 {test-package_myslice2}",
		"/other-dir/file": "symlink ../dir/file {test-package_myslice1}",
	},
}, {
	summary: "Same glob in several entries with until:mutate and reading from script",
	slices: []setup.SliceKey{
		{"test-package", "myslice1"},
		{"test-package", "myslice2"},
	},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice1:
					contents:
						/dir/**: {until: mutate}
					mutate: |
						content.read("/dir/file")
				myslice2:
					contents:
						/dir/**:
					mutate: |
						content.read("/dir/file")
		`,
	},
	filesystem: map[string]string{
		"/dir/nested/other-file":        "file 0644 6b86b273",
		"/dir/several/":                 "dir 0755",
		"/dir/several/levels/":          "dir 0755",
		"/dir/several/levels/deep/file": "file 0644 6bc26dff",
		"/dir/":                         "dir 0755",
		"/dir/file":                     "file 0644 cc55e2ec",
		"/dir/nested/":                  "dir 0755",
		"/dir/nested/file":              "file 0644 84237a05",
		"/dir/other-file":               "file 0644 63d5dd49",
		"/dir/several/levels/deep/":     "dir 0755",
	},
	manifestPaths: map[string]string{
		"/dir/":                         "dir 0755 {test-package_myslice2}",
		"/dir/file":                     "file 0644 cc55e2ec {test-package_myslice2}",
		"/dir/nested/":                  "dir 0755 {test-package_myslice2}",
		"/dir/nested/file":              "file 0644 84237a05 {test-package_myslice2}",
		"/dir/nested/other-file":        "file 0644 6b86b273 {test-package_myslice2}",
		"/dir/other-file":               "file 0644 63d5dd49 {test-package_myslice2}",
		"/dir/several/":                 "dir 0755 {test-package_myslice2}",
		"/dir/several/levels/":          "dir 0755 {test-package_myslice2}",
		"/dir/several/levels/deep/":     "dir 0755 {test-package_myslice2}",
		"/dir/several/levels/deep/file": "file 0644 6bc26dff {test-package_myslice2}",
	},
}, {
	summary: "Overlapping globs, until:mutate and reading from script",
	slices: []setup.SliceKey{
		{"test-package", "myslice2"},
		{"test-package", "myslice1"},
	},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice1:
					contents:
						/dir/**:
					mutate: |
						content.read("/dir/file")
				myslice2:
					contents:
						/dir/nested/**: {until: mutate}
					mutate: |
						content.read("/dir/file")
		`,
	},
	filesystem: map[string]string{
		"/dir/":                         "dir 0755",
		"/dir/file":                     "file 0644 cc55e2ec",
		"/dir/nested/":                  "dir 0755",
		"/dir/nested/file":              "file 0644 84237a05",
		"/dir/nested/other-file":        "file 0644 6b86b273",
		"/dir/other-file":               "file 0644 63d5dd49",
		"/dir/several/":                 "dir 0755",
		"/dir/several/levels/":          "dir 0755",
		"/dir/several/levels/deep/":     "dir 0755",
		"/dir/several/levels/deep/file": "file 0644 6bc26dff",
	},
	manifestPaths: map[string]string{
		"/dir/":                         "dir 0755 {test-package_myslice1}",
		"/dir/file":                     "file 0644 cc55e2ec {test-package_myslice1}",
		"/dir/nested/":                  "dir 0755 {test-package_myslice1}",
		"/dir/nested/file":              "file 0644 84237a05 {test-package_myslice1}",
		"/dir/nested/other-file":        "file 0644 6b86b273 {test-package_myslice1}",
		"/dir/other-file":               "file 0644 63d5dd49 {test-package_myslice1}",
		"/dir/several/":                 "dir 0755 {test-package_myslice1}",
		"/dir/several/levels/":          "dir 0755 {test-package_myslice1}",
		"/dir/several/levels/deep/":     "dir 0755 {test-package_myslice1}",
		"/dir/several/levels/deep/file": "file 0644 6bc26dff {test-package_myslice1}",
	},
}, {
	summary: "Overlapping glob and single entry, until:mutate on entry and reading from script",
	slices: []setup.SliceKey{
		{"test-package", "myslice1"},
		{"test-package", "myslice2"},
	},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice1:
					contents:
						/dir/**:
					mutate: |
						content.read("/dir/file")
				myslice2:
					contents:
						/dir/file: {until: mutate}
					mutate: |
						content.read("/dir/file")
		`,
	},
	filesystem: map[string]string{
		"/dir/":                         "dir 0755",
		"/dir/file":                     "file 0644 cc55e2ec",
		"/dir/nested/":                  "dir 0755",
		"/dir/nested/file":              "file 0644 84237a05",
		"/dir/nested/other-file":        "file 0644 6b86b273",
		"/dir/other-file":               "file 0644 63d5dd49",
		"/dir/several/":                 "dir 0755",
		"/dir/several/levels/":          "dir 0755",
		"/dir/several/levels/deep/":     "dir 0755",
		"/dir/several/levels/deep/file": "file 0644 6bc26dff",
	},
	manifestPaths: map[string]string{
		"/dir/":                         "dir 0755 {test-package_myslice1}",
		"/dir/file":                     "file 0644 cc55e2ec {test-package_myslice1}",
		"/dir/nested/":                  "dir 0755 {test-package_myslice1}",
		"/dir/nested/file":              "file 0644 84237a05 {test-package_myslice1}",
		"/dir/nested/other-file":        "file 0644 6b86b273 {test-package_myslice1}",
		"/dir/other-file":               "file 0644 63d5dd49 {test-package_myslice1}",
		"/dir/several/":                 "dir 0755 {test-package_myslice1}",
		"/dir/several/levels/":          "dir 0755 {test-package_myslice1}",
		"/dir/several/levels/deep/":     "dir 0755 {test-package_myslice1}",
		"/dir/several/levels/deep/file": "file 0644 6bc26dff {test-package_myslice1}",
	},
}, {
	summary: "Overlapping glob and single entry, until:mutate on glob and reading from script",
	slices: []setup.SliceKey{
		{"test-package", "myslice1"},
		{"test-package", "myslice2"},
	},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice1:
					contents:
						/dir/**: {until: mutate}
					mutate: |
						content.read("/dir/file")
				myslice2:
					contents:
						/dir/file:
					mutate: |
						content.read("/dir/file")
		`,
	},
	filesystem: map[string]string{
		"/dir/":     "dir 0755",
		"/dir/file": "file 0644 cc55e2ec",
	},
	manifestPaths: map[string]string{
		"/dir/file": "file 0644 cc55e2ec {test-package_myslice2}",
	},
}, {
	summary: "Overlapping glob and single entry, until:mutate on both and reading from script",
	slices: []setup.SliceKey{
		{"test-package", "myslice1"},
		{"test-package", "myslice2"},
	},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice1:
					contents:
						/dir/**: {until: mutate}
					mutate: |
						content.read("/dir/file")
				myslice2:
					contents:
						/dir/file: {until: mutate}
					mutate: |
						content.read("/dir/file")
		`,
	},
	filesystem:    map[string]string{},
	manifestPaths: map[string]string{},
}, {
	summary: "Content not created in packages with until:mutate on one and reading from script",
	slices: []setup.SliceKey{
		{"test-package", "myslice1"},
		{"test-package", "myslice2"},
	},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice1:
					contents:
						/file: {text: foo, until: mutate}
					mutate: |
						content.read("/file")
				myslice2:
					contents:
						/file: {text: foo}
					mutate: |
						content.read("/file")
		`,
	},
	filesystem:    map[string]string{"/file": "file 0644 2c26b46b"},
	manifestPaths: map[string]string{"/file": "file 0644 2c26b46b {test-package_myslice1,test-package_myslice2}"},
}, {
	summary: "Install two packages, both are recorded",
	slices: []setup.SliceKey{
		{"test-package", "myslice"},
		{"other-package", "myslice"},
	},
	pkgs: []*testutil.TestPackage{{
		Name:    "test-package",
		Hash:    "h1",
		Version: "v1",
		Arch:    "a1",
		Data:    testutil.PackageData["test-package"],
	}, {
		Name:    "other-package",
		Hash:    "h2",
		Version: "v2",
		Arch:    "a2",
		Data:    testutil.PackageData["other-package"],
	}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
	`,
		"slices/mydir/other-package.yaml": `
			package: other-package
			slices:
				myslice:
					contents:
	`,
	},
	manifestPkgs: map[string]string{
		"test-package":  "test-package v1 a1 h1",
		"other-package": "other-package v2 a2 h2",
	},
}, {
	summary: "Two packages, only one is selected and recorded",
	slices: []setup.SliceKey{
		{"test-package", "myslice"},
	},
	pkgs: []*testutil.TestPackage{{
		Name:    "test-package",
		Hash:    "h1",
		Version: "v1",
		Arch:    "a1",
		Data:    testutil.PackageData["test-package"],
	}, {
		Name:    "other-package",
		Hash:    "h2",
		Version: "v2",
		Arch:    "a2",
		Data:    testutil.PackageData["other-package"],
	}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
	`,
		"slices/mydir/other-package.yaml": `
			package: other-package
			slices:
				myslice:
					contents:
	`,
	},
	manifestPkgs: map[string]string{
		"test-package": "test-package v1 a1 h1",
	},
}, {
	summary: "Relative paths are properly trimmed during extraction",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name: "test-package",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			// This particular path starting with "/foo" is chosen to test for
			// a particular bug; which appeared due to the usage of
			// strings.TrimLeft() instead strings.TrimPrefix() to determine a
			// relative path. Since TrimLeft takes in a cutset instead of a
			// prefix, the desired relative path was not produced.
			// See https://github.com/canonical/chisel/pull/145.
			testutil.Dir(0755, "./foo-bar/"),
		}),
	}},
	hackopt: func(c *C, opts *slicer.RunOptions) {
		opts.TargetDir = filepath.Join(filepath.Clean(opts.TargetDir), "foo")
		err := os.Mkdir(opts.TargetDir, 0755)
		c.Assert(err, IsNil)
	},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/foo-bar/:
					mutate: |
						content.list("/foo-bar/")
		`,
	},
}, {
	summary: "Producing a manifest is not mandatory",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	hackopt: func(c *C, opts *slicer.RunOptions) {
		// Remove the manifest slice that the tests add automatically.
		var index int
		for i, slice := range opts.Selection.Slices {
			if slice.Name == "manifest" {
				index = i
				break
			}
		}
		opts.Selection.Slices = append(opts.Selection.Slices[:index], opts.Selection.Slices[index+1:]...)
	},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
		`,
	},
}, {
	summary: "No valid archives defined due to invalid pro value",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	release: map[string]string{
		"chisel.yaml": `
			format: v1
			archives:
				invalid:
					version: 20.04
					components: [main]
					suites: [focal]
					priority: 10
					public-keys: [test-key]
					pro: unknown-value
		`,
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
		`,
	},
	error: `cannot find package "test-package" in archive\(s\)`,
}, {
	summary: "Valid hard link in two slices in the same package",
	slices: []setup.SliceKey{
		{"test-package", "slice1"},
		{"test-package", "slice2"}},
	pkgs: []*testutil.TestPackage{{
		Name: "test-package",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0755, "./"),
			testutil.Reg(0644, "./file", "foo"),
			testutil.Hlk(0644, "./hardlink", "./file"),
		}),
	}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
		package: test-package
		slices:
			slice1:
				contents:
					/file:
					/hardlink:
			slice2:
				contents:
					/hardlink:
	`,
	},
	filesystem: map[string]string{
		"/file":     "file 0644 2c26b46b <1>",
		"/hardlink": "file 0644 2c26b46b <1>",
	},
	manifestPaths: map[string]string{
		"/file":     "file 0644 2c26b46b <1> {test-package_slice1}",
		"/hardlink": "file 0644 2c26b46b <1> {test-package_slice1,test-package_slice2}",
	},
}, {
	summary: "Hard link entries can be extracted without extracting the regular file",
	slices: []setup.SliceKey{
		{"test-package", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name: "test-package",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0755, "./"),
			testutil.Reg(0644, "./file", "foo"),
			testutil.Hlk(0644, "./hardlink1", "./file"),
			testutil.Hlk(0644, "./hardlink2", "./file"),
		}),
	}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/hardlink*:
		`,
	},
	filesystem: map[string]string{
		"/hardlink1": "file 0644 2c26b46b <1>",
		"/hardlink2": "file 0644 2c26b46b <1>",
	},
	manifestPaths: map[string]string{
		"/hardlink1": "file 0644 2c26b46b <1> {test-package_myslice}",
		"/hardlink2": "file 0644 2c26b46b <1> {test-package_myslice}",
	},
}, {
	summary: "Hard link identifier for different groups",
	slices: []setup.SliceKey{
		{"test-package", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name: "test-package",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0755, "./"),
			testutil.Reg(0644, "./file1", "text for file1"),
			testutil.Reg(0644, "./file2", "text for file2"),
			testutil.Hlk(0644, "./hardlink1", "./file1"),
			testutil.Hlk(0644, "./hardlink2", "./file2"),
		}),
	}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/**:
		`,
	},
	filesystem: map[string]string{
		"/file1":     "file 0644 df82bbbd <1>",
		"/file2":     "file 0644 dcddda2e <2>",
		"/hardlink1": "file 0644 df82bbbd <1>",
		"/hardlink2": "file 0644 dcddda2e <2>",
	},
	manifestPaths: map[string]string{
		"/file1":     "file 0644 df82bbbd <1> {test-package_myslice}",
		"/file2":     "file 0644 dcddda2e <2> {test-package_myslice}",
		"/hardlink1": "file 0644 df82bbbd <1> {test-package_myslice}",
		"/hardlink2": "file 0644 dcddda2e <2> {test-package_myslice}",
	},
}, {
	summary: "Single hard link entry can be extracted without regular file and no hard links are created",
	slices: []setup.SliceKey{
		{"test-package", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name: "test-package",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0755, "./"),
			testutil.Reg(0644, "./file", "foo"),
			testutil.Hlk(0644, "./hardlink", "./file"),
		}),
	}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/hardlink:
		`,
	},
	filesystem: map[string]string{
		"/hardlink": "file 0644 2c26b46b",
	},
	manifestPaths: map[string]string{
		"/hardlink": "file 0644 2c26b46b {test-package_myslice}",
	},
}, {
	summary: "Hard link to symlink does not follow symlink",
	slices: []setup.SliceKey{
		{"test-package", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name: "test-package",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0755, "./"),
			testutil.Dir(0755, "./"),
			testutil.Reg(0644, "./file", "foo"),
			testutil.Lnk(0644, "./symlink", "./file"),
			testutil.Hlk(0644, "./hardlink", "./symlink"),
		}),
	}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
					package: test-package
					slices:
						myslice:
							contents:
								/hardlink:
								/symlink:
				`,
	},
	filesystem: map[string]string{
		"/hardlink": "symlink ./file <1>",
		"/symlink":  "symlink ./file <1>",
	},
	manifestPaths: map[string]string{
		"/symlink":  "symlink ./file <1> {test-package_myslice}",
		"/hardlink": "symlink ./file <1> {test-package_myslice}",
	},
}, {
	summary: "Hard link identifiers are unique across packages",
	slices: []setup.SliceKey{
		{"test-package1", "myslice"},
		{"test-package2", "myslice"},
	},
	pkgs: []*testutil.TestPackage{{
		Name: "test-package1",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0755, "./"),
			testutil.Reg(0644, "./file1", "foo"),
			testutil.Hlk(0644, "./hardlink1", "./file1"),
		}),
	}, {
		Name: "test-package2",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0755, "./"),
			testutil.Reg(0644, "./file2", "foo"),
			testutil.Hlk(0644, "./hardlink2", "./file2"),
		}),
	}},
	release: map[string]string{
		"slices/mydir/test-package1.yaml": `
			package: test-package1
			slices:
				myslice:
					contents:
						/file1:
						/hardlink1:
		`,
		"slices/mydir/test-package2.yaml": `
			package: test-package2
			slices:
				myslice:
					contents:
						/file2:
						/hardlink2:
		`,
	},
	filesystem: map[string]string{
		"/file1":     "file 0644 2c26b46b <1>",
		"/hardlink1": "file 0644 2c26b46b <1>",
		"/file2":     "file 0644 2c26b46b <2>",
		"/hardlink2": "file 0644 2c26b46b <2>",
	},
	manifestPaths: map[string]string{
		"/file1":     "file 0644 2c26b46b <1> {test-package1_myslice}",
		"/hardlink1": "file 0644 2c26b46b <1> {test-package1_myslice}",
		"/file2":     "file 0644 2c26b46b <2> {test-package2_myslice}",
		"/hardlink2": "file 0644 2c26b46b <2> {test-package2_myslice}",
	},
}, {
	summary: "Mutations for hard links are forbidden",
	slices: []setup.SliceKey{
		{"test-package", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name: "test-package",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0755, "./"),
			testutil.Reg(0644, "./file", "foo"),
			testutil.Hlk(0644, "./hardlink", "./file"),
		}),
	}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/file:
						/hardlink: {mutable: true}
					mutate: |
						content.write("/hardlink", "new content")
		`,
	},
	error: `slice test-package_myslice: cannot mutate a hard link: /hardlink`,
}, {
	summary: "Hard links can be marked as mutable, but not mutated",
	slices: []setup.SliceKey{
		{"test-package", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name: "test-package",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0755, "./"),
			testutil.Reg(0644, "./file", "foo"),
			testutil.Hlk(0644, "./hardlink", "./file"),
		}),
	}},
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/file:
						/hardlink: {mutable: true}
		`,
	},
	filesystem: map[string]string{
		"/file":     "file 0644 2c26b46b <1>",
		"/hardlink": "file 0644 2c26b46b <1>",
	},
	manifestPaths: map[string]string{
		"/file":     "file 0644 2c26b46b <1> {test-package_myslice}",
		"/hardlink": "file 0644 2c26b46b <1> {test-package_myslice}",
	},
}}

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
			armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
`

func (s *S) TestRun(c *C) {
	for _, test := range slicerTests {
		for _, testSlices := range testutil.Permutations(test.slices) {
			c.Logf("Summary: %s", test.summary)

			if _, ok := test.release["chisel.yaml"]; !ok {
				test.release["chisel.yaml"] = defaultChiselYaml
			}
			if test.pkgs == nil {
				test.pkgs = []*testutil.TestPackage{{
					Name: "test-package",
					Data: testutil.PackageData["test-package"],
				}}
			}
			for _, pkg := range test.pkgs {
				// We need to set these fields for manifest validation.
				if pkg.Arch == "" {
					pkg.Arch = "arch"
				}
				if pkg.Hash == "" {
					pkg.Hash = "hash"
				}
				if pkg.Version == "" {
					pkg.Version = "version"
				}
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

			// Create a manifest slice and add it to the selection.
			manifestPackage := test.slices[0].Package
			manifestPath := "/chisel-data/manifest.wall"
			release.Packages[manifestPackage].Slices["manifest"] = &setup.Slice{
				Package:   manifestPackage,
				Name:      "manifest",
				Essential: nil,
				Contents: map[string]setup.PathInfo{
					"/chisel-data/**": {
						Kind:     "generate",
						Generate: "manifest",
					},
				},
				Scripts: setup.SliceScripts{},
			}
			testSlices = append(testSlices, setup.SliceKey{
				Package: manifestPackage,
				Slice:   "manifest",
			})

			selection, err := setup.Select(release, testSlices)
			c.Assert(err, IsNil)

			archives := map[string]archive.Archive{}
			for name, setupArchive := range release.Archives {
				pkgs := make(map[string]*testutil.TestPackage)
				for _, pkg := range test.pkgs {
					if len(pkg.Archives) == 0 || slices.Contains(pkg.Archives, name) {
						pkgs[pkg.Name] = pkg
					}
				}
				archive := &testutil.TestArchive{
					Opts: archive.Options{
						Label:      setupArchive.Name,
						Version:    setupArchive.Version,
						Suites:     setupArchive.Suites,
						Components: setupArchive.Components,
						Pro:        setupArchive.Pro,
						Arch:       test.arch,
					},
					Packages: pkgs,
				}
				archives[name] = archive
			}

			options := slicer.RunOptions{
				Selection: selection,
				Archives:  archives,
				TargetDir: c.MkDir(),
			}
			if test.hackopt != nil {
				test.hackopt(c, &options)
			}
			err = slicer.Run(&options)
			if test.error != "" {
				c.Assert(err, ErrorMatches, test.error)
				continue
			}
			c.Assert(err, IsNil)

			if test.filesystem == nil && test.manifestPaths == nil && test.manifestPkgs == nil {
				continue
			}
			mfest := readManifest(c, options.TargetDir, manifestPath)

			// Assert state of final filesystem.
			if test.filesystem != nil {
				filesystem := testutil.TreeDump(options.TargetDir)
				c.Assert(filesystem["/chisel-data/"], Not(HasLen), 0)
				c.Assert(filesystem[manifestPath], Not(HasLen), 0)
				delete(filesystem, "/chisel-data/")
				delete(filesystem, manifestPath)
				c.Assert(filesystem, DeepEquals, test.filesystem)
			}

			// Assert state of the files recorded in the manifest.
			if test.manifestPaths != nil {
				pathsDump, err := treeDumpManifestPaths(mfest)
				c.Assert(err, IsNil)
				c.Assert(pathsDump[manifestPath], Not(HasLen), 0)
				delete(pathsDump, manifestPath)
				c.Assert(pathsDump, DeepEquals, test.manifestPaths)
			}

			// Assert state of the packages recorded in the manifest.
			if test.manifestPkgs != nil {
				pkgsDump, err := dumpManifestPkgs(mfest)
				c.Assert(err, IsNil)
				c.Assert(pkgsDump, DeepEquals, test.manifestPkgs)
			}
		}
	}
}

func treeDumpManifestPaths(mfest *manifest.Manifest) (map[string]string, error) {
	result := make(map[string]string)
	err := mfest.IteratePaths("", func(path *manifest.Path) error {
		var fsDump string
		switch {
		case strings.HasSuffix(path.Path, "/"):
			fsDump = fmt.Sprintf("dir %s", path.Mode)
		case path.Link != "":
			fsDump = fmt.Sprintf("symlink %s", path.Link)
		default: // Regular
			if path.Size == 0 {
				fsDump = fmt.Sprintf("file %s empty", path.Mode)
			} else if path.FinalSHA256 != "" {
				fsDump = fmt.Sprintf("file %s %s %s", path.Mode, path.SHA256[:8], path.FinalSHA256[:8])
			} else {
				fsDump = fmt.Sprintf("file %s %s", path.Mode, path.SHA256[:8])
			}
		}

		if path.HardLinkID != 0 {
			// Append <hardLinkID> to the end of the path dump.
			fsDump = fmt.Sprintf("%s <%d>", fsDump, path.HardLinkID)
		}

		// append {slice1, ..., sliceN} to the end of the path dump.
		slicesStr := make([]string, 0, len(path.Slices))
		for _, slice := range path.Slices {
			slicesStr = append(slicesStr, slice)
		}
		sort.Strings(slicesStr)
		result[path.Path] = fmt.Sprintf("%s {%s}", fsDump, strings.Join(slicesStr, ","))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func dumpManifestPkgs(mfest *manifest.Manifest) (map[string]string, error) {
	result := map[string]string{}
	err := mfest.IteratePackages(func(pkg *manifest.Package) error {
		result[pkg.Name] = fmt.Sprintf("%s %s %s %s", pkg.Name, pkg.Version, pkg.Arch, pkg.Digest)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func readManifest(c *C, targetDir, manifestPath string) *manifest.Manifest {
	f, err := os.Open(path.Join(targetDir, manifestPath))
	c.Assert(err, IsNil)
	defer f.Close()
	r, err := zstd.NewReader(f)
	c.Assert(err, IsNil)
	defer r.Close()
	mfest, err := manifest.Read(r)
	c.Assert(err, IsNil)
	err = manifest.Validate(mfest)
	c.Assert(err, IsNil)

	// Assert that the mode of the manifest.wall file matches the one recorded
	// in the manifest itself.
	s, err := os.Stat(path.Join(targetDir, manifestPath))
	c.Assert(err, IsNil)
	c.Assert(s.Mode(), Equals, fs.FileMode(0644))
	err = mfest.IteratePaths(manifestPath, func(p *manifest.Path) error {
		c.Assert(p.Mode, Equals, fmt.Sprintf("%#o", fs.FileMode(0644)))
		return nil
	})
	c.Assert(err, IsNil)

	return mfest
}
