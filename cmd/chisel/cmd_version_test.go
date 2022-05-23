package main_test

import (
	. "gopkg.in/check.v1"

	chisel "github.com/canonical/chisel/cmd/chisel"
)

func (s *ChiselSuite) TestVersionCommand(c *C) {
	restore := fakeVersion("4.56")
	defer restore()

	_, err := chisel.Parser().ParseArgs([]string{"version"})
	c.Assert(err, IsNil)
	c.Assert(s.Stdout(), Equals, "4.56\n")
	c.Assert(s.Stderr(), Equals, "")
}
