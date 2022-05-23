package deb_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/deb"
	"github.com/canonical/chisel/internal/testutil"
)

type extractTest struct {
	summary string
	pkgdata []byte
	options deb.ExtractOptions
	result  map[string]string
}

var extractTests = []extractTest{{
	summary: "Extract nothing",
	pkgdata: testutil.PackageData["run-one"],
	options: deb.ExtractOptions{
		Extract: nil,
	},
	result: map[string]string{},
}, {
	summary: "Extract a few entries",
	pkgdata: testutil.PackageData["run-one"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/usr/bin/run-one": []deb.ExtractInfo{{
				Path: "/usr/bin/run-one",
			}},
			"/usr/share/man/man1/run-one.1.gz": []deb.ExtractInfo{{
				Path: "/usr/share/man/man1/run-one.1.gz",
			}},
			"/usr/share/doc/": []deb.ExtractInfo{{
				Path: "/usr/share/doc/",
			}},
		},
	},
	result: map[string]string{
		"/usr/":                            "dir 0755",
		"/usr/bin/":                        "dir 0755",
		"/usr/bin/run-one":                 "file 0755 95a2a697",
		"/usr/share/":                      "dir 0755",
		"/usr/share/man/":                  "dir 0755",
		"/usr/share/man/man1/":             "dir 0755",
		"/usr/share/man/man1/run-one.1.gz": "file 0644 e47d052f",
		"/usr/share/doc/":                  "dir 0755",
	},
}, {
	summary: "Copy a couple of entries elsewhere",
	pkgdata: testutil.PackageData["run-one"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/usr/bin/run-one": []deb.ExtractInfo{{
				Path: "/usr/foo/bin/run-one-2",
				Mode: 0600,
			}},
			"/usr/share/": []deb.ExtractInfo{{
				Path: "/usr/other/",
				Mode: 0700,
			}},
		},
	},
	result: map[string]string{
		"/usr/":                  "dir 0755",
		"/usr/foo/":              "dir 0755",
		"/usr/foo/bin/":          "dir 0755",
		"/usr/foo/bin/run-one-2": "file 0600 95a2a697",
		"/usr/other/":            "dir 0700",
	},
}, {
	summary: "Extract symlink",
	pkgdata: testutil.PackageData["run-one"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/usr/bin/run-this-one": []deb.ExtractInfo{{
				Path: "/usr/bin/run-this-one",
			}},
		},
	},
	result: map[string]string{
		"/usr/":                 "dir 0755",
		"/usr/bin/":             "dir 0755",
		"/usr/bin/run-this-one": "symlink run-one",
	},
}, {
	summary: "Copy same file twice",
	pkgdata: testutil.PackageData["run-one"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/usr/bin/run-one": []deb.ExtractInfo{{
				Path: "/usr/bin/run-one",
			}, {
				Path: "/usr/bin/run-two",
			}},
		},
	},
	result: map[string]string{
		"/usr/":                            "dir 0755",
		"/usr/bin/":                        "dir 0755",
		"/usr/bin/run-one":                 "file 0755 95a2a697",
		"/usr/bin/run-two":                 "file 0755 95a2a697",
	},
}}

func (s *S) TestExtract(c *C) {

	for _, test := range extractTests {
		c.Logf("Test: %s", test.summary)
		dir := c.MkDir()
		options := test.options
		options.Package = "run-one"
		options.TargetDir = dir

		err := deb.Extract(bytes.NewBuffer(test.pkgdata), &options)
		c.Assert(err, IsNil)

		result := make(map[string]string)
		dirfs := os.DirFS(dir)
		err = fs.WalkDir(dirfs, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("walk error: %w", err)
			}
			if path == "." {
				return nil
			}
			finfo, err := d.Info()
			if err != nil {
				return fmt.Errorf("cannot get stat info for %q: %w", path, err)
			}
			fperm := finfo.Mode() & fs.ModePerm
			ftype := finfo.Mode() & fs.ModeType
			fpath := filepath.Join(dir, path)
			switch ftype {
			case fs.ModeDir:
				result["/"+path+"/"] = fmt.Sprintf("dir %#o", fperm)
			case fs.ModeSymlink:
				lpath, err := os.Readlink(fpath)
				if err != nil {
					return err
				}
				result["/"+path] = fmt.Sprintf("symlink %s", lpath)
			case 0: // Regular
				data, err := ioutil.ReadFile(fpath)
				if err != nil {
					return fmt.Errorf("cannot read file: %w", err)
				}
				var entry string
				if len(data) == 0 {
					entry = fmt.Sprintf("file %#o empty", fperm)
				} else {
					sum := sha256.Sum256(data)
					entry = fmt.Sprintf("file %#o %.4x", fperm, sum)
				}
				result["/"+path] = entry
			default:
				return fmt.Errorf("unknown file type %d: %s", ftype, fpath)
			}
			return nil
		})
		c.Assert(err, IsNil)
		c.Assert(result, DeepEquals, test.result)
	}
}
