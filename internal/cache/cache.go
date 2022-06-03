package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
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

const digestKind = "sha256"

var MissErr = fmt.Errorf("not cached")

func (c *Cache) filePath(digest string) string {
	return filepath.Join(c.Dir, digestKind, digest)
}

func (c *Cache) Create(digest string) *Writer {
	if c.Dir == "" {
		return &Writer{err: fmt.Errorf("internal error: cache directory is unset")}
	}
	err := os.MkdirAll(filepath.Join(c.Dir, digestKind), 0755)
	if err != nil {
		return &Writer{err: fmt.Errorf("cannot create cache directory: %v", err)}
	}
	var file *os.File
	if digest == "" {
		file, err = os.CreateTemp(c.filePath(""), "tmp.*")
	} else {
		file, err = os.Create(c.filePath(digest + ".tmp"))
	}
	if err != nil {
		return &Writer{err: fmt.Errorf("cannot create cache file: %v", err)}
	}
	return &Writer{
		dir:    c.Dir,
		digest: digest,
		hash:   sha256.New(),
		file:   file,
	}
}

func (c *Cache) Write(digest string, data []byte) error {
	f := c.Create(digest)
	_, err1 := f.Write(data)
	err2 := f.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

func (c *Cache) Open(digest string) (io.ReadCloser, error) {
	if c.Dir == "" || digest == "" {
		return nil, MissErr
	}
	filePath := c.filePath(digest)
	file, err := os.Open(filePath)
	if os.IsNotExist(err) {
		return nil, MissErr
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

func (c *Cache) Read(digest string) ([]byte, error) {
	file, err := c.Open(digest)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("cannot read file from cache: %v", err)
	}
	return data, nil
}

func (c *Cache) Expire(timeout time.Duration) error {
	list, err := ioutil.ReadDir(filepath.Join(c.Dir, digestKind))
	if err != nil {
		return fmt.Errorf("cannot list cache directory: %v", err)
	}
	expired := time.Now().Add(-timeout)
	for _, finfo := range list {
		if finfo.ModTime().After(expired) {
			continue
		}
		err = os.Remove(filepath.Join(c.Dir, digestKind, finfo.Name()))
		if err != nil {
			return fmt.Errorf("cannot expire cache entry: %v", err)
		}
	}
	return nil
}
