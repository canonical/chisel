package testutil

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	check "gopkg.in/check.v1"
)

// CanApplyOwnership returns whether the current process can change the
// ownership of entries to the given uid and gid, which are expected to
// be different from the current process ones. Chisel tolerates ownership
// it cannot set, with a warning, so tests need to know whether a given
// ownership is expected to land on the extracted entries or not.
//
// Probing is required rather than checking for effective root: inside a
// user namespace (e.g. under unshare) the effective uid is 0, but other
// ids may be unmapped and unsettable (EINVAL).
func CanApplyOwnership(c *check.C, uid, gid int) bool {
	dir := c.MkDir()
	path := filepath.Join(dir, "probe")
	err := os.WriteFile(path, nil, 0600)
	c.Assert(err, check.IsNil)
	err = os.Chown(path, uid, gid)
	if err == nil {
		return true
	}
	c.Assert(
		errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EINVAL),
		check.Equals,
		true,
	)
	return false
}
