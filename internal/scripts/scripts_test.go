package scripts_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/scripts"
	"github.com/canonical/chisel/internal/testutil"
)

type scriptsTest struct {
	summary string
	content map[string]string
	hackdir func(c *C, dir string)
	script  string
	result  map[string]string
	mutated map[string]string
	checkr  func(path string) error
	checkw  func(path string) error
	error   string
}

var scriptsTests = []scriptsTest{{
	summary: "Allow reassignment (non-standard Starlark)",
	script: `
		data = 1
		data = 2
	`,
	result: map[string]string{},
}, {
	summary: "Overwrite a couple of files",
	content: map[string]string{
		"foo/file1.txt": ``,
		"foo/file2.txt": ``,
	},
	script: `
		content.write("/foo/file1.txt", "data1")
		content.write("/foo/file2.txt", "data2")
	`,
	result: map[string]string{
		"/foo/":          "dir 0755",
		"/foo/file1.txt": "file 0644 5b41362b",
		"/foo/file2.txt": "file 0644 d98cf53e",
	},
	mutated: map[string]string{
		"/foo/file1.txt": "file 0644 5b41362b",
		"/foo/file2.txt": "file 0644 d98cf53e",
	},
}, {
	summary: "Read a file",
	content: map[string]string{
		"foo/file1.txt": `data1`,
		"foo/file2.txt": ``,
	},
	script: `
		data = content.read("/foo/file1.txt")
		content.write("/foo/file2.txt", data)
	`,
	result: map[string]string{
		"/foo/":          "dir 0755",
		"/foo/file1.txt": "file 0644 5b41362b",
		"/foo/file2.txt": "file 0644 5b41362b",
	},
	mutated: map[string]string{
		"/foo/file2.txt": "file 0644 5b41362b",
	},
}, {
	summary: "List a directory",
	content: map[string]string{
		"foo/file1.txt": `data1`,
		"foo/file2.txt": `data1`,
		"bar/file3.txt": `data1`,
	},
	script: `
		content.write("/foo/file1.txt", ",".join(content.list("/foo")))
		content.write("/foo/file2.txt", ",".join(content.list("/")))
	`,
	result: map[string]string{
		"/foo/":          "dir 0755",
		"/foo/file1.txt": "file 0644 98139a06", // "file1.txt,file2.txt"
		"/foo/file2.txt": "file 0644 47c22b01", // "bar/,foo/"
		"/bar/":          "dir 0755",
		"/bar/file3.txt": "file 0644 5b41362b",
	},
}, {
	summary: "OnWrite is called for modified files only",
	content: map[string]string{
		"foo/file1.txt": `placeholder`,
		"foo/file2.txt": `placeholder`,
		// This file is not mutable, it cannot be written to.
		"foo/file3.txt": `placeholder`,
	},
	script: `
		content.write("/foo/file1.txt", "data1")
		content.write("/foo/file2.txt", "data2")
	`,
	checkw: func(p string) error {
		if p == "foo/file3.txt" {
			return fmt.Errorf("no write: %s", p)
		}
		return nil
	},
	result: map[string]string{
		"/foo/":          "dir 0755",
		"/foo/file1.txt": "file 0644 5b41362b",
		"/foo/file2.txt": "file 0644 d98cf53e",
		"/foo/file3.txt": "file 0644 40978892",
	},
	mutated: map[string]string{
		"/foo/file1.txt": "file 0644 5b41362b",
		"/foo/file2.txt": "file 0644 d98cf53e",
	},
}, {
	summary: "Mode is not changed when writing to a file",
	content: map[string]string{
		"foo/file1.txt": ``,
		"foo/file2.txt": ``,
	},
	hackdir: func(c *C, dir string) {
		fpath1 := filepath.Join(dir, "foo/file1.txt")
		_ = os.Chmod(fpath1, 0744)
	},
	script: `
		content.write("/foo/file1.txt", "data1")
		content.write("/foo/file2.txt", "data2")
	`,
	result: map[string]string{
		"/foo/":          "dir 0755",
		"/foo/file1.txt": "file 0744 5b41362b",
		"/foo/file2.txt": "file 0644 d98cf53e",
	},
}, {
	summary: "Forbid relative paths",
	content: map[string]string{
		"foo/file1.txt": `data1`,
	},
	script: `
		content.read("foo/file1.txt")
	`,
	error: `content path must be absolute, got: foo/file1.txt`,
}, {
	summary: "Forbid leaving the content root",
	content: map[string]string{
		"foo/file1.txt": `data1`,
	},
	script: `
		content.read("/foo/../../file1.txt")
	`,
	error: `invalid content path: /foo/../../file1.txt`,
}, {
	summary: "Forbid leaving the content via bad symlinks",
	content: map[string]string{
		"foo/file3.txt": ``,
	},
	hackdir: func(c *C, dir string) {
		fpath1 := filepath.Join(dir, "foo/file1.txt")
		fpath2 := filepath.Join(dir, "foo/file2.txt")
		c.Assert(os.Symlink("file2.txt", fpath1), IsNil)
		c.Assert(os.Symlink("../../bar", fpath2), IsNil)
	},
	script: `
		content.read("/foo/file1.txt")
	`,
	error: `invalid content symlink: /foo/file2.txt`,
}, {
	summary: "Path errors refer to the root",
	content: map[string]string{},
	script: `
		content.read("/foo/file1.txt")
	`,
	error: `open /foo/file1.txt: no such file or directory`,
}, {
	summary: "Check reads",
	content: map[string]string{
		"bar/file1.txt": `data1`,
	},
	script: `
		content.write("/foo/../bar/file2.txt", "data2")
		content.read("/foo/../bar/file2.txt")
	`,
	checkr: func(p string) error { return fmt.Errorf("no read: %s", p) },
	error:  `no read: /bar/file2.txt`,
}, {
	summary: "Check writes",
	content: map[string]string{
		"bar/file1.txt": `data1`,
	},
	script: `
		content.read("/foo/../bar/file1.txt")
		content.write("/foo/../bar/file1.txt", "data1")
	`,
	checkw: func(p string) error { return fmt.Errorf("no write: %s", p) },
	error:  `no write: /bar/file1.txt`,
}, {
	summary: "Check lists",
	content: map[string]string{
		"bar/file1.txt": `data1`,
	},
	script: `
		content.write("/foo/../bar/file2.txt", "data2")
		content.list("/foo/../bar/")
	`,
	checkr: func(p string) error { return fmt.Errorf("no read: %s", p) },
	error:  `no read: /bar/`,
}, {
	summary: "Check lists",
	content: map[string]string{
		"bar/file1.txt": `data1`,
	},
	script: `
		content.write("/foo/../bar/file2.txt", "data2")
		content.list("/foo/../bar")
	`,
	checkr: func(p string) error { return fmt.Errorf("no read: %s", p) },
	error:  `no read: /bar/`,
}, {
	summary: "Check reads on symlinks",
	content: map[string]string{
		"foo/file2.txt": ``,
	},
	hackdir: func(c *C, dir string) {
		fpath1 := filepath.Join(dir, "foo/file1.txt")
		c.Assert(os.Symlink("file2.txt", fpath1), IsNil)
	},
	script: `
		content.read("/foo/file1.txt")
	`,
	checkr: func(p string) error {
		if p == "/foo/file2.txt" {
			return fmt.Errorf("no read: %s", p)
		}
		return nil
	},
	error: `no read: /foo/file2.txt`,
}, {
	summary: "Check writes on symlinks",
	content: map[string]string{
		"foo/file2.txt": ``,
	},
	hackdir: func(c *C, dir string) {
		fpath1 := filepath.Join(dir, "foo/file1.txt")
		c.Assert(os.Symlink("file2.txt", fpath1), IsNil)
	},
	script: `
		content.write("/foo/file1.txt", "")
	`,
	checkw: func(p string) error {
		if p == "/foo/file2.txt" {
			return fmt.Errorf("no write: %s", p)
		}
		return nil
	},
	error: `no write: /foo/file2.txt`,
}}

