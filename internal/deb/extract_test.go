package deb_test

import (
	"bytes"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/deb"
	"github.com/canonical/chisel/internal/testutil"
)

type extractTest struct {
	summary string
	pkgdata []byte
	options deb.ExtractOptions
	globbed map[string][]string
	result  map[string]string
	error   string
}

var extractTests = []extractTest{{
	summary: "Extract nothing",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: nil,
	},
	result: map[string]string{},
}, {
	summary: "Extract a few entries",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/dir/file": []deb.ExtractInfo{{
				Path: "/dir/file",
			}},
			"/dir/other_file": []deb.ExtractInfo{{
				Path: "/dir/other_file",
			}},
			"/dir/several/levels/deep/file": []deb.ExtractInfo{{
				Path: "/dir/several/levels/deep/file",
			}},
			"/dir/nested/": []deb.ExtractInfo{{
				Path: "/dir/nested/",
			}},
			"/other_dir/": []deb.ExtractInfo{{
				Path: "/other_dir/",
			}},
		},
	},
	result: map[string]string{
		"/dir/":                         "dir 0755",
		"/dir/file":                     "file 0644 cc55e2ec",
		"/dir/nested/":                  "dir 0755",
		"/dir/other_file":               "file 0644 63d5dd49",
		"/dir/several/":                 "dir 0755",
		"/dir/several/levels/":          "dir 0755",
		"/dir/several/levels/deep/":     "dir 0755",
		"/dir/several/levels/deep/file": "file 0644 6bc26dff",
		"/other_dir/":                   "dir 0755",
	},
}, {

	summary: "Copy a couple of entries elsewhere",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/dir/file": []deb.ExtractInfo{{
				Path: "/foo/file_copy",
				Mode: 0600,
			}},
			"/dir/several/levels/deep/": []deb.ExtractInfo{{
				Path: "/foo/bar/dir_copy",
				Mode: 0700,
			}},
		},
	},
	result: map[string]string{
		"/foo/":              "dir 0755",
		"/foo/bar/":          "dir 0755",
		"/foo/bar/dir_copy/": "dir 0700",
		"/foo/file_copy":     "file 0600 cc55e2ec",
	},
}, {

	summary: "Copy same file twice",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/dir/file": []deb.ExtractInfo{{
				Path: "/dir/foo/file_copy_1",
			}, {
				Path: "/dir/bar/file_copy_2",
			}},
		},
	},
	result: map[string]string{
		"/dir/":                "dir 0755",
		"/dir/bar/":            "dir 0755",
		"/dir/bar/file_copy_2": "file 0644 cc55e2ec",
		"/dir/foo/":            "dir 0755",
		"/dir/foo/file_copy_1": "file 0644 cc55e2ec",
	},
}, {
	summary: "Globbing a single dir level",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/*/": []deb.ExtractInfo{{
				Path: "/*/",
			}},
		},
	},
	result: map[string]string{
		"/dir/":       "dir 0755",
		"/other_dir/": "dir 0755",
		"/parent/":    "dir 01777",
		"/usr/":       "dir 0755",
	},
}, {
	summary: "Globbing for files with multiple levels at once",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/d**": []deb.ExtractInfo{{
				Path: "/d**",
			}},
		},
	},
	result: map[string]string{
		"/dir/":                         "dir 0755",
		"/dir/file":                     "file 0644 cc55e2ec",
		"/dir/nested/":                  "dir 0755",
		"/dir/nested/file":              "file 0644 84237a05",
		"/dir/nested/other_file":        "file 0644 6b86b273",
		"/dir/other_file":               "file 0644 63d5dd49",
		"/dir/several/":                 "dir 0755",
		"/dir/several/levels/":          "dir 0755",
		"/dir/several/levels/deep/":     "dir 0755",
		"/dir/several/levels/deep/file": "file 0644 6bc26dff",
	},
}, {
	summary: "Globbing with reporting of globbed paths",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/dir/s**": []deb.ExtractInfo{{
				Path: "/dir/s**",
			}},
			"/dir/n*/": []deb.ExtractInfo{{
				Path: "/dir/n*/",
			}},
		},
	},
	result: map[string]string{
		"/dir/":                         "dir 0755",
		"/dir/nested/":                  "dir 0755",
		"/dir/several/":                 "dir 0755",
		"/dir/several/levels/":          "dir 0755",
		"/dir/several/levels/deep/":     "dir 0755",
		"/dir/several/levels/deep/file": "file 0644 6bc26dff",
	},
	globbed: map[string][]string{
		"/dir/n*/": []string{"/dir/nested/"},
		"/dir/s**": []string{"/dir/several/levels/deep/", "/dir/several/levels/deep/file"},
	},
}, {
	summary: "Globbing must have matching source and target",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/foo/b**": []deb.ExtractInfo{{
				Path: "/foo/g**",
			}},
		},
	},
	error: `cannot extract .*: when using wildcards source and target paths must match: /foo/b\*\*`,
}, {
	summary: "Globbing must also have a single target",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/foo/b**": []deb.ExtractInfo{{
				Path: "/foo/b**",
			}, {
				Path: "/foo/g**",
			}},
		},
	},
	error: `cannot extract .*: when using wildcards source and target paths must match: /foo/b\*\*`,
}, {
	summary: "Globbing cannot change modes",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/dir/n**": []deb.ExtractInfo{{
				Path: "/dir/n**",
				Mode: 0777,
			}},
		},
	},
	error: `cannot extract .*: when using wildcards source and target paths must match: /dir/n\*\*`,
}, {
	summary: "Missing file",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/missing_file": []deb.ExtractInfo{{
				Path: "/missing_file",
			}},
		},
	},
	error: `cannot extract from package "test-package": no content at /missing_file`,
}, {
	summary: "Missing directory",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/missing_dir/": []deb.ExtractInfo{{
				Path: "/missing_dir/",
			}},
		},
	},
	error: `cannot extract from package "test-package": no content at /missing_dir/`,
}, {
	summary: "Missing glob",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/missing_dir/**": []deb.ExtractInfo{{
				Path: "/missing_dir/**",
			}},
		},
	},
	error: `cannot extract from package "test-package": no content at /missing_dir/\*\*`,
}, {
	summary: "Missing multiple entries",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/missing_file": []deb.ExtractInfo{{
				Path: "missing_file",
			}},
			"/missing_dir/": []deb.ExtractInfo{{
				Path: "/missing_dir/",
			}},
		},
	},
	error: `cannot extract from package "test-package": no content at:\n- /missing_dir/\n- /missing_file`,
}, {
	summary: "Optional entries may be missing",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/dir/": []deb.ExtractInfo{{
				Path: "/dir/",
			}},
			"/dir/optional": []deb.ExtractInfo{{
				Path:     "/other_dir/foo",
				Optional: true,
			}},
			"/optional_dir/": []deb.ExtractInfo{{
				Path:     "/foo/optional_dir/",
				Optional: true,
			}},
		},
	},
	result: map[string]string{
		"/dir/":       "dir 0755",
		"/other_dir/": "dir 0755",
	},
}, {
	summary: "Optional entries mixed in cannot be missing",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/dir/missing_file": []deb.ExtractInfo{{
				Path:     "/dir/optional",
				Optional: true,
			}, {
				Path:     "/dir/not_optional",
				Optional: false,
			}},
		},
	},
	error: `cannot extract from package "test-package": no content at /dir/missing_file`,
}}

func (s *S) TestExtract(c *C) {

	for _, test := range extractTests {
		c.Logf("Test: %s", test.summary)
		dir := c.MkDir()
		options := test.options
		options.Package = "test-package"
		options.TargetDir = dir

		if test.globbed != nil {
			options.Globbed = make(map[string][]string)
		}

		err := deb.Extract(bytes.NewBuffer(test.pkgdata), &options)
		if test.error != "" {
			c.Assert(err, ErrorMatches, test.error)
			continue
		} else {
			c.Assert(err, IsNil)
		}

		if test.globbed != nil {
			c.Assert(options.Globbed, DeepEquals, test.globbed)
		}

		result := testutil.TreeDump(dir)
		c.Assert(result, DeepEquals, test.result)
	}
}
