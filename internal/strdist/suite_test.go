package strdist_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/strdist"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	strdist.SetDebug(true)
	strdist.SetLogger(c)
}

func (s *S) TearDownTest(c *C) {
	strdist.SetDebug(false)
	strdist.SetLogger(nil)
}
