package slicer_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/slicer"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	slicer.SetDebug(true)
	slicer.SetLogger(c)
}

func (s *S) TearDownTest(c *C) {
	slicer.SetDebug(false)
	slicer.SetLogger(nil)
}
