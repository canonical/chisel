package slicer_test

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
	pkgs       map[string][]byte
	slices     []setup.SliceKey
	hackopt    func(c *C, opts *slicer.RunOptions)
	filesystem map[string]string
	report     map[string]string
	error      string
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
	report: map[string]string{
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
	report: map[string]string{
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
	report: map[string]string{
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
	report: map[string]string{
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
	report: map[string]string{
		"/parent/new/": "dir 0755 {test-package_myslice}",
	},
}, {
	summary: "Create new file using glob and preserve parent directory permissions",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	pkgs: map[string][]byte{
		"test-package": testutil.PackageData["test-package"],
	},
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
	report: map[string]string{
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
	report: map[string]string{
		"/dir/nested/copy-1": "file 0644 84237a05 {test-package_myslice}",
		"/dir/nested/copy-3": "file 0644 84237a05 {test-package_myslice}",
		"/dir/text-file-1":   "file 0644 5b41362b {test-package_myslice}",
		"/dir/text-file-3":   "file 0644 5b41362b {test-package_myslice}",
	},
}, {
	summary: "Copyright is installed",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	pkgs: map[string][]byte{
		// Add the copyright entries to the package.
		"test-package": testutil.MustMakeDeb(append(testutil.TestPackageEntries, testPackageCopyrightEntries...)),
	},
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
		// Hardcoded copyright entries.
		"/usr/":                                 "dir 0755",
		"/usr/share/":                           "dir 0755",
		"/usr/share/doc/":                       "dir 0755",
		"/usr/share/doc/test-package/":          "dir 0755",
		"/usr/share/doc/test-package/copyright": "file 0644 c2fca2aa",
	},
	report: map[string]string{
		"/dir/file": "file 0644 cc55e2ec {test-package_myslice}",
	},
}, {
	summary: "Install two packages",
	slices: []setup.SliceKey{
		{"test-package", "myslice"},
		{"other-package", "myslice"}},
	pkgs: map[string][]byte{
		"test-package":  testutil.PackageData["test-package"],
		"other-package": testutil.PackageData["other-package"],
	},
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
	report: map[string]string{
		"/foo/":     "dir 0755 {test-package_myslice}",
		"/dir/file": "file 0644 cc55e2ec {test-package_myslice}",
		"/bar/":     "dir 0755 {other-package_myslice}",
		"/file":     "file 0644 fc02ca0e {other-package_myslice}",
	},
}, {
	summary: "Install two packages, explicit path has preference over implicit parent",
	slices: []setup.SliceKey{
		{"implicit-parent", "myslice"},
		{"explicit-dir", "myslice"}},
	pkgs: map[string][]byte{
		"implicit-parent": testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0755, "./dir/"),
			testutil.Reg(0644, "./dir/file", "random"),
		}),
		"explicit-dir": testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(01777, "./dir/"),
		}),
	},
	release: map[string]string{
		"slices/mydir/implicit-parent.yaml": `
			package: implicit-parent
			slices:
				myslice:
					contents:
						/dir/file:
		`,
		"slices/mydir/explicit-dir.yaml": `
			package: explicit-dir
			slices:
				myslice:
					contents:
						/dir/:
		`,
	},
	filesystem: map[string]string{
		"/dir/":     "dir 01777",
		"/dir/file": "file 0644 a441b15f",
	},
	report: map[string]string{
		"/dir/":     "dir 01777 {explicit-dir_myslice}",
		"/dir/file": "file 0644 a441b15f {implicit-parent_myslice}",
	},
}, {
	summary: "Valid same file in two slices in different packages",
	slices: []setup.SliceKey{
		{"test-package", "myslice"},
		{"other-package", "myslice"}},
	pkgs: map[string][]byte{
		"test-package":  testutil.PackageData["test-package"],
		"other-package": testutil.PackageData["other-package"],
	},
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
	report: map[string]string{
		// Note: This is the only case where two slices can declare the same
		// file without conflicts.
		// TODO which slice(s) should own the file.
		"/textFile": "file 0644 c6c83d10 {other-package_myslice}",
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
	report: map[string]string{
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
	report: map[string]string{
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
	report: map[string]string{
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
	report: map[string]string{},
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
	report: map[string]string{
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
	report: map[string]string{
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
	pkgs: map[string][]byte{
		"copyright-symlink-openssl": testutil.MustMakeDeb(packageEntries["copyright-symlink-openssl"]),
		"copyright-symlink-libssl3": testutil.MustMakeDeb(packageEntries["copyright-symlink-libssl3"]),
	},
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
	summary: "Non-default archive",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
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
		"slices/mydir/test-package.yaml": `
			package: test-package
			archive: bar
			slices:
				myslice:
					contents:
						/dir/nested/file:
		`,
	},
	filesystem: map[string]string{
		"/dir/":            "dir 0755",
		"/dir/nested/":     "dir 0755",
		"/dir/nested/file": "file 0644 84237a05",
	},
	report: map[string]string{
		"/dir/nested/file": "file 0644 84237a05 {test-package_myslice}",
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
	report: map[string]string{
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
	report: map[string]string{
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
	report: map[string]string{
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
	report: map[string]string{
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
	report: map[string]string{
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
	filesystem: map[string]string{},
	report:     map[string]string{},
}, {
	summary: "Relative paths are properly trimmed during extraction",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	pkgs: map[string][]byte{
		"test-package": testutil.MustMakeDeb([]testutil.TarEntry{
			// This particular path starting with "/foo" is chosen to test for
			// a particular bug; which appeared due to the usage of
			// strings.TrimLeft() instead strings.TrimPrefix() to determine a
			// relative path. Since TrimLeft takes in a cutset instead of a
			// prefix, the desired relative path was not produced.
			// See https://github.com/canonical/chisel/pull/145.
			testutil.Dir(0755, "./foo-bar/"),
		}),
	},
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
		for _, slices := range testutil.Permutations(test.slices) {
			c.Logf("Summary: %s", test.summary)

			if _, ok := test.release["chisel.yaml"]; !ok {
				test.release["chisel.yaml"] = string(defaultChiselYaml)
			}

			if test.pkgs == nil {
				test.pkgs = map[string][]byte{
					"test-package": testutil.PackageData["test-package"],
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

			selection, err := setup.Select(release, slices)
			c.Assert(err, IsNil)

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
					pkgs: test.pkgs,
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
				c.Assert(testutil.TreeDump(targetDir), DeepEquals, test.filesystem)
			}

			if test.report != nil {
				c.Assert(treeDumpReport(report), DeepEquals, test.report)
			}
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
			fsDump = fmt.Sprintf("dir %#o", fperm)
		case fs.ModeSymlink:
			fsDump = fmt.Sprintf("symlink %s", entry.Link)
		case 0: // Regular
			if entry.Size == 0 {
				fsDump = fmt.Sprintf("file %#o empty", entry.Mode.Perm())
			} else if entry.FinalHash != "" {
				fsDump = fmt.Sprintf("file %#o %s %s", fperm, entry.Hash[:8], entry.FinalHash[:8])
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
		sort.Strings(slicesStr)
		result[entry.Path] = fmt.Sprintf("%s {%s}", fsDump, strings.Join(slicesStr, ","))
	}
	return result
}
