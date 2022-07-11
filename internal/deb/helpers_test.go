package deb_test

import (
	"github.com/canonical/chisel/internal/deb"
	. "gopkg.in/check.v1"
)

func inferArchFromPlatform(platformArch string) string {
	restore := deb.FakePlatformGoArch(platformArch)
	defer restore()
	goArch, _ := deb.InferArch()
	return goArch
}

func (s *S) TestInferArch(c *C) {
	c.Assert(inferArchFromPlatform("386"), Equals, "i386")
	c.Assert(inferArchFromPlatform("amd64"), Equals, "amd64")
	c.Assert(inferArchFromPlatform("arm"), Equals, "armhf")
	c.Assert(inferArchFromPlatform("arm64"), Equals, "arm64")
	c.Assert(inferArchFromPlatform("ppc64le"), Equals, "ppc64el")
	c.Assert(inferArchFromPlatform("riscv64"), Equals, "riscv64")
	c.Assert(inferArchFromPlatform("s390x"), Equals, "s390x")
	c.Assert(inferArchFromPlatform("i386"), Equals, "")
	c.Assert(inferArchFromPlatform("armhf"), Equals, "")
	c.Assert(inferArchFromPlatform("ppc64el"), Equals, "")
	c.Assert(inferArchFromPlatform("foo"), Equals, "")
	c.Assert(inferArchFromPlatform(""), Equals, "")
}

func (s *S) TestValidateArch(c *C) {
	c.Assert(deb.ValidateArch("i386"), IsNil)
	c.Assert(deb.ValidateArch("amd64"), IsNil)
	c.Assert(deb.ValidateArch("armhf"), IsNil)
	c.Assert(deb.ValidateArch("arm64"), IsNil)
	c.Assert(deb.ValidateArch("ppc64el"), IsNil)
	c.Assert(deb.ValidateArch("riscv64"), IsNil)
	c.Assert(deb.ValidateArch("s390x"), IsNil)
	c.Assert(deb.ValidateArch("386"), Not(IsNil))
	c.Assert(deb.ValidateArch("arm"), Not(IsNil))
	c.Assert(deb.ValidateArch("ppc64le"), Not(IsNil))
	c.Assert(deb.ValidateArch("foo"), Not(IsNil))
	c.Assert(deb.ValidateArch("i3866"), Not(IsNil))
	c.Assert(deb.ValidateArch(""), Not(IsNil))
}
