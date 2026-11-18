package fsutil

import (
	"os"
	"path/filepath"
	"sync"

	check "gopkg.in/check.v1"
)

type InternalS struct{}

var _ = check.Suite(&InternalS{})

// TestOwnershipWarningLoggedOnce verifies that the ownership warning is
// logged once, however many ownership changes are tolerated.
//
// This test lives in the fsutil package (not fsutil_test) so that it can
// reset the package-global warning state, which is otherwise logged at
// most once per process.
func (s *InternalS) TestOwnershipWarningLoggedOnce(c *check.C) {
	selfUid, selfGid := os.Geteuid(), os.Getegid()
	uid, gid := 1, 1
	if uid == selfUid {
		uid = 2
	}
	if gid == selfGid {
		gid = 2
	}

	dir := c.MkDir()
	path := filepath.Join(dir, "file")
	err := os.WriteFile(path, nil, 0600)
	c.Assert(err, check.IsNil)

	// The tolerated ownership failure can only be observed when the
	// current process cannot set the requested ownership.
	probeErr := os.Chown(path, uid, gid)
	if probeErr == nil {
		c.Log("cannot test: the current process can set any ownership")
		return
	}

	// Reset the warning and record the log to observe it.
	warnOwnership = sync.Once{}
	defer func() {
		warnOwnership = sync.Once{}
	}()
	logger := &recordingLogger{}
	SetLogger(logger)
	defer SetLogger(nil)

	options := &CreateOptions{Mode: 0644, Uid: &uid, Gid: &gid}
	c.Assert(applyOwnership(path, options), check.IsNil)
	c.Assert(applyOwnership(path, options), check.IsNil)

	c.Assert(logger.logs, check.HasLen, 1)
	c.Assert(logger.logs[0], check.Matches, "(?s).*WARNING: cannot preserve file ownership.*")
}

type recordingLogger struct {
	logs []string
}

func (l *recordingLogger) Output(_ int, s string) error {
	l.logs = append(l.logs, s)
	return nil
}
