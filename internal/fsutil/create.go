package fsutil

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

type CreateOptions struct {
	Path string
	Mode fs.FileMode
	Data io.Reader
	Link string
	// If MakeParents is true, missing parent directories of Path are
	// created with permissions 0755.
	MakeParents bool
}

type Info struct {
	Path string
	Mode fs.FileMode
	Hash string
	Size uint
	Link string
}

type Creator struct {
	// Created keeps track of information about the files created. If a file
	// is created several times it only tracks the latest one.
	Created map[string]Info
}

func NewCreator() *Creator {
	return &Creator{Created: make(map[string]Info)}
}

// Creates a filesystem entry according to the provided options.
func (fc Creator) Create(o *CreateOptions) error {
	rp := readerProxy{inner: o.Data, h: sha256.New()}
	o.Data = &rp

	var err error
	if o.MakeParents {
		if err := os.MkdirAll(filepath.Dir(o.Path), 0755); err != nil {
			return err
		}
	}
	switch o.Mode & fs.ModeType {
	case 0:
		err = createFile(o)
	case fs.ModeDir:
		err = createDir(o)
	case fs.ModeSymlink:
		err = createSymlink(o)
	default:
		err = fmt.Errorf("unsupported file type: %s", o.Path)
	}
	if err != nil {
		return err
	}

	fr := Info{
		Path: o.Path,
		Mode: o.Mode,
		Hash: hex.EncodeToString(rp.h.Sum(nil)),
		Size: rp.size,
		Link: o.Link,
	}
	fc.Created[o.Path] = fr
	return nil
}

func createDir(o *CreateOptions) error {
	debugf("Creating directory: %s (mode %#o)", o.Path, o.Mode)
	err := os.Mkdir(o.Path, o.Mode)
	if os.IsExist(err) {
		err = os.Chmod(o.Path, o.Mode)
	}
	return err
}

func createFile(o *CreateOptions) error {
	debugf("Writing file: %s (mode %#o)", o.Path, o.Mode)
	file, err := os.OpenFile(o.Path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, o.Mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, o.Data)
	err = file.Close()
	if copyErr != nil {
		return copyErr
	}
	return err
}

func createSymlink(o *CreateOptions) error {
	debugf("Creating symlink: %s => %s", o.Path, o.Link)
	fileinfo, err := os.Lstat(o.Path)
	if err == nil {
		if (fileinfo.Mode() & os.ModeSymlink) != 0 {
			link, err := os.Readlink(o.Path)
			if err != nil {
				return err
			}
			if link == o.Link {
				return nil
			}
		}
		err = os.Remove(o.Path)
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(o.Link, o.Path)
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
