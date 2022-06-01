package deb_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/deb"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	deb.SetDebug(true)
	deb.SetLogger(c)
}

func (s *S) TearDownTest(c *C) {
	deb.SetDebug(false)
	deb.SetLogger(nil)
}
