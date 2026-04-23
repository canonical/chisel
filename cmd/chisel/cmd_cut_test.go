package main_test

import (
	. "gopkg.in/check.v1"

	chisel "github.com/canonical/chisel/cmd/chisel"
)

func (s *ChiselSuite) TestCutRejectsPrivateSlice(c *C) {
	dir := c.MkDir()
	_, err := chisel.Parser().ParseArgs([]string{"cut", "--root", dir, "mypkg__priv"})
	c.Assert(err, ErrorMatches, `cannot cut private slice mypkg__priv`)
}
