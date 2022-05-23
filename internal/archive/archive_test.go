package archive_test

import (
	. "gopkg.in/check.v1"

	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/deb"
)

// TODO Implement local test server instead of using live archive.

func (s *S) TestOpenArchive(c *C) {
	options := archive.Options{
		Label:   "ubuntu",
		Version: "22.04",
		CacheDir: c.MkDir(),
		Arch:     "amd64",
	}

	archive, err := archive.Open(&options)
	c.Assert(err, IsNil)

	extractDir := c.MkDir()

	pkg, err := archive.Fetch("run-one")
	c.Assert(err, IsNil)

	err = deb.Extract(pkg, &deb.ExtractOptions{
		Package:   "run-one",
		TargetDir: extractDir,
		Extract: map[string][]deb.ExtractInfo{
			"/usr/share/doc/run-one/copyright": {
				{Path: "/copyright"},
			},
		},
	})
	c.Assert(err, IsNil)

	data, err := ioutil.ReadFile(filepath.Join(extractDir, "copyright"))
	c.Assert(err, IsNil)

	c.Assert(strings.Contains(string(data), "Upstream-Name: run-one"), Equals, true)
}
