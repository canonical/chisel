package tarball_test

import (
	"archive/tar"
	"bytes"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/tarball"
	"github.com/canonical/chisel/internal/testutil"
)

type extractTest struct {
	summary string
	pkgdata []byte
	options tarball.ExtractOptions
	hackopt func(c *C, o *tarball.ExtractOptions)
	result  map[string]string
	// paths which the extractor did not create explicitly.
	notCreated []string
	error      string
}

var extractTests = []extractTest{{
	summary: "Extract nothing",
	pkgdata: testutil.PackageData["test-package"],
	options: tarball.ExtractOptions{
		Extract: nil,
	},
	result: map[string]string{},
}, {
	summary: "Extract a few entries",
	pkgdata: testutil.PackageData["test-package"],
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/dir/file": []tarball.ExtractInfo{{
				Path: "/dir/file",
			}},
			"/dir/other-file": []tarball.ExtractInfo{{
				Path: "/dir/other-file",
			}},
			"/dir/several/levels/deep/file": []tarball.ExtractInfo{{
				Path: "/dir/several/levels/deep/file",
			}},
			"/dir/nested/": []tarball.ExtractInfo{{
				Path: "/dir/nested/",
			}},
			"/other-dir/": []tarball.ExtractInfo{{
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
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/dir/file": []tarball.ExtractInfo{{
				Path: "/dir/file",
			}},
			"/dir/other-file": []tarball.ExtractInfo{{
				Path: "/dir/other-file",
			}},
			"/dir/several/levels/deep/file": []tarball.ExtractInfo{{
				Path: "/dir/several/levels/deep/file",
			}},
			"/dir/nested/": []tarball.ExtractInfo{{
				Path: "/dir/nested/",
			}},
			"/other-dir/": []tarball.ExtractInfo{{
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
	hackopt: func(c *C, o *tarball.ExtractOptions) {
		o.Create = nil
	},
}, {
	summary: "Copy a couple of entries elsewhere",
	pkgdata: testutil.PackageData["test-package"],
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/dir/file": []tarball.ExtractInfo{{
				Path: "/foo/file-copy",
				Mode: 0600,
			}},
			"/dir/several/levels/deep/": []tarball.ExtractInfo{{
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
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/dir/file": []tarball.ExtractInfo{{
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
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/dir/s*/": []tarball.ExtractInfo{{
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
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/dir/s**": []tarball.ExtractInfo{{
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
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/dir/s**": []tarball.ExtractInfo{{
				Path: "/dir/s**",
			}},
			"/dir/n*/": []tarball.ExtractInfo{{
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
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/foo/b**": []tarball.ExtractInfo{{
				Path: "/foo/g**",
			}},
		},
	},
	error: `cannot extract from package "test-package": when using wildcards source and target paths must match: /foo/b\*\*`,
}, {
	summary: "Globbing must also have a single target",
	pkgdata: testutil.PackageData["test-package"],
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/foo/b**": []tarball.ExtractInfo{{
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
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/dir/n**": []tarball.ExtractInfo{{
				Path: "/dir/n**",
				Mode: 0777,
			}},
		},
	},
	error: `cannot extract from package "test-package": when using wildcards source and target paths must match: /dir/n\*\*`,
}, {
	summary: "Missing file",
	pkgdata: testutil.PackageData["test-package"],
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/missing-file": []tarball.ExtractInfo{{
				Path: "/missing-file",
			}},
		},
	},
	error: `cannot extract from package "test-package": no content at /missing-file`,
}, {
	summary: "Missing directory",
	pkgdata: testutil.PackageData["test-package"],
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/missing-dir/": []tarball.ExtractInfo{{
				Path: "/missing-dir/",
			}},
		},
	},
	error: `cannot extract from package "test-package": no content at /missing-dir/`,
}, {
	summary: "Missing glob",
	pkgdata: testutil.PackageData["test-package"],
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/missing-dir/**": []tarball.ExtractInfo{{
				Path: "/missing-dir/**",
			}},
		},
	},
	error: `cannot extract from package "test-package": no content at /missing-dir/\*\*`,
}, {
	summary: "Missing multiple entries",
	pkgdata: testutil.PackageData["test-package"],
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/missing-file": []tarball.ExtractInfo{{
				Path: "missing-file",
			}},
			"/missing-dir/": []tarball.ExtractInfo{{
				Path: "/missing-dir/",
			}},
		},
	},
	error: `cannot extract from package "test-package": no content at:\n- /missing-dir/\n- /missing-file`,
}, {
	summary: "Optional entries may be missing",
	pkgdata: testutil.PackageData["test-package"],
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/dir/": []tarball.ExtractInfo{{
				Path: "/dir/",
			}},
			"/dir/optional": []tarball.ExtractInfo{{
				Path:     "/other-dir/foo",
				Optional: true,
			}},
			"/optional-dir/": []tarball.ExtractInfo{{
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
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/dir/missing-file": []tarball.ExtractInfo{{
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
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/日本/語": []tarball.ExtractInfo{{
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
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/dir/": []tarball.ExtractInfo{{
				Path: "/dir/",
				Mode: 0777,
			}},
			"/d**": []tarball.ExtractInfo{{
				Path: "/d**",
			}},
		},
	},
	error: `cannot extract from package "test-package": path /dir/ requested twice with diverging mode: 0777 != 0000`,
}, {
	summary: "Single hard link entry can be extracted with the content entry",
	pkgdata: testutil.MustMakeDeb([]testutil.TarEntry{
		testutil.Dir(0755, "./"),
		testutil.Reg(0644, "./file", "text for file"),
		testutil.Hrd(0644, "./hardlink", "./file"),
	}),
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/**": []tarball.ExtractInfo{{
				Path: "/**",
			}},
		},
	},
	result: map[string]string{
		"/file":     "file 0644 28121945 <1>",
		"/hardlink": "file 0644 28121945 <1>",
	},
	notCreated: []string{},
}, {
	summary: "Single hard link entry can be extracted without the content entry",
	pkgdata: testutil.MustMakeDeb([]testutil.TarEntry{
		testutil.Dir(0755, "./"),
		testutil.Reg(0644, "./file", "text for file"),
		testutil.Hrd(0644, "./hardlink", "./file"),
	}),
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/hardlink": []tarball.ExtractInfo{{
				Path: "/hardlink",
			}},
		},
	},
	result: map[string]string{
		"/hardlink": "file 0644 28121945",
	},
	notCreated: []string{},
}, {
	summary: "Dangling hard link",
	pkgdata: testutil.MustMakeDeb([]testutil.TarEntry{
		testutil.Dir(0755, "./"),
		testutil.Hrd(0644, "./hardlink", "./non-existing-target"),
	}),
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/hardlink": []tarball.ExtractInfo{{
				Path: "/hardlink",
			}},
		},
	},
	error: `cannot extract from package "test-package": cannot create hard link /hardlink: no content at /non-existing-target`,
}, {
	summary: "Multiple dangling hard links",
	pkgdata: testutil.MustMakeDeb([]testutil.TarEntry{
		testutil.Dir(0755, "./"),
		testutil.Hrd(0644, "./hardlink1", "./non-existing-target"),
		testutil.Hrd(0644, "./hardlink2", "./non-existing-target"),
	}),
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/**": []tarball.ExtractInfo{{
				Path: "/**",
			}},
		},
	},
	error: `cannot extract from package "test-package": cannot create hard link /hardlink1: no content at /non-existing-target`,
}, {
	summary: "Hard link does not follow the symlink",
	pkgdata: testutil.MustMakeDeb([]testutil.TarEntry{
		testutil.Dir(0755, "./"),
		testutil.Lnk(0644, "./symlink", "./file"),
		testutil.Hrd(0644, "./hardlink", "./symlink"),
	}),
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/**": []tarball.ExtractInfo{{
				Path: "/**",
			}},
		},
	},
	result: map[string]string{
		"/hardlink": "symlink ./file <1>",
		"/symlink":  "symlink ./file <1>",
	},
	notCreated: []string{},
}, {
	summary: "Explicit extraction overrides existing file",
	pkgdata: testutil.PackageData["test-package"],
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/dir/": []tarball.ExtractInfo{{
				Path: "/dir/",
				Mode: 0777,
			}},
		},
	},
	hackopt: func(c *C, o *tarball.ExtractOptions) {
		err := os.Mkdir(path.Join(o.TargetDir, "/dir"), 0666)
		c.Assert(err, IsNil)
	},
	result: map[string]string{
		"/dir/": "dir 0777",
	},
	notCreated: []string{},
}, {
	summary: "Hardlink cannot escape target directory",
	pkgdata: testutil.MustMakeDeb([]testutil.TarEntry{
		testutil.Dir(0755, "./"),
		testutil.Hrd(0644, "./hardlink", "/etc/group"),
	}),
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/**": []tarball.ExtractInfo{{
				Path: "/**",
			}},
		},
	},
	error: `cannot extract from package "test-package": invalid link target /etc/group`,
}, {
	summary: "Cannot extract outside of target directory",
	pkgdata: testutil.MustMakeDeb([]testutil.TarEntry{
		testutil.Dir(0755, "./"),
		testutil.Reg(0644, "./../file", "hijacking system file"),
	}),
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/**": []tarball.ExtractInfo{{
				Path: "/**",
			}},
		},
	},
	error: `cannot extract from package "test-package": cannot create path /[a-z0-9\-\/]*/file outside of root /[a-z0-9\-\/]*`,
}}

func (s *S) TestExtract(c *C) {

	for _, test := range extractTests {
		c.Logf("Test: %s", test.summary)
		dir := c.MkDir()
		options := test.options
		options.Package = "test-package"
		options.TargetDir = dir
		createdPaths := make(map[string]bool)
		options.Create = func(_ []tarball.ExtractInfo, o *fsutil.CreateOptions) error {
			relPath := filepath.Clean("/" + strings.TrimPrefix(o.Path, dir))
			if o.Mode.IsDir() {
				relPath = relPath + "/"
			}
			createdPaths[relPath] = true
			_, err := fsutil.Create(o)
			return err
		}

		if test.hackopt != nil {
			test.hackopt(c, &options)
		}

		err := tarball.Extract(bytes.NewReader(test.pkgdata), &options)
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
	options tarball.ExtractOptions
	calls   map[string][]tarball.ExtractInfo
}{{
	summary: "Create is called with the set of ExtractInfo(s) that match the file",
	pkgdata: testutil.MustMakeDeb([]testutil.TarEntry{
		testutil.Dir(0755, "./"),
		testutil.Dir(0766, "./dir/"),
		testutil.Reg(0644, "./dir/file", "whatever"),
	}),
	options: tarball.ExtractOptions{
		Extract: map[string][]tarball.ExtractInfo{
			"/dir/": []tarball.ExtractInfo{{
				Path: "/dir/",
			}},
			"/d**": []tarball.ExtractInfo{{
				Path: "/d**",
			}},
			"/d?r/": []tarball.ExtractInfo{{
				Path: "/d?r/",
			}},
			"/dir/file": []tarball.ExtractInfo{{
				Path: "/dir/file",
			}, {
				Path: "/dir/file-cpy",
			}},
			"/foo/": []tarball.ExtractInfo{{
				Path:     "/foo/",
				Optional: true,
			}},
		},
	},
	calls: map[string][]tarball.ExtractInfo{
		"/dir/": []tarball.ExtractInfo{
			tarball.ExtractInfo{
				Path: "/d**",
			},
			tarball.ExtractInfo{
				Path: "/d?r/",
			},
			tarball.ExtractInfo{
				Path: "/dir/",
			},
		},
		"/dir/file": []tarball.ExtractInfo{
			tarball.ExtractInfo{
				Path: "/d**",
			},
			tarball.ExtractInfo{
				Path: "/dir/file",
			},
		},
		"/dir/file-cpy": []tarball.ExtractInfo{
			tarball.ExtractInfo{
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
		createExtractInfos := map[string][]tarball.ExtractInfo{}
		options.Create = func(extractInfos []tarball.ExtractInfo, o *fsutil.CreateOptions) error {
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

		err := tarball.Extract(bytes.NewReader(test.pkgdata), &options)
		c.Assert(err, IsNil)

		c.Assert(createExtractInfos, DeepEquals, test.calls)
	}
}

func (s *S) TestExtractOwnership(c *C) {
	// Entries are created with the ownership recorded in the package
	// tarball (e.g. the setgid-shadow group of the PAM chkpwd helpers).
	// Applying ownership requires sufficient privilege (e.g. running as
	// root); otherwise a warning is logged once and the entries keep the
	// ownership of the current process user and group.
	shadowGid := 42
	entries := []testutil.TarEntry{{
		Header: tar.Header{
			Typeflag: tar.TypeDir,
			Name:     "./usr/",
			Mode:     0755,
			Uid:      0,
			Gid:      shadowGid,
			Uname:    "root",
			Gname:    "shadow",
			Format:   tar.FormatGNU,
		},
	}, {
		Header: tar.Header{
			Typeflag: tar.TypeDir,
			Name:     "./usr/sbin/",
			Mode:     0755,
			Uid:      0,
			Gid:      shadowGid,
			Uname:    "root",
			Gname:    "shadow",
			Format:   tar.FormatGNU,
		},
	}, {
		Header: tar.Header{
			Typeflag: tar.TypeReg,
			Name:     "./usr/sbin/unix_chkpwd",
			Mode:     02755,
			Uid:      0,
			Gid:      shadowGid,
			Uname:    "root",
			Gname:    "shadow",
			Format:   tar.FormatGNU,
		},
		Content: []byte("chkpwd-binary-content"),
	}}
	pkgdata, err := testutil.MakeDeb(entries)
	c.Assert(err, IsNil)

	dir := c.MkDir()
	// The ownership warnings are logged through fsutil's logger.
	fsutil.SetLogger(c)
	defer fsutil.SetLogger(nil)
	options := tarball.ExtractOptions{
		Package:   "test-package",
		TargetDir: dir,
		Extract: map[string][]tarball.ExtractInfo{
			"/usr/sbin/unix_chkpwd": []tarball.ExtractInfo{{
				Path: "/usr/sbin/unix_chkpwd",
			}},
		},
	}
	err = tarball.Extract(bytes.NewReader(pkgdata), &options)
	c.Assert(err, IsNil)

	info, err := os.Lstat(filepath.Join(dir, "usr/sbin/unix_chkpwd"))
	c.Assert(err, IsNil)
	c.Assert(info.Mode()&fs.ModeSetgid != 0, Equals, true)
	stat, ok := info.Sys().(*syscall.Stat_t)
	c.Assert(ok, Equals, true)
	dirInfo, err := os.Lstat(filepath.Join(dir, "usr/sbin"))
	c.Assert(err, IsNil)
	dirStat, ok := dirInfo.Sys().(*syscall.Stat_t)
	c.Assert(ok, Equals, true)
	if os.Geteuid() == 0 {
		// Ownership of the entry and its parent directories is
		// preserved from the tarball.
		c.Assert(stat.Uid, Equals, uint32(0))
		c.Assert(stat.Gid, Equals, uint32(shadowGid))
		c.Assert(dirStat.Gid, Equals, uint32(shadowGid))
	} else {
		// Without sufficient privilege the entries are owned by the
		// current process user and group, and a warning is logged.
		c.Assert(stat.Uid, Equals, uint32(os.Getuid()))
		c.Assert(stat.Gid, Equals, uint32(os.Getgid()))
		c.Assert(dirStat.Gid, Equals, uint32(os.Getgid()))
		c.Assert(c.GetTestLog(), Matches, "(?s).*WARNING: cannot preserve file ownership.*")
	}
}

func (s *S) TestExtractOwnershipHardLinks(c *C) {
	// When the content entry of a hard link is not extracted in the
	// first pass, the content is extracted in a second pass to the
	// link path as a regular file, and the ownership recorded in the
	// package tarball is preserved for it too.
	shadowGid := 42
	entries := []testutil.TarEntry{{
		Header: tar.Header{
			Typeflag: tar.TypeDir,
			Name:     "./",
			Mode:     0755,
			Uname:    "root",
			Gname:    "root",
			Format:   tar.FormatGNU,
		},
	}, {
		Header: tar.Header{
			Typeflag: tar.TypeReg,
			Name:     "./file",
			Mode:     0644,
			Uid:      0,
			Gid:      shadowGid,
			Uname:    "root",
			Gname:    "shadow",
			Format:   tar.FormatGNU,
		},
		Content: []byte("text for file"),
	}, {
		Header: tar.Header{
			Typeflag: tar.TypeLink,
			Name:     "./hardlink",
			Mode:     0644,
			Uid:      0,
			Gid:      shadowGid,
			Uname:    "root",
			Gname:    "shadow",
			Format:   tar.FormatGNU,
			Linkname: "./file",
		},
	}}
	pkgdata, err := testutil.MakeDeb(entries)
	c.Assert(err, IsNil)

	dir := c.MkDir()
	options := tarball.ExtractOptions{
		Package:   "test-package",
		TargetDir: dir,
		Extract: map[string][]tarball.ExtractInfo{
			"/hardlink": []tarball.ExtractInfo{{
				Path: "/hardlink",
			}},
		},
	}
	err = tarball.Extract(bytes.NewReader(pkgdata), &options)
	c.Assert(err, IsNil)

	info, err := os.Lstat(filepath.Join(dir, "hardlink"))
	c.Assert(err, IsNil)
	stat, ok := info.Sys().(*syscall.Stat_t)
	c.Assert(ok, Equals, true)
	if os.Geteuid() == 0 {
		// The ownership recorded for the content entry is applied to
		// the file created in its place.
		c.Assert(stat.Uid, Equals, uint32(0))
		c.Assert(stat.Gid, Equals, uint32(shadowGid))
	} else {
		// Without sufficient privilege the file is owned by the
		// current process user and group.
		c.Assert(stat.Uid, Equals, uint32(os.Getuid()))
		c.Assert(stat.Gid, Equals, uint32(os.Getgid()))
	}
}
