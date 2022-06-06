package scripts_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/scripts"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	scripts.SetDebug(true)
	scripts.SetLogger(c)
}

func (s *S) TearDownTest(c *C) {
	scripts.SetDebug(false)
	scripts.SetLogger(nil)
}
