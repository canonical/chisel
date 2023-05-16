package jsonwall_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/jsonwall"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	jsonwall.SetDebug(true)
	jsonwall.SetLogger(c)
}

func (s *S) TearDownTest(c *C) {
	jsonwall.SetDebug(false)
	jsonwall.SetLogger(nil)
}
