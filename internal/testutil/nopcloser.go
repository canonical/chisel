package testutil

import (
	"io"
)

// readSeekNopCloser is an io.ReadSeeker that does nothing on Close.
// It is an extension of io.NopCloser that also implements io.Seeker.
type readSeekNopCloser struct {
	io.ReadSeeker
}

func (readSeekNopCloser) Close() error { return nil }

func ReadSeekNopCloser(r io.ReadSeeker) io.ReadSeekCloser {
	return readSeekNopCloser{r}
}
