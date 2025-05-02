// SPDX-License-Identifier: Apache-2.0

package apacheutil_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/apacheutil"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	apacheutil.SetDebug(true)
	apacheutil.SetLogger(c)
}

func (s *S) TearDownTest(c *C) {
	apacheutil.SetDebug(false)
	apacheutil.SetLogger(nil)
}
