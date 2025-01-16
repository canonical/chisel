package testutil

import (
	"io"
)

// readSeekNopCloser is an io.ReadSeeker that does nothing on Close.
type readSeekNopCloser struct {
	io.ReadSeeker
}

func (readSeekNopCloser) Close() error { return nil }

// ReadSeekNopCloser is an extension of io.NopCloser that also implements
// io.Seeker.
func ReadSeekNopCloser(r io.ReadSeeker) io.ReadSeekCloser {
	return readSeekNopCloser{r}
}
