package slicer_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/slicer"
	"github.com/canonical/chisel/internal/testutil"
)

type slicerTest struct {
	summary string
	arch    string
	release map[string]string
	slices  []setup.SliceKey
	hackopt func(c *C, opts *slicer.RunOptions)
	result  map[string]string
	error   string
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
	result: map[string]string{
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
	result: map[string]string{
		"/usr/":          "dir 0755",
		"/usr/bin/":      "dir 0755",
		"/usr/bin/hello": "file 0775 eaf29575",
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
	result: map[string]string{
		"/tmp/":    "dir 01777", // This is the magic.
		"/tmp/new": "file 0644 5b41362b",
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
	result: map[string]string{
		"/tmp/":        "dir 01777", // This is the magic.
		"/tmp/new/":    "dir 0755",
		"/tmp/new/sub": "file 0644 5b41362b",
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
	result: map[string]string{
		"/tmp/":     "dir 01777", // This is the magic.
		"/tmp/new/": "dir 0755",
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
	result: map[string]string{
		"/tmp/":           "dir 01777",
		"/tmp/file1":      "file 0644 5b41362b",
		"/tmp/file3":      "file 0644 5b41362b",
		"/usr/":           "dir 0755",
		"/usr/bin/":       "dir 0755",
		"/usr/bin/hello1": "file 0775 eaf29575",
		"/usr/bin/hello3": "file 0775 eaf29575",
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
	result: map[string]string{
		"/tmp/":      "dir 01777",
		"/tmp/file1": "file 0644 d98cf53e",
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
	result: map[string]string{
		"/tmp/":      "dir 01777",
		"/tmp/file1": "file 0644 5b41362b",
		"/foo/":      "dir 0755",
		"/foo/file2": "file 0644 5b41362b",
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
	result: map[string]string{
		"/tmp/":      "dir 01777",
		"/foo/":      "dir 0755",
		"/foo/file2": "file 0644 5b41362b",
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
	result: map[string]string{
		"/usr/": "dir 0755",
		"/etc/": "dir 0755",
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
	result: map[string]string{
		"/usr/":          "dir 0755",
		"/usr/bin/":      "dir 0755",
		"/usr/bin/hallo": "file 0775 eaf29575",
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
	error: `slice base-files_myslice: cannot read file which is not selected: /a/d`,
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
	error: `slice base-files_myslice: readdirent /a/b/c: not a directory`,
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
	error: `slice base-files_myslice: cannot read file which is not selected: /etc`,
}}

const defaultChiselYaml = `
	format: chisel-v1
	archives:
		ubuntu:
			version: 22.04
			components: [main, universe]
`

type testArchive struct {
	arch string
	pkgs map[string][]byte
}

func (a *testArchive) Options() *archive.Options {
	return &archive.Options{Arch: a.arch}
}

func (a *testArchive) Fetch(pkg string) (io.ReadCloser, error) {
	if data, ok := a.pkgs[pkg]; ok {
		return ioutil.NopCloser(bytes.NewBuffer(data)), nil
	}
	return nil, fmt.Errorf("attempted to open %q package", pkg)
}

func (a *testArchive) Exists(pkg string) bool {
	_, ok := a.pkgs[pkg]
	return ok
}

func (s *S) TestRun(c *C) {
	for _, test := range slicerTests {
		c.Logf("Summary: %s", test.summary)

		if _, ok := test.release["chisel.yaml"]; !ok {
			test.release["chisel.yaml"] = string(defaultChiselYaml)
		}

		releaseDir := c.MkDir()
		for path, data := range test.release {
			fpath := filepath.Join(releaseDir, path)
			err := os.MkdirAll(filepath.Dir(fpath), 0755)
			c.Assert(err, IsNil)
			err = ioutil.WriteFile(fpath, testutil.Reindent(data), 0644)
			c.Assert(err, IsNil)
		}

		release, err := setup.ReadRelease(releaseDir)
		c.Assert(err, IsNil)

		selection, err := setup.Select(release, test.slices)
		c.Assert(err, IsNil)

		archives := map[string]archive.Archive{
			"ubuntu": &testArchive{
				arch: test.arch,
				pkgs: map[string][]byte{
					"base-files": testutil.PackageData["base-files"],
				},
			},
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
		err = slicer.Run(&options)
		if test.error == "" {
			c.Assert(err, IsNil)
		} else {
			c.Assert(err, ErrorMatches, test.error)
			continue
		}

		if test.result != nil {
			result := make(map[string]string, len(copyrightEntries)+len(test.result))
			for k, v := range copyrightEntries {
				result[k] = v
			}
			for k, v := range test.result {
				result[k] = v
			}
			c.Assert(testutil.TreeDump(targetDir), DeepEquals, result)
		}
	}
}
