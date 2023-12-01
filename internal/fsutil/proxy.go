package fsutil

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"io/fs"
)

type FileInfo struct {
	Path string
	Mode fs.FileMode
	Hash string
	Size uint
}

// FileCreatorProxy implements the FileCreator interface while logging data about the files created.
type FileCreatorProxy struct {
	// map indexed by path.
	Files map[string]FileInfo
}

var _ FileCreator = (*FileCreatorProxy)(nil)

func NewFileCreatorProxy() *FileCreatorProxy {
	return &FileCreatorProxy{Files: make(map[string]FileInfo)}
}

func (fcp *FileCreatorProxy) Create(options *CreateOptions) error {
	rp := readerProxy{inner: options.Data, h: sha256.New()}
	options.Data = &rp
	fileCreator := DefaultFileCreator{}
	err := fileCreator.Create(options)
	fr := FileInfo{
		Path: options.Path,
		Mode: options.Mode,
		Hash: hex.EncodeToString(rp.h.Sum(nil)),
		Size: rp.size,
	}
	fcp.Files[options.Path] = fr
	return err
}

// readerProxy implements the io.Reader interface proxying the calls to its inner io.Reader. On each read, the proxy
// calculates the file size and hash.
type readerProxy struct {
	inner io.Reader
	h     hash.Hash
	size  uint
}

var _ io.Reader = (*readerProxy)(nil)

func (fr *readerProxy) Read(p []byte) (n int, err error) {
	n, err = fr.inner.Read(p)
	fr.h.Write(p[:n])
	fr.size += uint(n)
	return n, err
}
