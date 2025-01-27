// SPDX-License-Identifier: Apache-2.0

package manifest_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/public/manifest"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	manifest.SetDebug(true)
	manifest.SetLogger(c)
}

func (s *S) TearDownTest(c *C) {
	manifest.SetDebug(false)
	manifest.SetLogger(nil)
}
