package deb_test

import (
	"bytes"
	"path/filepath"
	"sort"
	"strings"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/deb"
	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/testutil"
)

type extractTest struct {
	summary string
	pkgdata []byte
	options deb.ExtractOptions
	hackopt func(o *deb.ExtractOptions)
	globbed map[string][]string
	result  map[string]string
	// paths which the extractor created explicitly.
	createdPaths []string
	error        string
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
		"/tmp/":               "dir 01777",
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
	createdPaths: []string{
		"/etc",
		"/etc/os-release",
		"/tmp",
		"/usr",
		"/usr/bin",
		"/usr/bin/hello",
		"/usr/lib",
		"/usr/lib/os-release",
		"/usr/share",
		"/usr/share/doc",
	},
}, {
	summary: "Extract a few entries, nil Create closure",
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
		"/tmp/":               "dir 01777",
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
	hackopt: func(o *deb.ExtractOptions) {
		o.Create = nil
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
	createdPaths: []string{"/usr", "/usr/foo/bin/hello-2", "/usr/other"},
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
	createdPaths: []string{"/usr", "/usr/bin", "/usr/bin/hallo", "/usr/bin/hello"},
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
	createdPaths: []string{"/etc/default", "/etc/dpkg"},
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
	createdPaths: []string{
		"/etc/debian_version",
		"/etc/default",
		"/etc/dpkg",
		"/etc/dpkg/origins",
		"/etc/dpkg/origins/debian",
		"/etc/dpkg/origins/ubuntu",
	},
}, {
	summary: "Globbing with reporting of globbed paths",
	pkgdata: testutil.PackageData["base-files"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/etc/de**": []deb.ExtractInfo{{
				Path: "/etc/de**",
			}},
			"/etc/dp*/": []deb.ExtractInfo{{
				Path: "/etc/dp*/",
			}},
		},
	},
	result: map[string]string{
		"/etc/":               "dir 0755",
		"/etc/dpkg/":          "dir 0755",
		"/etc/default/":       "dir 0755",
		"/etc/debian_version": "file 0644 cce26cfe",
	},
	globbed: map[string][]string{
		"/etc/dp*/": []string{"/etc/dpkg/"},
		"/etc/de**": []string{"/etc/debian_version", "/etc/default/"},
	},
	createdPaths: []string{"/etc/debian_version", "/etc/default", "/etc/dpkg"},
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
}, {
	summary: "Missing file",
	pkgdata: testutil.PackageData["base-files"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/etc/passwd": []deb.ExtractInfo{{
				Path: "/etc/passwd",
			}},
		},
	},
	error: `cannot extract from package "base-files": no content at /etc/passwd`,
}, {
	summary: "Missing directory",
	pkgdata: testutil.PackageData["base-files"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/etd/": []deb.ExtractInfo{{
				Path: "/etd/",
			}},
		},
	},
	error: `cannot extract from package "base-files": no content at /etd/`,
}, {
	summary: "Missing glob",
	pkgdata: testutil.PackageData["base-files"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/etd/**": []deb.ExtractInfo{{
				Path: "/etd/**",
			}},
		},
	},
	error: `cannot extract from package "base-files": no content at /etd/\*\*`,
}, {
	summary: "Missing multiple entries",
	pkgdata: testutil.PackageData["base-files"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/etc/passwd": []deb.ExtractInfo{{
				Path: "/etc/passwd",
			}},
			"/etd/": []deb.ExtractInfo{{
				Path: "/etd/",
			}},
		},
	},
	error: `cannot extract from package "base-files": no content at:\n- /etc/passwd\n- /etd/`,
}, {
	summary: "Optional entries may be missing",
	pkgdata: testutil.PackageData["base-files"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/etc/": []deb.ExtractInfo{{
				Path: "/etc/",
			}},
			"/usr/foo/hallo": []deb.ExtractInfo{{
				Path:     "/usr/bin/foo/hallo",
				Optional: true,
			}},
			"/other/path/": []deb.ExtractInfo{{
				Path:     "/tmp/new/path/",
				Optional: true,
			}},
		},
	},
	result: map[string]string{
		"/etc/":     "dir 0755",
		"/usr/":     "dir 0755",
		"/usr/bin/": "dir 0755",
		"/tmp/":     "dir 01777",
	},
	createdPaths: []string{"/etc", "/tmp", "/usr", "/usr/bin"},
}, {
	summary: "Optional entries mixed in cannot be missing",
	pkgdata: testutil.PackageData["base-files"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/usr/bin/hallo": []deb.ExtractInfo{{
				Path:     "/usr/bin/hallo",
				Optional: true,
			}, {
				Path:     "/usr/bin/hallow",
				Optional: false,
			}},
		},
	},
	error: `cannot extract from package "base-files": no content at /usr/bin/hallo`,
}}

func (s *S) TestExtract(c *C) {

	for _, test := range extractTests {
		c.Logf("Test: %s", test.summary)
		dir := c.MkDir()
		options := test.options
		options.Package = "base-files"
		options.TargetDir = dir
		var createdPaths []string
		options.Create = func(o *fsutil.CreateOptions) error {
			relPath := filepath.Clean("/" + strings.TrimPrefix(o.Path, dir))
			createdPaths = append(createdPaths, relPath)
			_, err := fsutil.Create(o)
			return err
		}

		if test.hackopt != nil {
			test.hackopt(&options)
		}

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

		sort.Strings(createdPaths)
		sort.Strings(test.createdPaths)
		c.Assert(createdPaths, DeepEquals, test.createdPaths)

		result := testutil.TreeDump(dir)
		c.Assert(result, DeepEquals, test.result)
	}
}
