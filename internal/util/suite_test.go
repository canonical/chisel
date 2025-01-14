// SPDX-License-Identifier: Apache-2.0
package util_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/util"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	util.SetDebug(true)
	util.SetLogger(c)
}

func (s *S) TearDownTest(c *C) {
	util.SetDebug(false)
	util.SetLogger(nil)
}
