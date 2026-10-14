package testutil

import (
	"bytes"
	"io"
)

// TestPkg is a package holding an in-memory tar stream, providing an
// OpenTarFunc over it. It is not tied to any package format.
type TestPkg struct {
	tarData []byte
}

// NewTestPkg returns a TestPkg holding a tar stream with the given entries.
func NewTestPkg(entries ...TarEntry) *TestPkg {
	return &TestPkg{tarData: MustMakeTar(entries)}
}

// NewTestPkgFromTar returns a TestPkg holding the given raw tar stream.
func NewTestPkgFromTar(tarData []byte) *TestPkg {
	return &TestPkg{tarData: tarData}
}

// OpenTar returns a function opening a fresh reader over the package's
// tar stream on every call, conforming to the OpenTarFunc contract.
func (p *TestPkg) OpenTar() func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(p.tarData)), nil
	}
}
