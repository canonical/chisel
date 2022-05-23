package slicer_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/testutil"
	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/slicer"
)

type slicerTest struct {
	summary string
	release map[string]string
	slices  []setup.SliceKey
	result  map[string]string
	error   string
}

var slicerTests = []slicerTest{{
	summary: "Simple extraction",
	slices:  []setup.SliceKey{{"run-one", "myslice"}},
	release: map[string]string{
		"slices/mydir/run-one.yaml": `
			package: run-one
			slices:
				myslice:
					contents:
						/usr/bin/run-one:
						/usr/bin/run-two: {copy: /usr/bin/run-one}
						/bin/run-lnk:     {symlink: ../usr/bin/run-two}
						/etc/passwd:      {text: data1}
						/etc/dir/:        {make: true}
		`,
	},
	result: map[string]string{
		"/usr/":            "dir 0755",
		"/usr/bin/":        "dir 0755",
		"/usr/bin/run-one": "file 0755 95a2a697",
		"/usr/bin/run-two": "file 0755 95a2a697",
		"/bin/":            "dir 0755",
		"/bin/run-lnk":     "symlink ../usr/bin/run-two",
		"/etc/":            "dir 0755",
		"/etc/dir/":        "dir 0755",
		"/etc/passwd":      "file 0644 5b41362b",

	},
}}

const defaultChiselYaml = `
	format: chisel-v1
	archives:
		ubuntu:
			version: 22.04
			components: [main, universe]
`

type testArchive struct {
	pkgs map[string][]byte
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
				pkgs: map[string][]byte{
					"run-one": testutil.PackageData["run-one"],
				},
			},
		}

		targetDir := c.MkDir()
		options := slicer.RunOptions{
			Selection: selection,
			Archives:  archives,
			TargetDir: targetDir,
		}
		err = slicer.Run(&options)
		if test.error == "" {
			c.Assert(err, IsNil)
		} else {
			c.Assert(err, ErrorMatches, test.error)
			continue
		}

		c.Assert(testutil.TreeDump(targetDir), DeepEquals, test.result)
	}
}
