package slicer_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/slicer"
)

func Test(t *testing.T) { TestingT(t) }

type S struct {
	interceptor *LogInterceptor
}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	slicer.SetDebug(true)
	s.interceptor = &LogInterceptor{c, ""}
	slicer.SetLogger(s.interceptor)
}

func (s *S) TearDownTest(c *C) {
	slicer.SetDebug(false)
	slicer.SetLogger(nil)
}

func (s *S) LogInterceptor() *LogInterceptor {
	return s.interceptor
}

// Helper because go-check does not have the ability to get the log output that
// was emitted only a given test case.
type LogInterceptor struct {
	c      *C
	output string
}

func (in *LogInterceptor) Output(calldepth int, s string) error {
	in.output += s + "\n"
	return in.c.Output(calldepth, s)
}

func (in *LogInterceptor) Reset() {
	in.output = ""
}

func (in *LogInterceptor) Get() string {
	return in.output
}
