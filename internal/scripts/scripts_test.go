package scripts_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/scripts"
	"github.com/canonical/chisel/internal/testutil"
)

type scriptsTest struct {
	summary string
	content map[string]string
	script  string
	result  map[string]string
	checkr  func(path string) error
	checkw  func(path string) error
	error   string
}

var scriptsTests = []scriptsTest{{
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
	error: `no read: /bar/file2.txt`,
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
	error: `no write: /bar/file1.txt`,
}}

func (s *S) TestScripts(c *C) {
	for _, test := range scriptsTests {
		c.Logf("Summary: %s", test.summary)

		rootDir := c.MkDir()
		for path, data := range test.content {
			fpath := filepath.Join(rootDir, path)
			err := os.MkdirAll(filepath.Dir(fpath), 0755)
			c.Assert(err, IsNil)
			err = ioutil.WriteFile(fpath, []byte(data), 0644)
			c.Assert(err, IsNil)
		}

		content := &scripts.ContentValue{
			RootDir:    rootDir,
			CheckRead:  test.checkr,
			CheckWrite: test.checkw,
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
	}
}
