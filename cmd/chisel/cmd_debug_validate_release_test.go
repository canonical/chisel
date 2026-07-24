package main_test

import (
	"os"
	"path/filepath"

	. "gopkg.in/check.v1"

	chisel "github.com/canonical/chisel/cmd/chisel"
	"github.com/canonical/chisel/internal/testutil"
)

// writeRelease writes the provided release files into a temporary directory
// and returns its path.
func writeRelease(c *C, files map[string]string) string {
	dir := c.MkDir()
	for path, data := range files {
		fpath := filepath.Join(dir, path)
		err := os.MkdirAll(filepath.Dir(fpath), 0o755)
		c.Assert(err, IsNil)
		err = os.WriteFile(fpath, testutil.Reindent(data), 0o644)
		c.Assert(err, IsNil)
	}
	err := os.MkdirAll(filepath.Join(dir, "slices"), 0o755)
	c.Assert(err, IsNil)
	return dir
}

func (s *ChiselSuite) TestValidateReleaseSuccess(c *C) {
	s.ResetStdStreams()

	dir := writeRelease(c, map[string]string{
		"chisel.yaml": testutil.DefaultChiselYaml,
		"slices/mydir/pkg-a.yaml": `
			package: pkg-a
			slices:
				myslice:
					contents:
						/file/path:
		`,
	})
	_, err := chisel.Parser().ParseArgs(
		[]string{"debug", "validate-release", "--release", dir},
	)
	c.Assert(err, IsNil)
	c.Assert(s.Stdout(), Equals, "Release is valid\n")
}

func (s *ChiselSuite) TestValidateReleaseErrorPropagation(c *C) {
	s.ResetStdStreams()

	dir := writeRelease(c, map[string]string{
		"chisel.yaml": testutil.DefaultChiselYaml,
		"slices/mydir/pkg-a.yaml": `
			package: pkg-a
			slices:
				myslice:
					contents:
						/path1:
		`,
		"slices/mydir/pkg-b.yaml": `
			package: pkg-b
			slices:
				myslice:
					contents:
						/path1:
		`,
	})
	_, err := chisel.Parser().ParseArgs(
		[]string{"debug", "validate-release", "--release", dir},
	)
	c.Assert(err, ErrorMatches, `slices pkg-a_myslice and pkg-b_myslice conflict on /path1`)
	c.Assert(s.Stdout(), Equals, "")
}

func (s *ChiselSuite) TestValidateReleaseExtraArgs(c *C) {
	s.ResetStdStreams()

	dir := writeRelease(c, map[string]string{
		"chisel.yaml": testutil.DefaultChiselYaml,
		"slices/mydir/pkg-a.yaml": `
			package: pkg-a
			slices:
				myslice:
					contents:
						/file/path:
		`,
	})
	_, err := chisel.Parser().ParseArgs(
		[]string{"debug", "validate-release", "--release", dir, "extra"},
	)
	c.Assert(err, ErrorMatches, `too many arguments for command`)
}
