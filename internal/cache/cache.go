package cache

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"golang.org/x/crypto/sha3"
)

func DefaultDir(suffix string) string {
	cacheDir := os.Getenv("XDG_CACHE_HOME")
	if cacheDir == "" {
		homeDir := os.Getenv("HOME")
		if homeDir != "" {
			cacheDir = filepath.Join(homeDir, ".cache")
		} else {
			var err error
			cacheDir, err = os.MkdirTemp("", "cache-*")
			if err != nil {
				panic("no proper location for cache: " + err.Error())
			}
		}
	}
	return filepath.Join(cacheDir, suffix)
}

type Cache struct {
	Dir string
}

type Writer struct {
	dir    string
	digest string
	hash   hash.Hash
	file   *os.File
	err    error
}

func (cw *Writer) fail(err error) error {
	if cw.err == nil {
		cw.err = err
		cw.file.Close()
		os.Remove(cw.file.Name())
	}
	return err
}

func (cw *Writer) Write(data []byte) (n int, err error) {
	if cw.err != nil {
		return 0, cw.err
	}
	n, err = cw.file.Write(data)
	if err != nil {
		return n, cw.fail(err)
	}
	cw.hash.Write(data)
	return n, nil
}

func (cw *Writer) Close() error {
	if cw.err != nil {
		return cw.err
	}
	err := cw.file.Close()
	if err != nil {
		return cw.fail(err)
	}
	sum := cw.hash.Sum(nil)
	digest := hex.EncodeToString(sum[:])
	if cw.digest == "" {
		cw.digest = digest
	} else if digest != cw.digest {
		return cw.fail(fmt.Errorf("expected digest %s, got %s", cw.digest, digest))
	}
	fname := cw.file.Name()
	err = os.Rename(fname, filepath.Join(filepath.Dir(fname), cw.digest))
	if err != nil {
		return cw.fail(err)
	}
	cw.err = io.EOF
	return nil
}

func (cw *Writer) Digest() string {
	return cw.digest
}

type DigestKind string

const (
	SHA256 DigestKind = "sha256"
	SHA384 DigestKind = "sha384"
	SHA512 DigestKind = "sha512"
)

var digestKinds = []DigestKind{SHA256, SHA384, SHA512}

// ValidateKind returns an error unless kind is a digest kind Chisel
// supports.
func ValidateKind(kind DigestKind) error {
	if !slices.Contains(digestKinds, kind) {
		return fmt.Errorf("unsupported digest kind: %q", kind)
	}
	return nil
}

var ErrMiss = fmt.Errorf("not cached")

func (c *Cache) filePath(digestKind DigestKind, digest string) string {
	return filepath.Join(c.Dir, string(digestKind), digest)
}

func (c *Cache) Create(digestKind DigestKind, digest string) *Writer {
	if c.Dir == "" {
		return &Writer{err: fmt.Errorf("internal error: cache directory is unset")}
	}

	var h hash.Hash
	switch digestKind {
	case SHA256:
		h = sha256.New()
	case SHA384:
		h = sha3.New384()
	case SHA512:
		h = sha512.New()
	default:
		return &Writer{err: fmt.Errorf("internal error: unsupported digest kind: %q", digestKind)}
	}
	err := os.MkdirAll(filepath.Join(c.Dir, string(digestKind)), 0755)
	if err != nil {
		return &Writer{err: fmt.Errorf("cannot create cache directory: %v", err)}
	}
	var file *os.File
	if digest == "" {
		file, err = os.CreateTemp(c.filePath(digestKind, ""), "tmp.*")
	} else {
		file, err = os.Create(c.filePath(digestKind, digest+".tmp"))
	}
	if err != nil {
		return &Writer{err: fmt.Errorf("cannot create cache file: %v", err)}
	}
	return &Writer{
		dir:    c.Dir,
		digest: digest,
		hash:   h,
		file:   file,
	}
}

func (c *Cache) Write(digestKind DigestKind, digest string, data []byte) error {
	f := c.Create(digestKind, digest)
	_, err1 := f.Write(data)
	err2 := f.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

func (c *Cache) Open(digestKind DigestKind, digest string) (io.ReadSeekCloser, error) {
	if c.Dir == "" || digest == "" {
		return nil, ErrMiss
	}
	filePath := c.filePath(digestKind, digest)
	file, err := os.Open(filePath)
	if os.IsNotExist(err) {
		return nil, ErrMiss
	} else if err != nil {
		return nil, fmt.Errorf("cannot open cache file: %v", err)
	}
	// Use mtime as last reuse time.
	now := time.Now()
	if err := os.Chtimes(filePath, now, now); err != nil {
		return nil, fmt.Errorf("cannot update cached file timestamp: %v", err)
	}
	return file, nil
}

func (c *Cache) Read(digestKind DigestKind, digest string) ([]byte, error) {
	file, err := c.Open(digestKind, digest)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("cannot read file from cache: %v", err)
	}
	return data, nil
}

func (c *Cache) Expire(timeout time.Duration) error {
	expired := time.Now().Add(-timeout)
	for _, digestKind := range digestKinds {
		digestKindDir := filepath.Join(c.Dir, string(digestKind))
		entries, err := os.ReadDir(digestKindDir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("cannot list cache directory: %v", err)
		}
		for _, entry := range entries {
			finfo, err := entry.Info()
			if err != nil {
				return err
			}
			if finfo.ModTime().After(expired) {
				continue
			}
			err = os.Remove(filepath.Join(digestKindDir, finfo.Name()))
			if err != nil {
				return fmt.Errorf("cannot expire cache entry: %v", err)
			}
		}
	}
	return nil
}
