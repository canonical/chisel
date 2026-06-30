package tarball

import (
	"io"

	"github.com/ulikunitz/xz"
)

func XZDataReader(pkgReader io.ReadSeeker) (io.ReadCloser, error) {
	xzReader, err := xz.NewReader(pkgReader)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(xzReader), nil
}
