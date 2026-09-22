package bin

import (
	"io"

	"github.com/ulikunitz/xz"
)

type Pkg struct {
	reader io.ReadSeekCloser
}

func OpenPkg(reader io.ReadSeekCloser) *Pkg {
	return &Pkg{reader: reader}
}

func (p *Pkg) TarStream() (io.ReadCloser, error) {
	_, err := p.reader.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	xzReader, err := xz.NewReader(p.reader)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(xzReader), nil
}

func (p *Pkg) Close() error {
	return p.reader.Close()
}
