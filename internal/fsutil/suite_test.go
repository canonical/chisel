package fsutil_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/fsutil"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	fsutil.SetDebug(true)
	fsutil.SetLogger(c)
}

func (s *S) TearDownTest(c *C) {
	fsutil.SetDebug(false)
	fsutil.SetLogger(nil)
}
