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
	result  map[string]string
	error   string
}

var extractTests = []extractTest{{
	summary: "Extract nothing",
	pkgdata: testutil.PackageData["base-files"],
	options: deb.ExtractOptions{
		Extract: nil,
	},
	result: map[string]string{},
}, {
	summary: "Extract a few entries",
	pkgdata: testutil.PackageData["base-files"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/usr/bin/hello": []deb.ExtractInfo{{
				Path: "/usr/bin/hello",
			}},
			"/etc/os-release": []deb.ExtractInfo{{
				Path: "/etc/os-release",
			}},
			"/usr/lib/os-release": []deb.ExtractInfo{{
				Path: "/usr/lib/os-release",
			}},
			"/usr/share/doc/": []deb.ExtractInfo{{
				Path: "/usr/share/doc/",
			}},
			"/tmp/": []deb.ExtractInfo{{
				Path: "/tmp/",
			}},
		},
	},
	result: map[string]string{
		"/tmp/":               "dir 01775",
		"/usr/":               "dir 0755",
		"/usr/bin/":           "dir 0755",
		"/usr/bin/hello":      "file 0775 eaf29575",
		"/usr/share/":         "dir 0755",
		"/usr/share/doc/":     "dir 0755",
		"/usr/lib/":           "dir 0755",
		"/usr/lib/os-release": "file 0644 ec6fae43",
		"/etc/":               "dir 0755",
		"/etc/os-release":     "symlink ../usr/lib/os-release",
	},
}, {
	summary: "Globbing a single dir level",
	pkgdata: testutil.PackageData["base-files"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/etc/d*/": []deb.ExtractInfo{{
				Path: "/etc/d*/",
			}},
		},
	},
	result: map[string]string{
		"/etc/":         "dir 0755",
		"/etc/dpkg/":    "dir 0755",
		"/etc/default/": "dir 0755",
	},
}, {

	summary: "Copy a couple of entries elsewhere",
	pkgdata: testutil.PackageData["base-files"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/usr/bin/hello": []deb.ExtractInfo{{
				Path: "/usr/foo/bin/hello-2",
				Mode: 0600,
			}},
			"/usr/share/": []deb.ExtractInfo{{
				Path: "/usr/other/",
				Mode: 0700,
			}},
		},
	},
	result: map[string]string{
		"/usr/":                "dir 0755",
		"/usr/foo/":            "dir 0755",
		"/usr/foo/bin/":        "dir 0755",
		"/usr/foo/bin/hello-2": "file 0600 eaf29575",
		"/usr/other/":          "dir 0700",
	},
}, {

	summary: "Copy same file twice",
	pkgdata: testutil.PackageData["base-files"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/usr/bin/hello": []deb.ExtractInfo{{
				Path: "/usr/bin/hello",
			}, {
				Path: "/usr/bin/hallo",
			}},
		},
	},
	result: map[string]string{
		"/usr/":          "dir 0755",
		"/usr/bin/":      "dir 0755",
		"/usr/bin/hello": "file 0775 eaf29575",
		"/usr/bin/hallo": "file 0775 eaf29575",
	},
}, {
	summary: "Globbing for files with multiple levels at once",
	pkgdata: testutil.PackageData["base-files"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/etc/d**": []deb.ExtractInfo{{
				Path: "/etc/d**",
			}},
		},
	},
	result: map[string]string{
		"/etc/":                    "dir 0755",
		"/etc/dpkg/":               "dir 0755",
		"/etc/dpkg/origins/":       "dir 0755",
		"/etc/dpkg/origins/debian": "file 0644 50f35af8",
		"/etc/dpkg/origins/ubuntu": "file 0644 d2537b95",
		"/etc/default/":            "dir 0755",
		"/etc/debian_version":      "file 0644 cce26cfe",
	},
}, {
	summary: "Globbing must have matching source and target",
	pkgdata: testutil.PackageData["base-files"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/etc/d**": []deb.ExtractInfo{{
				Path: "/etc/g**",
			}},
		},
	},
	error: `cannot extract .*: when using wildcards source and target paths must match: /etc/d\*\*`,
}, {
	summary: "Globbing must also have a single target",
	pkgdata: testutil.PackageData["base-files"],
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
	pkgdata: testutil.PackageData["base-files"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/etc/d**": []deb.ExtractInfo{{
				Path: "/etc/d**",
				Mode: 0777,
			}},
		},
	},
	error: `cannot extract .*: when using wildcards source and target paths must match: /etc/d\*\*`,
}}

func (s *S) TestExtract(c *C) {

	for _, test := range extractTests {
		c.Logf("Test: %s", test.summary)
		dir := c.MkDir()
		options := test.options
		options.Package = "base-files"
		options.TargetDir = dir

		err := deb.Extract(bytes.NewBuffer(test.pkgdata), &options)
		if test.error != "" {
			c.Assert(err, ErrorMatches, test.error)
			continue
		} else {
			c.Assert(err, IsNil)
		}

		result := testutil.TreeDump(dir)
		c.Assert(result, DeepEquals, test.result)
	}
}
