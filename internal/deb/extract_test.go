package deb_test

import (
	"bytes"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/deb"
	"github.com/canonical/chisel/internal/testutil"
)

func (s *S) TestOpenTar(c *C) {
	pkgReader := testutil.ReadSeekNopCloser(bytes.NewReader(testutil.PackageData["test-package"]))
	openTar := deb.OpenTar(pkgReader)

	// Each call must return a fresh stream over the same content.
	testutil.CheckOpenTar(c, openTar)
}