func (s *S) TestScripts(c *C) {
	for _, test := range scriptsTests {
		c.Logf("Summary: %s", test.summary)

		rootDir := c.MkDir()
		for path, data := range test.content {
			fpath := filepath.Join(rootDir, path)
			err := os.MkdirAll(filepath.Dir(fpath), 0755)
			c.Assert(err, IsNil)
			err = os.WriteFile(fpath, []byte(data), 0644)
			c.Assert(err, IsNil)
		}
		if test.hackdir != nil {
			test.hackdir(c, rootDir)
		}

		mutatedFiles := map[string]string{}
		content := &scripts.ContentValue{
			RootDir:    rootDir,
			CheckRead:  test.checkr,
			CheckWrite: test.checkw,
			OnWrite: func(entry *fsutil.Entry) error {
				// Set relative path.
				entry.Path = strings.TrimPrefix(entry.Path, rootDir)
				mutatedFiles[entry.Path] = testutil.TreeDumpEntry(entry)
				return nil
			},
		}
		namespace := map[string]scripts.Value{
			"content": content,
		}
		err := scripts.Run(&scripts.RunOptions{
			Namespace: namespace,
			Script:    string(testutil.Reindent(test.script)),
		})
		if test.error == "" {
			c.Assert(err, IsNil)
		} else {
			c.Assert(err, ErrorMatches, test.error)
			continue
		}

		c.Assert(testutil.TreeDump(rootDir), DeepEquals, test.result)

		if test.mutated != nil {
			c.Assert(mutatedFiles, DeepEquals, test.mutated)
		}
	}
}

func (s *S) TestContentRelative(c *C) {
	content := scripts.ContentValue{RootDir: "foo"}
	_, err := content.RealPath("/bar", scripts.CheckNone)
	c.Assert(err, ErrorMatches, "internal error: content defined with relative root: foo")
}
