package setup_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/slicer"
	"github.com/canonical/chisel/internal/testutil"
	. "gopkg.in/check.v1"
)

type testRun struct {
	summary string
	arch    string
	release map[string]string
	slices  []setup.SliceKey
	result  map[string]string
	error   string
}

var testReleasesOfBaseFiles = map[string]string{
	"chisel.yaml": `
		format: chisel-v1
		archives:
			ubuntu:
				version: 22.04
				components: [main, universe]
	`,
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
			yourslice:
				contents:
					/tmp/file1: {text: data1, arch: amd64}
					/tmp/file2: {text: data1, arch: i386}
					/tmp/file3: {text: data1, arch: [i386, amd64]}
					/usr/bin/hello1: {copy: /usr/bin/hello, arch: amd64}
					/usr/bin/hello2: {copy: /usr/bin/hello, arch: i386}
					/usr/bin/hello3: {copy: /usr/bin/hello, arch: [i386, amd64]}
			theirslice:
				essential:
					- base-files_yourslice
				contents:
					/tmp/file4: {text: data2}
	`,
}

var testRuns = []testRun{{
	summary: "First fresh install of a few slices",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: testReleasesOfBaseFiles,
	result: map[string]string{
		"/usr/":                          "dir 0755",
		"/usr/bin/":                      "dir 0755",
		"/usr/bin/hello":                 "file 0775 eaf29575",
		"/usr/bin/hallo":                 "file 0775 eaf29575",
		"/bin/":                          "dir 0755",
		"/bin/hallo":                     "symlink ../usr/bin/hallo",
		"/etc/":                          "dir 0755",
		"/etc/dir/":                      "dir 0755",
		"/etc/dir/sub/":                  "dir 01777",
		"/etc/passwd":                    "file 0644 5b41362b",
		"/var/":                          "dir 0755",
		"/var/lib/":                      "dir 0755",
		"/var/lib/chisel/":               "dir 0755",
		"/var/lib/chisel/chisel.db":      "file 0755 ce9d2c42",
	},
}, {
	summary: "Second Run, consists of previous slices as well",
	arch:    "amd64",
	slices:  []setup.SliceKey{{"base-files", "myslice"}, {"base-files", "yourslice"}},
	release: testReleasesOfBaseFiles,
	result: map[string]string{
		"/tmp/":                          "dir 01777",
		"/tmp/file1":                     "file 0644 5b41362b",
		"/tmp/file3":                     "file 0644 5b41362b",
		"/usr/":                          "dir 0755",
		"/usr/bin/":                      "dir 0755",
		"/usr/bin/hello":                 "file 0775 eaf29575",
		"/usr/bin/hello1":                "file 0775 eaf29575",
		"/usr/bin/hello3":                "file 0775 eaf29575",
		"/usr/bin/hallo":                 "file 0775 eaf29575",
		"/bin/":                          "dir 0755",
		"/bin/hallo":                     "symlink ../usr/bin/hallo",
		"/etc/":                          "dir 0755",
		"/etc/dir/":                      "dir 0755",
		"/etc/dir/sub/":                  "dir 01777",
		"/etc/passwd":                    "file 0644 5b41362b",
		"/var/":                          "dir 0755",
		"/var/lib/":                      "dir 0755",
		"/var/lib/chisel/":               "dir 0755",
		"/var/lib/chisel/chisel.db":      "file 0755 194620de",
	},
}, {
	summary: "Third Run, new slice requiring previous slice",
	arch:    "amd64",
	slices:  []setup.SliceKey{{"base-files", "theirslice"}},
	release: testReleasesOfBaseFiles,
	result: map[string]string{
		"/tmp/":                          "dir 01777",
		"/tmp/file1":                     "file 0644 5b41362b",
		"/tmp/file3":                     "file 0644 5b41362b",
		"/tmp/file4":                     "file 0644 d98cf53e",
		"/usr/":                          "dir 0755",
		"/usr/bin/":                      "dir 0755",
		"/usr/bin/hello":                 "file 0775 eaf29575",
		"/usr/bin/hello1":                "file 0775 eaf29575",
		"/usr/bin/hello3":                "file 0775 eaf29575",
		"/usr/bin/hallo":                 "file 0775 eaf29575",
		"/bin/":                          "dir 0755",
		"/bin/hallo":                     "symlink ../usr/bin/hallo",
		"/etc/":                          "dir 0755",
		"/etc/dir/":                      "dir 0755",
		"/etc/dir/sub/":                  "dir 01777",
		"/etc/passwd":                    "file 0644 5b41362b",
		"/var/":                          "dir 0755",
		"/var/lib/":                      "dir 0755",
		"/var/lib/chisel/":               "dir 0755",
		"/var/lib/chisel/chisel.db":      "file 0755 03c28243",
	},
}, {
	summary: "Fourth Run, reinstall previous slice",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: testReleasesOfBaseFiles,
	result: map[string]string{
		"/tmp/":                          "dir 01777",
		"/tmp/file1":                     "file 0644 5b41362b",
		"/tmp/file3":                     "file 0644 5b41362b",
		"/tmp/file4":                     "file 0644 d98cf53e",
		"/usr/":                          "dir 0755",
		"/usr/bin/":                      "dir 0755",
		"/usr/bin/hello":                 "file 0775 eaf29575",
		"/usr/bin/hello1":                "file 0775 eaf29575",
		"/usr/bin/hello3":                "file 0775 eaf29575",
		"/usr/bin/hallo":                 "file 0775 eaf29575",
		"/bin/":                          "dir 0755",
		"/bin/hallo":                     "symlink ../usr/bin/hallo",
		"/etc/":                          "dir 0755",
		"/etc/dir/":                      "dir 0755",
		"/etc/dir/sub/":                  "dir 01777",
		"/etc/passwd":                    "file 0644 5b41362b",
		"/var/":                          "dir 0755",
		"/var/lib/":                      "dir 0755",
		"/var/lib/chisel/":               "dir 0755",
		"/var/lib/chisel/chisel.db":      "file 0755 03c28243",
	},
}}

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

func(test *testRun) installTestSlices(c *C, targetDir string, db *setup.ChiselDB) error {
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

	selection, err := setup.Select(release, test.slices, db)
	c.Assert(err, IsNil)

	archives := map[string]archive.Archive{
		"ubuntu": &testArchive{
			arch: test.arch,
			pkgs: map[string][]byte{
				"base-files": testutil.PackageData["base-files"],
			},
		},
	}

	options := slicer.RunOptions{
		Selection: selection,
		Archives:  archives,
		TargetDir: targetDir,
	}
	err = slicer.Run(&options)
	c.Assert(err, IsNil)

	err = db.WriteInstalledSlices(targetDir, selection.Slices)
	c.Assert(err, IsNil)

	if test.result != nil {
		c.Assert(testutil.TreeDump(targetDir), DeepEquals, test.result)
	}

	return nil
}

func(s *S) TestChiselDB(c *C) {
	targetDir := c.MkDir()

	for _, test := range testRuns {
		c.Logf("Summary: %s", test.summary)

		db, err := setup.ReadDB(targetDir)
		c.Assert(err, IsNil)

		err = test.installTestSlices(c, targetDir, db)
		c.Assert(err, IsNil)

		// check if installed slices are appended in "db"
		for _, slice := range test.slices {
			isInstalled := db.IsSliceInstalled(slice)
			c.Assert(isInstalled, DeepEquals, true)
		}

		db, err = setup.ReadDB(targetDir)
		c.Assert(err, IsNil)

		// check if installed slices are present in the database file
		for _, slice := range test.slices {
			isInstalled := db.IsSliceInstalled(slice)
			c.Assert(isInstalled, DeepEquals, true)
		}
	}
}