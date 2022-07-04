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

func (s *S) testOpenArchiveArch(c *C, arch string) {
	options := archive.Options{
		Label:    "ubuntu",
		Version:  "22.04",
		CacheDir: c.MkDir(),
		Arch:     arch,
	}

	archive, err := archive.Open(&options)
	c.Assert(err, IsNil)

	extractDir := c.MkDir()

	pkg, err := archive.Fetch("hostname")
	c.Assert(err, IsNil)

	err = deb.Extract(pkg, &deb.ExtractOptions{
		Package:   "hostname",
		TargetDir: extractDir,
		Extract: map[string][]deb.ExtractInfo{
			"/usr/share/doc/hostname/copyright": {
				{Path: "/copyright"},
			},
		},
	})
	c.Assert(err, IsNil)

	data, err := ioutil.ReadFile(filepath.Join(extractDir, "copyright"))
	c.Assert(err, IsNil)

	copyrightTop := "This package was written by Peter Tobias <tobias@et-inf.fho-emden.de>\non Thu, 16 Jan 1997 01:00:34 +0100."
	c.Assert(strings.HasPrefix(string(data), copyrightTop), Equals, true)
}

func (s *S) TestOpenArchive(c *C) {
	s.testOpenArchiveArch(c, "amd64")
	s.testOpenArchiveArch(c, "arm64")
}
