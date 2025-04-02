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
	"strings"
)

type CreateOptions struct {
	Root string
	// Path is relative to Root.
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
	Path   string
	Mode   fs.FileMode
	SHA256 string
	Size   int
	Link   string
}

// Create creates a filesystem entry according to the provided options and returns
// the information about the created entry.
//
// Create can return errors from the os package.
func Create(options *CreateOptions) (*Entry, error) {
	o, err := getValidOptions(options)
	if err != nil {
		return nil, err
	}

	rp := &readerProxy{inner: options.Data, h: sha256.New()}
	// Use the proxy instead of the raw Reader.
	o.Data = rp

	path, err := absPath(o.Root, o.Path)
	if err != nil {
		return nil, err
	}

	var hash string
	if o.MakeParents {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}
	}

	switch o.Mode & fs.ModeType {
	case 0:
		if o.Link != "" {
			o.Link = filepath.Clean(o.Link)
			if filepath.IsAbs(o.Link) {
				if !strings.HasPrefix(filepath.Clean(o.Link), o.Root) {
					return nil, fmt.Errorf("invalid hardlink %s target: %s is outside of root %s", path, o.Link, o.Root)
				}
			}
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
		err = fmt.Errorf("unsupported file type: %s", path)
	}
	if err != nil {
		return nil, err
	}

	// Entry should describe the created file, not the target the link points to.
	s, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	mode := s.Mode()
	if o.Link != "" {
		if options.Mode.IsRegular() {
			// Hard link.
			// In the case where the hard link points to a symlink the entry
			// should identify the created file and not the symlink. A hard link
			// is identified by the mode being regular and link not empty.
			mode = mode &^ fs.ModeSymlink
		}
	} else if o.OverrideMode && mode != o.Mode {
		err := os.Chmod(path, o.Mode)
		if err != nil {
			return nil, err
		}
		mode = o.Mode
	}

	entry := &Entry{
		Path:   path,
		Mode:   mode,
		SHA256: hash,
		Size:   rp.size,
		Link:   o.Link,
	}
	return entry, nil
}

// CreateWriter handles the creation of a regular file and collects the
// information recorded in Entry. The Hash and Size attributes are set on
// calling Close() on the Writer.
func CreateWriter(options *CreateOptions) (io.WriteCloser, *Entry, error) {
	o, err := getValidOptions(options)
	if err != nil {
		return nil, nil, err
	}

	path, err := absPath(o.Root, o.Path)
	if err != nil {
		return nil, nil, err
	}

	if !o.Mode.IsRegular() {
		return nil, nil, fmt.Errorf("unsupported file type: %s", path)
	}
	if o.MakeParents {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, nil, err
		}
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, o.Mode)
	if err != nil {
		return nil, nil, err
	}
	entry := &Entry{
		Path: path,
		Mode: o.Mode,
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
	path, err := absPath(o.Root, o.Path)
	if err != nil {
		return err
	}
	err = os.Mkdir(path, o.Mode)
	if os.IsExist(err) {
		return nil
	}
	return err
}

func createFile(o *CreateOptions) error {
	debugf("Writing file: %s (mode %#o)", o.Path, o.Mode)
	path, err := absPath(o.Root, o.Path)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, o.Mode)
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
	path, err := absPath(o.Root, o.Path)
	if err != nil {
		return err
	}
	fileinfo, err := os.Lstat(path)
	if err == nil {
		if (fileinfo.Mode() & os.ModeSymlink) != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if link == o.Link {
				return nil
			}
		}
		err = os.Remove(path)
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(o.Link, path)
}

func createHardLink(o *CreateOptions) error {
	debugf("Creating hard link: %s => %s", o.Path, o.Link)
	path, err := absPath(o.Root, o.Path)
	if err != nil {
		return err
	}
	err = os.Link(o.Link, path)
	if err != nil && os.IsExist(err) {
		linkInfo, serr := os.Lstat(o.Link)
		if serr != nil {
			return serr
		}
		pathInfo, serr := os.Lstat(path)
		if serr != nil {
			return serr
		}
		if os.SameFile(linkInfo, pathInfo) {
			return nil
		}
	}
	return err
}

func getValidOptions(options *CreateOptions) (*CreateOptions, error) {
	optsCopy := *options
	o := &optsCopy
	if o.Root == "" {
		return nil, fmt.Errorf("internal error: CreateOptions.Root is unset")
	}
	if o.Root != "/" {
		o.Root = filepath.Clean(o.Root) + "/"
	}
	return o, nil
}

// absPath requires root to be a clean path that ends in "/".
func absPath(root, relPath string) (string, error) {
	path := filepath.Clean(filepath.Join(root, relPath))
	if !strings.HasPrefix(path, root) {
		return "", fmt.Errorf("cannot create path %s outside of root %s", path, root)
	}
	return path, nil
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
