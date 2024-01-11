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
			"/a1/f1": []deb.ExtractInfo{{
				Path: "/a1/f1",
			}},
			"/a1/f2": []deb.ExtractInfo{{
				Path: "/a1/f2",
			}},
			"/a1/b1/f1": []deb.ExtractInfo{{
				Path: "/a1/b1/f1",
			}},
			"/a1/b1/c1/f1": []deb.ExtractInfo{{
				Path: "/a1/b1/c1/f1",
			}},
		},
	},
	result: map[string]string{
		"/a1/":         "dir 0755",
		"/a1/f1":       "file 0644 dfa6f45e",
		"/a1/f2":       "file 0644 908add3a",
		"/a1/b1/":      "dir 0755",
		"/a1/b1/f1":    "file 0644 513b26ef",
		"/a1/b1/c1/":   "dir 0755",
		"/a1/b1/c1/f1": "file 0644 440a3c5f",
	},
}, {

	summary: "Copy a couple of entries elsewhere",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/a1/f1": []deb.ExtractInfo{{
				Path: "/a1/b1/other_file",
				Mode: 0600,
			}},
			"/a1/": []deb.ExtractInfo{{
				Path: "/other_dir/",
				Mode: 0700,
			}},
		},
	},
	result: map[string]string{
		"/a1/":              "dir 0755",
		"/a1/b1/":           "dir 0755",
		"/a1/b1/other_file": "file 0600 dfa6f45e",
		"/other_dir/":       "dir 0700",
	},
}, {

	summary: "Copy same file twice",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/a1/f2": []deb.ExtractInfo{{
				Path: "/tmp/one_copy",
			}, {
				Path: "/tmp/two_copy",
			}},
		},
	},
	result: map[string]string{
		"/tmp/":         "dir 0755",
		"/tmp/one_copy": "file 0644 908add3a",
		"/tmp/two_copy": "file 0644 908add3a",
	},
}, {
	summary: "Globbing a single dir level",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/a1/b*/": []deb.ExtractInfo{{
				Path: "/a1/b*/",
			}},
		},
	},
	result: map[string]string{
		"/a1/":    "dir 0755",
		"/a1/b1/": "dir 0755",
	},
}, {
	summary: "Globbing for files with multiple levels at once",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/a1/b**": []deb.ExtractInfo{{
				Path: "/a1/b**",
			}},
		},
	},
	result: map[string]string{
		"/a1/":         "dir 0755",
		"/a1/b1/":      "dir 0755",
		"/a1/b1/c1/":   "dir 0755",
		"/a1/b1/f1":    "file 0644 513b26ef",
		"/a1/b1/f2":    "file 0644 55ce7f13",
		"/a1/b1/c1/f1": "file 0644 440a3c5f",
	},
}, {
	summary: "Globbing with reporting of globbed paths",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/a1/b**": []deb.ExtractInfo{{
				Path: "/a1/b**",
			}},
			"/a2/b*/": []deb.ExtractInfo{{
				Path: "/a2/b*/",
			}},
		},
	},
	result: map[string]string{
		"/a1/":         "dir 0755",
		"/a1/b1/":      "dir 0755",
		"/a1/b1/c1/":   "dir 0755",
		"/a1/b1/f1":    "file 0644 513b26ef",
		"/a1/b1/f2":    "file 0644 55ce7f13",
		"/a1/b1/c1/f1": "file 0644 440a3c5f",
		"/a2/":         "dir 0755",
		"/a2/b1/":      "dir 0755",
	},
	globbed: map[string][]string{
		"/a1/b**": []string{"/a1/b1/", "/a1/b1/f1", "/a1/b1/f2", "/a1/b1/c1/", "/a1/b1/c1/f1"},
		"/a2/b*/": []string{"/a2/b1/"},
	},
}, {
	summary: "Globbing must have matching source and target",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/d1/d**": []deb.ExtractInfo{{
				Path: "/d1/g**",
			}},
		},
	},
	error: `cannot extract .*: when using wildcards source and target paths must match: /d1/d\*\*`,
}, {
	summary: "Globbing must also have a single target",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/etc/d**": []deb.ExtractInfo{{
				Path: "/etc/d**",
			}, {
				Path: "/etc/d**",
			}},
		},
	},
	error: `cannot extract .*: when using wildcards source and target paths must match: /etc/d\*\*`,
}, {
	summary: "Globbing cannot change modes",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/d1/d**": []deb.ExtractInfo{{
				Path: "/d1/d**",
				Mode: 0777,
			}},
		},
	},
	error: `cannot extract .*: when using wildcards source and target paths must match: /etc/d\*\*`,
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
			"/a1/": []deb.ExtractInfo{{
				Path: "/a1/",
			}},
			"/a2/optional": []deb.ExtractInfo{{
				Path:     "/a2/optional",
				Optional: true,
			}},
			"/other_optional/": []deb.ExtractInfo{{
				Path:     "/other_optional",
				Optional: true,
			}},
		},
	},
	result: map[string]string{
		"/a1/": "dir 0755",
		"/a2/": "dir 0755",
	},
}, {
	summary: "Optional entries mixed in cannot be missing",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/a1/missing_file": []deb.ExtractInfo{{
				Path:     "/a1/optional",
				Optional: true,
			}, {
				Path:     "/a1/not_optional",
				Optional: false,
			}},
		},
	},
	error: `cannot extract from package "test-package": no content at /a1/missing_file`,
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
