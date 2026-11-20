package bin

import (
	"io"

	"github.com/ulikunitz/xz"
)

// DataReader takes a Reader for a bin package (an XZ-compressed tarball) and
// returns a Reader to the inner tarball.
func DataReader(pkgReader io.Reader) (io.ReadCloser, error) {
	xzReader, err := xz.NewReader(pkgReader)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(xzReader), nil
}

func OpenTar(pkgReader io.ReadSeeker) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		_, err := pkgReader.Seek(0, io.SeekStart)
		if err != nil {
			return nil, err
		}
		return DataReader(pkgReader)
	}
}
