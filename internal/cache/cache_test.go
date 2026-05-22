package cache_test

import (
	. "gopkg.in/check.v1"

	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/canonical/chisel/internal/cache"
)

const (
	data1Digest = "5b41362bc82b7f3d56edc5a306db22105707d01ff4819e26faef9724a2d406c9"
	data2Digest = "d98cf53e0c8b77c14a96358d5b69584225b4bb9026423cbc2f7b0161894c402c"
	data3Digest = "f60f2d65da046fcaaf8a10bd96b5630104b629e111aff46ce89792e1caa11b18"
)

func (s *S) TestDefaultDir(c *C) {
	oldA := os.Getenv("HOME")
	oldB := os.Getenv("XDG_CACHE_HOME")
	defer func() {
		os.Setenv("HOME", oldA)
		os.Setenv("XDG_CACHE_HOME", oldB)
	}()

	os.Setenv("HOME", "/home/user")
	os.Setenv("XDG_CACHE_HOME", "")
	c.Assert(cache.DefaultDir("foo/bar"), Equals, "/home/user/.cache/foo/bar")

	os.Setenv("HOME", "/home/user")
	os.Setenv("XDG_CACHE_HOME", "/xdg/cache")
	c.Assert(cache.DefaultDir("foo/bar"), Equals, "/xdg/cache/foo/bar")

	os.Setenv("HOME", "")
	os.Setenv("XDG_CACHE_HOME", "")
	defaultDir := cache.DefaultDir("foo/bar")
	c.Assert(strings.HasPrefix(defaultDir, os.TempDir()), Equals, true)
	c.Assert(strings.Contains(defaultDir, "/cache-"), Equals, true)
	c.Assert(strings.HasSuffix(defaultDir, "/foo/bar"), Equals, true)
}

func (s *S) TestCacheEmpty(c *C) {
	cc := cache.Cache{c.MkDir()}

	_, err := cc.Open(cache.SHA256, data1Digest)
	c.Assert(err, Equals, cache.MissErr)
	_, err = cc.Read(cache.SHA256, data1Digest)
	c.Assert(err, Equals, cache.MissErr)
	_, err = cc.Read(cache.SHA256, "")
	c.Assert(err, Equals, cache.MissErr)
}

func (s *S) TestCacheReadWrite(c *C) {
	cc := cache.Cache{Dir: c.MkDir()}

	data1Path := filepath.Join(cc.Dir, "sha256", data1Digest)
	data2Path := filepath.Join(cc.Dir, "sha256", data2Digest)
	data3Path := filepath.Join(cc.Dir, "sha256", data3Digest)

	err := cc.Write(cache.SHA256, data1Digest, []byte("data1"))
	c.Assert(err, IsNil)
	data1, err := cc.Read(cache.SHA256, data1Digest)
	c.Assert(err, IsNil)
	c.Assert(string(data1), Equals, "data1")

	err = cc.Write(cache.SHA256, "", []byte("data2"))
	c.Assert(err, IsNil)
	data2, err := cc.Read(cache.SHA256, data2Digest)
	c.Assert(err, IsNil)
	c.Assert(string(data2), Equals, "data2")

	_, err = cc.Read(cache.SHA256, data3Digest)
	c.Assert(err, Equals, cache.MissErr)
	_, err = cc.Read(cache.SHA256, "")
	c.Assert(err, Equals, cache.MissErr)

	_, err = os.Stat(data1Path)
	c.Assert(err, IsNil)
	_, err = os.Stat(data2Path)
	c.Assert(err, IsNil)
	_, err = os.Stat(data3Path)
	c.Assert(os.IsNotExist(err), Equals, true)

	now := time.Now()
	expired := now.Add(-time.Hour - time.Second)
	err = os.Chtimes(data1Path, now, expired)
	c.Assert(err, IsNil)

	err = cc.Expire(time.Hour)
	c.Assert(err, IsNil)
	_, err = os.Stat(data1Path)
	c.Assert(os.IsNotExist(err), Equals, true)
}

func (s *S) TestCacheCreate(c *C) {
	cc := cache.Cache{Dir: c.MkDir()}

	w := cc.Create(cache.SHA256, "")

	c.Assert(w.Digest(), Equals, "")

	_, err := w.Write([]byte("da"))
	c.Assert(err, IsNil)
	_, err = w.Write([]byte("ta"))
	c.Assert(err, IsNil)
	_, err = w.Write([]byte("1"))
	c.Assert(err, IsNil)
	err = w.Close()
	c.Assert(err, IsNil)

	c.Assert(w.Digest(), Equals, data1Digest)

	data1, err := cc.Read(cache.SHA256, data1Digest)
	c.Assert(err, IsNil)
	c.Assert(string(data1), Equals, "data1")
}

func (s *S) TestCacheWrongDigest(c *C) {
	cc := cache.Cache{Dir: c.MkDir()}

	w := cc.Create(cache.SHA256, data1Digest)

	c.Assert(w.Digest(), Equals, data1Digest)

	_, err := w.Write([]byte("data2"))
	errClose := w.Close()
	c.Assert(err, IsNil)
	c.Assert(errClose, ErrorMatches, "expected digest "+data1Digest+", got "+data2Digest)

	_, err = cc.Read(cache.SHA256, data1Digest)
	c.Assert(err, Equals, cache.MissErr)
	_, err = cc.Read(cache.SHA256, data2Digest)
	c.Assert(err, Equals, cache.MissErr)
}

func (s *S) TestCacheOpen(c *C) {
	cc := cache.Cache{Dir: c.MkDir()}

	err := cc.Write(cache.SHA256, data1Digest, []byte("data1"))
	c.Assert(err, IsNil)

	f, err := cc.Open(cache.SHA256, data1Digest)
	c.Assert(err, IsNil)
	data1, err := io.ReadAll(f)
	closeErr := f.Close()
	c.Assert(err, IsNil)
	c.Assert(closeErr, IsNil)

	c.Assert(string(data1), Equals, "data1")
}

func (s *S) TestCacheSHA3384(c *C) {
	cc := cache.Cache{Dir: c.MkDir()}

	w := cc.Create(cache.SHA3384, "")
	_, err := w.Write([]byte("data1"))
	c.Assert(err, IsNil)
	err = w.Close()
	c.Assert(err, IsNil)
	sha3Digest := w.Digest()

	c.Assert(sha3Digest, Not(Equals), data1Digest)

	data, err := cc.Read(cache.SHA3384, sha3Digest)
	c.Assert(err, IsNil)
	c.Assert(string(data), Equals, "data1")

	// SHA3-384 entry must not be visible under SHA-256.
	_, err = cc.Open(cache.SHA256, sha3Digest)
	c.Assert(err, Equals, cache.MissErr)

	// SHA-256 entry must not be visible under SHA3-384.
	err = cc.Write(cache.SHA256, data1Digest, []byte("data1"))
	c.Assert(err, IsNil)
	_, err = cc.Open(cache.SHA3384, data1Digest)
	c.Assert(err, Equals, cache.MissErr)
}
