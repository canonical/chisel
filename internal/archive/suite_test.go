package archive_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/archive"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	archive.SetLogger(c)
}

func (s *S) TearDownTest(c *C) {
	archive.SetLogger(nil)
}
