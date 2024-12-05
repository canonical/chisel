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
	// If Link is not empty and the symlink flag is set in Mode, a symlink is
	// created. If the symlink flag is not set in Mode, a hard link is created.
	Link string
	// If MakeParents is true, missing parent directories of Path are
	// created with permissions 0755.
	MakeParents bool
	// If OverrideMode is true and entry already exists, update the mode. Does
	// not affect symlinks.
	OverrideMode bool
}

type Entry struct {
	Path     string
	Mode     fs.FileMode
	SHA256   string
	Size     int
	Link     string
	HardLink bool
}

// Create creates a filesystem entry according to the provided options and returns
// the information about the created entry.
//
// Create can return errors from the os package.
func Create(options *CreateOptions) (*Entry, error) {
	rp := &readerProxy{inner: options.Data, h: sha256.New()}
	// Use the proxy instead of the raw Reader.
	optsCopy := *options
	optsCopy.Data = rp
	o := &optsCopy

	var err error
	var hash string
	var hardLink bool
	if o.MakeParents {
		if err := os.MkdirAll(filepath.Dir(o.Path), 0755); err != nil {
			return nil, err
		}
	}

	switch o.Mode & fs.ModeType {
	case 0:
		if o.Link != "" {
			err = createHardLink(o)
			hardLink = true
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
	mode := s.Mode()
	if o.OverrideMode && mode != o.Mode && o.Link == "" {
		err := os.Chmod(o.Path, o.Mode)
		if err != nil {
			return nil, err
		}
		mode = o.Mode
	}

	entry := &Entry{
		Path:     o.Path,
		Mode:     mode,
		SHA256:   hash,
		Size:     rp.size,
		Link:     o.Link,
		HardLink: hardLink,
	}
	return entry, nil
}

// CreateWriter handles the creation of a regular file and collects the
// information recorded in Entry. The Hash and Size attributes are set on
// calling Close() on the Writer.
func CreateWriter(options *CreateOptions) (io.WriteCloser, *Entry, error) {
	if !options.Mode.IsRegular() {
		return nil, nil, fmt.Errorf("unsupported file type: %s", options.Path)
	}
	if options.MakeParents {
		if err := os.MkdirAll(filepath.Dir(options.Path), 0755); err != nil {
			return nil, nil, err
		}
	}
	file, err := os.OpenFile(options.Path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, options.Mode)
	if err != nil {
		return nil, nil, err
	}
	entry := &Entry{
		Path: options.Path,
		Mode: options.Mode,
	}
	wp := &writerProxy{
		entry: entry,
		inner: file,
		h:     sha256.New(),
		size:  0,
	}
	return wp, entry, nil
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
	err := os.Link(o.Link, o.Path)
	if err != nil && os.IsExist(err) {
		linkInfo, serr := os.Lstat(o.Link)
		if serr != nil {
			return serr
		}
		pathInfo, serr := os.Lstat(o.Path)
		if serr != nil {
			return serr
		}
		if os.SameFile(linkInfo, pathInfo) {
			return nil
		}
	}
	return err
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

// writerProxy implements the io.WriteCloser interface proxying the calls to its
// inner io.WriteCloser. On each write, the proxy keeps track of the file size
// and hash. The associated entry hash and size are updated when Close() is
// called.
type writerProxy struct {
	inner io.WriteCloser
	h     hash.Hash
	size  int
	entry *Entry
}

var _ io.WriteCloser = (*writerProxy)(nil)

func (rp *writerProxy) Write(p []byte) (n int, err error) {
	n, err = rp.inner.Write(p)
	rp.h.Write(p[:n])
	rp.size += n
	return n, err
}

func (rp *writerProxy) Close() error {
	rp.entry.SHA256 = hex.EncodeToString(rp.h.Sum(nil))
	rp.entry.Size = rp.size
	return rp.inner.Close()
}
