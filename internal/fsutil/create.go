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
	// If Link is set and the symlink flag is set in Mode, a symlink is
	// created. If the Mode is not set to symlink, a hard link is created
	// instead.
	Link string
	// If MakeParents is true, missing parent directories of Path are
	// created with permissions 0755.
	MakeParents bool
}

type Entry struct {
	Path string
	Mode fs.FileMode
	Hash string
	Size int
	Link string
}

// Create creates a filesystem entry according to the provided options and returns
// the information about the created entry.
func Create(options *CreateOptions) (*Entry, error) {
	rp := &readerProxy{inner: options.Data, h: sha256.New()}
	// Use the proxy instead of the raw Reader.
	optsCopy := *options
	optsCopy.Data = rp
	o := &optsCopy

	var err error
	var hash string
	if o.MakeParents {
		if err := os.MkdirAll(filepath.Dir(o.Path), 0755); err != nil {
			return nil, err
		}
	}

	switch o.Mode & fs.ModeType {
	case 0:
		if o.Link != "" {
			// Creating the hard link does not involve reading the file.
			// Therefore, its size and hash is not calculated here.
			err = createHardLink(o)
		} else {
			err = createFile(o)
			hash = hex.EncodeToString(rp.h.Sum(nil))
		}
	case fs.ModeDir:
		err = createDir(o)
	case fs.ModeSymlink:
		err = createSymlink(o)
	default:
		err = fmt.Errorf("unsupported file type: %s", o.Path)
	}
	if err != nil {
		return nil, err
	}

	s, err := os.Lstat(o.Path)
	if err != nil {
		return nil, err
	}
	entry := &Entry{
		Path: o.Path,
		Mode: s.Mode(),
		Hash: hash,
		Size: rp.size,
		Link: o.Link,
	}
	return entry, nil
}

func createDir(o *CreateOptions) error {
	debugf("Creating directory: %s (mode %#o)", o.Path, o.Mode)
	err := os.Mkdir(o.Path, o.Mode)
	if os.IsExist(err) {
		return nil
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

func createHardLink(o *CreateOptions) error {
	debugf("Creating hard link: %s => %s", o.Path, o.Link)
	linkInfo, err := os.Lstat(o.Link)
	if err != nil && os.IsNotExist(err) {
		return fmt.Errorf("link target does not exist: %s", o.Link)
	} else if err != nil {
		return err
	}

	pathInfo, err := os.Lstat(o.Path)
	if err == nil || os.IsExist(err) {
		if os.SameFile(linkInfo, pathInfo) {
			return nil
		}
		return fmt.Errorf("path %s already exists", o.Path)
	} else if !os.IsNotExist(err) {
		return err
	}

	return os.Link(o.Link, o.Path)
}

// readerProxy implements the io.Reader interface proxying the calls to its
// inner io.Reader. On each read, the proxy keeps track of the file size and hash.
type readerProxy struct {
	inner io.Reader
	h     hash.Hash
	size  int
}

var _ io.Reader = (*readerProxy)(nil)

func (rp *readerProxy) Read(p []byte) (n int, err error) {
	n, err = rp.inner.Read(p)
	rp.h.Write(p[:n])
	rp.size += n
	return n, err
}
