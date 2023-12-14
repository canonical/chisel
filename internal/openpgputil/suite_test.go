package openpgputil_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/openpgputil"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	openpgputil.SetDebug(true)
	openpgputil.SetLogger(c)
}

func (s *S) TearDownTest(c *C) {
	openpgputil.SetDebug(false)
	openpgputil.SetLogger(nil)
}
