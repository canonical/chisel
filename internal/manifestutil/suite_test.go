package manifestutil_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/manifestutil"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	manifestutil.SetDebug(true)
	manifestutil.SetLogger(c)
}

func (s *S) TearDownTest(c *C) {
	manifestutil.SetDebug(false)
	manifestutil.SetLogger(nil)
}
