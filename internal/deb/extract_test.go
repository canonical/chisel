package deb_test

import (
	"bytes"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/deb"
	"github.com/canonical/chisel/internal/testutil"
)

var (
	Reg = testutil.Reg
	Dir = testutil.Dir
	Lnk = testutil.Lnk
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
		"/etc/": "dir 0755",
	},
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
}, {
	summary: "Implicit parent directories",
	pkgdata: testutil.MustMakeDeb([]testutil.TarEntry{
		Dir(0701, "./a/"),
		Dir(0702, "./a/b/"),
		Reg(0601, "./a/b/c", ""),
	}),
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/a/b/c": []deb.ExtractInfo{{Path: "/a/b/c"}},
		},
	},
	result: map[string]string{
		"/a/":    "dir 0701",
		"/a/b/":  "dir 0702",
		"/a/b/c": "file 0601 empty",
	},
}, {
	summary: "Implicit parent directories with different target path",
	pkgdata: testutil.MustMakeDeb([]testutil.TarEntry{
		Dir(0701, "./a/"),
		Dir(0702, "./b/"),
		Reg(0601, "./b/x", "shark"),
		Dir(0703, "./c/"),
		Reg(0602, "./c/y", "octopus"),
		Dir(0704, "./d/"),
	}),
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/b/x": []deb.ExtractInfo{{Path: "/a/x"}},
			"/c/y": []deb.ExtractInfo{{Path: "/d/y"}},
		},
	},
	result: map[string]string{
		"/a/":  "dir 0701",
		"/a/x": "file 0601 31fc1594",
		"/d/":  "dir 0704",
		"/d/y": "file 0602 5633c9b8",
	},
}, {
	summary: "Implicit parent directories with a glob",
	pkgdata: testutil.MustMakeDeb([]testutil.TarEntry{
		Dir(0701, "./a/"),
		Dir(0702, "./a/aa/"),
		Dir(0703, "./a/aa/aaa/"),
		Reg(0601, "./a/aa/aaa/ffff", ""),
	}),
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/a/aa/a**": []deb.ExtractInfo{{
				Path: "/a/aa/a**",
			}},
		},
	},
	result: map[string]string{
		"/a/":            "dir 0701",
		"/a/aa/":         "dir 0702",
		"/a/aa/aaa/":     "dir 0703",
		"/a/aa/aaa/ffff": "file 0601 empty",
	},
}, {
	summary: "Implicit parent directories with a glob and non-sorted tarball",
	pkgdata: testutil.MustMakeDeb([]testutil.TarEntry{
		Reg(0601, "./a/b/c/d", ""),
		Dir(0702, "./a/b/"),
		Dir(0703, "./a/b/c/"),
		Dir(0701, "./a/"),
	}),
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/a/b/c/*": []deb.ExtractInfo{{
				Path: "/a/b/c/*",
			}},
		},
	},
	result: map[string]string{
		"/a/":      "dir 0701",
		"/a/b/":    "dir 0702",
		"/a/b/c/":  "dir 0703",
		"/a/b/c/d": "file 0601 empty",
	},
}, {
	summary: "Implicit parent directories with a glob and some parents missing in the tarball",
	pkgdata: testutil.MustMakeDeb([]testutil.TarEntry{
		Reg(0601, "./a/b/c/d", ""),
		Dir(0702, "./a/b/"),
	}),
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/a/b/c/*": []deb.ExtractInfo{{
				Path: "/a/b/c/*",
			}},
		},
	},
	result: map[string]string{
		"/a/":      "dir 0755",
		"/a/b/":    "dir 0702",
		"/a/b/c/":  "dir 0755",
		"/a/b/c/d": "file 0601 empty",
	},
}, {
	summary: "Implicit parent directories with copied dirs and different modes",
	pkgdata: testutil.MustMakeDeb([]testutil.TarEntry{
		Dir(0701, "./a/"),
		Dir(0702, "./a/b/"),
		Dir(0703, "./a/b/c/"),
		Reg(0601, "./a/b/c/d", ""),
		Dir(0704, "./e/"),
		Dir(0705, "./e/f/"),
	}),
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/a/b/**": []deb.ExtractInfo{{
				Path: "/a/b/**",
			}},
			"/e/f/": []deb.ExtractInfo{{
				Path: "/a/",
			}},
			"/e/": []deb.ExtractInfo{{
				Path: "/a/b/c/",
				Mode: 0706,
			}},
		},
	},
	result: map[string]string{
		"/a/":      "dir 0705",
		"/a/b/":    "dir 0702",
		"/a/b/c/":  "dir 0706",
		"/a/b/c/d": "file 0601 empty",
	},
}, {
	summary: "Copies with different permissions",
	pkgdata: testutil.MustMakeDeb([]testutil.TarEntry{
		Dir(0701, "./a/"),
		Reg(0601, "./b", ""),
	}),
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/a/": []deb.ExtractInfo{
				{Path: "/b/"},
				{Path: "/c/", Mode: 0702},
				{Path: "/d/", Mode: 01777},
				{Path: "/e/"},
				{Path: "/f/", Mode: 0723},
				{Path: "/g/"},
			},
		},
	},
	result: map[string]string{
		"/b/": "dir 0701",
		"/c/": "dir 0702",
		"/d/": "dir 01777",
		"/e/": "dir 0701",
		"/f/": "dir 0723",
		"/g/": "dir 0701",
	},
}}

func (s *S) TestExtract(c *C) {

	for _, test := range extractTests {
		c.Logf("Test: %s", test.summary)
		dir := c.MkDir()
		options := test.options
		options.Package = "base-files"
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
