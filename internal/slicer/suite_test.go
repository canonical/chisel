package slicer_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/slicer"
)

func Test(t *testing.T) { TestingT(t) }

type S struct {
	logProxy *LogProxy
}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	slicer.SetDebug(true)
	s.logProxy = &LogProxy{c, ""}
	slicer.SetLogger(s.logProxy)
}

func (s *S) TearDownTest(c *C) {
	slicer.SetDebug(false)
	slicer.SetLogger(nil)
}

func (s *S) LogProxy() *LogProxy {
	return s.logProxy
}

// Helper because go-check does not have the ability to get the log output that
// was emitted only a given test case.
type LogProxy struct {
	c      *C
	output string
}

func (in *LogProxy) Output(calldepth int, s string) error {
	in.output += s + "\n"
	return in.c.Output(calldepth, s)
}

func (in *LogProxy) Reset() {
	in.output = ""
}

func (in *LogProxy) Get() string {
	return in.output
}
