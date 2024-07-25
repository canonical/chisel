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
	result  map[string]string
	// paths which the extractor did not create explicitly.
	notCreated []string
	error      string
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
			"/dir/other-file": []deb.ExtractInfo{{
				Path: "/dir/other-file",
			}},
			"/dir/several/levels/deep/file": []deb.ExtractInfo{{
				Path: "/dir/several/levels/deep/file",
			}},
			"/dir/nested/": []deb.ExtractInfo{{
				Path: "/dir/nested/",
			}},
			"/other-dir/": []deb.ExtractInfo{{
				Path: "/other-dir/",
			}},
		},
	},
	result: map[string]string{
		"/dir/":                         "dir 0755",
		"/dir/file":                     "file 0644 cc55e2ec",
		"/dir/nested/":                  "dir 0755",
		"/dir/other-file":               "file 0644 63d5dd49",
		"/dir/several/":                 "dir 0755",
		"/dir/several/levels/":          "dir 0755",
		"/dir/several/levels/deep/":     "dir 0755",
		"/dir/several/levels/deep/file": "file 0644 6bc26dff",
		"/other-dir/":                   "dir 0755",
	},
	notCreated: []string{},
}, {
	summary: "Extract a few entries, nil Create closure",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/dir/file": []deb.ExtractInfo{{
				Path: "/dir/file",
			}},
			"/dir/other-file": []deb.ExtractInfo{{
				Path: "/dir/other-file",
			}},
			"/dir/several/levels/deep/file": []deb.ExtractInfo{{
				Path: "/dir/several/levels/deep/file",
			}},
			"/dir/nested/": []deb.ExtractInfo{{
				Path: "/dir/nested/",
			}},
			"/other-dir/": []deb.ExtractInfo{{
				Path: "/other-dir/",
			}},
		},
	},
	result: map[string]string{
		"/dir/":                         "dir 0755",
		"/dir/file":                     "file 0644 cc55e2ec",
		"/dir/nested/":                  "dir 0755",
		"/dir/other-file":               "file 0644 63d5dd49",
		"/dir/several/":                 "dir 0755",
		"/dir/several/levels/":          "dir 0755",
		"/dir/several/levels/deep/":     "dir 0755",
		"/dir/several/levels/deep/file": "file 0644 6bc26dff",
		"/other-dir/":                   "dir 0755",
	},
	hackopt: func(o *deb.ExtractOptions) {
		o.Create = nil
	},
}, {
	summary: "Copy a couple of entries elsewhere",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/dir/file": []deb.ExtractInfo{{
				Path: "/foo/file-copy",
				Mode: 0600,
			}},
			"/dir/several/levels/deep/": []deb.ExtractInfo{{
				Path: "/foo/bar/dir-copy",
				Mode: 0700,
			}},
		},
	},
	result: map[string]string{
		"/foo/":              "dir 0755",
		"/foo/bar/":          "dir 0755",
		"/foo/bar/dir-copy/": "dir 0700",
		"/foo/file-copy":     "file 0600 cc55e2ec",
	},
	notCreated: []string{"/foo/", "/foo/bar/"},
}, {
	summary: "Copy same file twice",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/dir/file": []deb.ExtractInfo{{
				Path: "/dir/foo/file-copy-1",
			}, {
				Path: "/dir/bar/file-copy-2",
			}},
		},
	},
	result: map[string]string{
		"/dir/":                "dir 0755",
		"/dir/bar/":            "dir 0755",
		"/dir/bar/file-copy-2": "file 0644 cc55e2ec",
		"/dir/foo/":            "dir 0755",
		"/dir/foo/file-copy-1": "file 0644 cc55e2ec",
	},
	notCreated: []string{"/dir/bar/", "/dir/foo/"},
}, {
	summary: "Globbing a single dir level",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/dir/s*/": []deb.ExtractInfo{{
				Path: "/dir/s*/",
			}},
		},
	},
	result: map[string]string{
		"/dir/":         "dir 0755",
		"/dir/several/": "dir 0755",
	},
	notCreated: []string{},
}, {
	summary: "Globbing for files with multiple levels at once",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/dir/s**": []deb.ExtractInfo{{
				Path: "/dir/s**",
			}},
		},
	},
	result: map[string]string{
		"/dir/":                         "dir 0755",
		"/dir/several/":                 "dir 0755",
		"/dir/several/levels/":          "dir 0755",
		"/dir/several/levels/deep/":     "dir 0755",
		"/dir/several/levels/deep/file": "file 0644 6bc26dff",
	},
	notCreated: []string{},
}, {
	summary: "Globbing multiple paths",
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
	notCreated: []string{},
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
	error: `cannot extract from package "test-package": when using wildcards source and target paths must match: /foo/b\*\*`,
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
	error: `cannot extract from package "test-package": when using wildcards source and target paths must match: /foo/b\*\*`,
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
	error: `cannot extract from package "test-package": when using wildcards source and target paths must match: /dir/n\*\*`,
}, {
	summary: "Missing file",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/missing-file": []deb.ExtractInfo{{
				Path: "/missing-file",
			}},
		},
	},
	error: `cannot extract from package "test-package": no content at /missing-file`,
}, {
	summary: "Missing directory",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/missing-dir/": []deb.ExtractInfo{{
				Path: "/missing-dir/",
			}},
		},
	},
	error: `cannot extract from package "test-package": no content at /missing-dir/`,
}, {
	summary: "Missing glob",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/missing-dir/**": []deb.ExtractInfo{{
				Path: "/missing-dir/**",
			}},
		},
	},
	error: `cannot extract from package "test-package": no content at /missing-dir/\*\*`,
}, {
	summary: "Missing multiple entries",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/missing-file": []deb.ExtractInfo{{
				Path: "missing-file",
			}},
			"/missing-dir/": []deb.ExtractInfo{{
				Path: "/missing-dir/",
			}},
		},
	},
	error: `cannot extract from package "test-package": no content at:\n- /missing-dir/\n- /missing-file`,
}, {
	summary: "Optional entries may be missing",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/dir/": []deb.ExtractInfo{{
				Path: "/dir/",
			}},
			"/dir/optional": []deb.ExtractInfo{{
				Path:     "/other-dir/foo",
				Optional: true,
			}},
			"/optional-dir/": []deb.ExtractInfo{{
				Path:     "/foo/optional-dir/",
				Optional: true,
			}},
		},
	},
	result: map[string]string{
		"/dir/": "dir 0755",
	},
	notCreated: []string{},
}, {
	summary: "Optional entries mixed in cannot be missing",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/dir/missing-file": []deb.ExtractInfo{{
				Path:     "/dir/optional",
				Optional: true,
			}, {
				Path:     "/dir/not-optional",
				Optional: false,
			}},
		},
	},
	error: `cannot extract from package "test-package": no content at /dir/missing-file`,
}, {
	summary: "Extract non-ASCII path and preserve parent directories permissions",
	pkgdata: testutil.MustMakeDeb([]testutil.TarEntry{
		testutil.Dir(0755, "./"),
		testutil.Dir(0766, "./日本/"),
		testutil.Reg(0644, "./日本/語", "whatever"),
	}),
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/日本/語": []deb.ExtractInfo{{
				Path: "/日本/語",
			}},
		},
	},
	result: map[string]string{
		"/日本/":  "dir 0766",
		"/日本/語": "file 0644 85738f8f",
	},
	notCreated: []string{},
}, {
	summary: "Entries for same destination must have the same mode",
	pkgdata: testutil.PackageData["test-package"],
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/dir/": []deb.ExtractInfo{{
				Path: "/dir/",
				Mode: 0777,
			}},
			"/d**": []deb.ExtractInfo{{
				Path: "/d**",
			}},
		},
	},
	error: `cannot extract from package "test-package": path /dir/ requested twice with diverging mode: 0777 != 0000`,
}}

func (s *S) TestExtract(c *C) {

	for _, test := range extractTests {
		c.Logf("Test: %s", test.summary)
		dir := c.MkDir()
		options := test.options
		options.Package = "test-package"
		options.TargetDir = dir
		createdPaths := make(map[string]bool)
		options.Create = func(_ []deb.ExtractInfo, o *fsutil.CreateOptions) error {
			relPath := filepath.Clean("/" + strings.TrimPrefix(o.Path, dir))
			if o.Mode.IsDir() {
				relPath = relPath + "/"
			}
			createdPaths[relPath] = true
			_, err := fsutil.Create(o)
			return err
		}

		if test.hackopt != nil {
			test.hackopt(&options)
		}

		err := deb.Extract(bytes.NewBuffer(test.pkgdata), &options)
		if test.error != "" {
			c.Assert(err, ErrorMatches, test.error)
			continue
		} else {
			c.Assert(err, IsNil)
		}

		if test.notCreated != nil {
			notCreated := []string{}
			for path := range test.result {
				if !createdPaths[path] {
					notCreated = append(notCreated, path)
				}
			}
			sort.Strings(notCreated)
			sort.Strings(test.notCreated)
			c.Assert(notCreated, DeepEquals, test.notCreated)
		}

		result := testutil.TreeDump(dir)
		c.Assert(result, DeepEquals, test.result)
	}
}

var extractCreateCallbackTests = []struct {
	summary string
	pkgdata []byte
	options deb.ExtractOptions
	calls   map[string][]deb.ExtractInfo
}{{
	summary: "Create is called with the set of ExtractInfo(s) that match the file",
	pkgdata: testutil.MustMakeDeb([]testutil.TarEntry{
		testutil.Dir(0755, "./"),
		testutil.Dir(0766, "./dir/"),
		testutil.Reg(0644, "./dir/file", "whatever"),
	}),
	options: deb.ExtractOptions{
		Extract: map[string][]deb.ExtractInfo{
			"/dir/": []deb.ExtractInfo{{
				Path: "/dir/",
			}},
			"/d**": []deb.ExtractInfo{{
				Path: "/d**",
			}},
			"/d?r/": []deb.ExtractInfo{{
				Path: "/d?r/",
			}},
			"/dir/file": []deb.ExtractInfo{{
				Path: "/dir/file",
			}, {
				Path: "/dir/file-cpy",
			}},
			"/foo/": []deb.ExtractInfo{{
				Path:     "/foo/",
				Optional: true,
			}},
		},
	},
	calls: map[string][]deb.ExtractInfo{
		"/dir/": []deb.ExtractInfo{
			deb.ExtractInfo{
				Path: "/d**",
			},
			deb.ExtractInfo{
				Path: "/d?r/",
			},
			deb.ExtractInfo{
				Path: "/dir/",
			},
		},
		"/dir/file": []deb.ExtractInfo{
			deb.ExtractInfo{
				Path: "/d**",
			},
			deb.ExtractInfo{
				Path: "/dir/file",
			},
		},
		"/dir/file-cpy": []deb.ExtractInfo{
			deb.ExtractInfo{
				Path: "/dir/file-cpy",
			},
		},
	},
}}

func (s *S) TestExtractCreateCallback(c *C) {
	for _, test := range extractCreateCallbackTests {
		c.Logf("Test: %s", test.summary)
		dir := c.MkDir()
		options := test.options
		options.Package = "test-package"
		options.TargetDir = dir
		createExtractInfos := map[string][]deb.ExtractInfo{}
		options.Create = func(extractInfos []deb.ExtractInfo, o *fsutil.CreateOptions) error {
			if extractInfos == nil {
				// Creating implicit parent directories, we don't care about those.
				return nil
			}
			relPath := filepath.Clean("/" + strings.TrimPrefix(o.Path, dir))
			if o.Mode.IsDir() {
				relPath = relPath + "/"
			}
			sort.Slice(extractInfos, func(i, j int) bool {
				return extractInfos[i].Path < extractInfos[j].Path
			})
			createExtractInfos[relPath] = extractInfos
			return nil
		}

		err := deb.Extract(bytes.NewBuffer(test.pkgdata), &options)
		c.Assert(err, IsNil)

		c.Assert(createExtractInfos, DeepEquals, test.calls)
	}
}
