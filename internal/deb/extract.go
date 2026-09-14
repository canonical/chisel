package deb

import (
	"compress/gzip"
	"fmt"
	"io"

	"github.com/blakesmith/ar"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// Pkg reads the tar stream of a Debian package held in a seekable reader.
type Pkg struct {
	reader io.ReadSeekCloser
}

// OpenPkg wraps a seekable reader over Debian package data.
func OpenPkg(reader io.ReadSeekCloser) *Pkg {
	return &Pkg{reader: reader}
}

// TarStream returns a reader over the data tarball of the Debian
// package, from the start of the package.
func (p *Pkg) TarStream() (io.ReadCloser, error) {
	_, err := p.reader.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	arReader := ar.NewReader(p.reader)
	var dataReader io.ReadCloser
	for dataReader == nil {
		arHeader, err := arReader.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("no data payload")
		}
		if err != nil {
			return nil, err
		}
		switch arHeader.Name {
		case "data.tar.gz":
			gzipReader, err := gzip.NewReader(arReader)
			if err != nil {
				return nil, err
			}
			dataReader = gzipReader
		case "data.tar.xz":
			xzReader, err := xz.NewReader(arReader)
			if err != nil {
				return nil, err
			}
			dataReader = io.NopCloser(xzReader)
		case "data.tar.zst":
			zstdReader, err := zstd.NewReader(arReader)
			if err != nil {
				return nil, err
			}
			dataReader = zstdReader.IOReadCloser()
		}
	}

	return dataReader, nil
}

func (p *Pkg) Close() error {
	return p.reader.Close()
}
