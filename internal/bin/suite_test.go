package bin_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/bin"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	bin.SetDebug(true)
	bin.SetLogger(c)
}

func (s *S) TearDownTest(c *C) {
	bin.SetDebug(false)
	bin.SetLogger(nil)
}
