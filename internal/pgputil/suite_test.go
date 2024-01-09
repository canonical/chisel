package pgputil_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/pgputil"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	pgputil.SetDebug(true)
	pgputil.SetLogger(c)
}

func (s *S) TearDownTest(c *C) {
	pgputil.SetDebug(false)
	pgputil.SetLogger(nil)
}
