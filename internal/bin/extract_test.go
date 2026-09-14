package bin_test

import (
	"archive/tar"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/bin"
	"github.com/canonical/chisel/internal/tarball"
	"github.com/canonical/chisel/internal/testutil"
)

// Compile-time check that Pkg implements tarball.PkgReader.
var _ tarball.PkgReader = (*bin.Pkg)(nil)

func (s *S) TestPkgTarStream(c *C) {
	pkg := testutil.NewBinPkg(testutil.MustMakeBin([]testutil.TarEntry{
		testutil.Dir(0o755, "./"),
		testutil.Reg(0o644, "./file", "content"),
	}))

	// Each call returns a fresh stream over the same content.
	for range 2 {
		tarStream, err := pkg.TarStream()
		c.Assert(err, IsNil)
		tarReader := tar.NewReader(tarStream)
		_, err = tarReader.Next()
		c.Assert(err, IsNil)
		err = tarStream.Close()
		c.Assert(err, IsNil)
	}
}

func (s *S) TestPkgTarStreamInvalid(c *C) {
	pkg := testutil.NewBinPkg([]byte("not an xz stream"))

	_, err := pkg.TarStream()
	c.Assert(err, ErrorMatches, "xz.*")
}
