package deb_test

import (
	"archive/tar"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/deb"
	"github.com/canonical/chisel/internal/tarball"
	"github.com/canonical/chisel/internal/testutil"
)

// Compile-time check that Pkg implements tarball.PkgReader.
var _ tarball.PkgReader = (*deb.Pkg)(nil)

func (s *S) TestPkgTarStream(c *C) {
	pkg := testutil.NewDebPkg(testutil.PackageData["test-package"])

	// Each call returns a fresh stream over the same content.
	for i := 0; i < 2; i++ {
		tarStream, err := pkg.TarStream()
		c.Assert(err, IsNil)
		tarReader := tar.NewReader(tarStream)
		_, err = tarReader.Next()
		c.Assert(err, IsNil)
		err = tarStream.Close()
		c.Assert(err, IsNil)
	}
}

func (s *S) TestPkgExtract(c *C) {
	pkg := testutil.NewDebPkg(testutil.PackageData["test-package"])

	dir := c.MkDir()
	err := tarball.Extract(pkg, &tarball.ExtractOptions{
		Package:   "test-package",
		TargetDir: dir,
		Extract: map[string][]tarball.ExtractInfo{
			"/dir/file": {{Path: "/dir/file"}},
			"/dir/nested/": {{
				Path: "/dir/nested/",
			}},
		},
	})
	c.Assert(err, IsNil)

	result := testutil.TreeDump(dir)
	c.Assert(result, DeepEquals, map[string]string{
		"/dir/":        "dir 0755",
		"/dir/file":    "file 0644 cc55e2ec",
		"/dir/nested/": "dir 0755",
	})
}
