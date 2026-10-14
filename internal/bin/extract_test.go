package bin_test

import (
	"archive/tar"
	"bytes"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/bin"
	"github.com/canonical/chisel/internal/testutil"
)

func (s *S) TestDataReader(c *C) {
	data := testutil.MustMakeBin([]testutil.TarEntry{
		testutil.Dir(0o755, "./"),
		testutil.Reg(0o644, "./file", "content"),
	})

	dataReader, err := bin.DataReader(bytes.NewReader(data))
	c.Assert(err, IsNil)
	tarReader := tar.NewReader(dataReader)
	_, err = tarReader.Next()
	c.Assert(err, IsNil)
}

func (s *S) TestDataReaderInvalid(c *C) {
	_, err := bin.DataReader(bytes.NewReader([]byte("not an xz stream")))
	c.Assert(err, ErrorMatches, "xz.*")
}

func (s *S) TestOpenTar(c *C) {
	data := testutil.MustMakeBin([]testutil.TarEntry{
		testutil.Dir(0o755, "./"),
		testutil.Reg(0o644, "./file", "content"),
	})
	pkgReader := testutil.ReadSeekNopCloser(bytes.NewReader(data))
	openTar := bin.OpenTar(pkgReader)

	// Each call must return a fresh stream over the same content.
	testutil.CheckOpenTar(c, openTar)
}
