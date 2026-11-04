package main_test

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"time"

	. "gopkg.in/check.v1"

	chisel "github.com/canonical/chisel/cmd/chisel"
)

var cacheExpiryTests = []struct {
	value  string
	expiry time.Duration
	err    string
}{
	{value: "", expiry: 7 * 24 * time.Hour},
	{value: "0", expiry: 0},
	{value: "1", expiry: 24 * time.Hour},
	{value: "30", expiry: 30 * 24 * time.Hour},
	{value: "65535", expiry: 65535 * 24 * time.Hour},
	{value: "65536", err: `invalid CHISEL_CACHE_EXPIRY "65536", expected a number of days`},
	{value: "-1", err: `invalid CHISEL_CACHE_EXPIRY "-1", expected a number of days`},
	{value: "7d", err: `invalid CHISEL_CACHE_EXPIRY "7d", expected a number of days`},
	{value: "1.5", err: `invalid CHISEL_CACHE_EXPIRY "1.5", expected a number of days`},
	{value: " 7", err: `invalid CHISEL_CACHE_EXPIRY " 7", expected a number of days`},
}

func (s *ChiselSuite) TestCacheExpiry(c *C) {
	for _, test := range cacheExpiryTests {
		c.Logf("CHISEL_CACHE_EXPIRY=%q", test.value)
		restore := fakeEnv("CHISEL_CACHE_EXPIRY", test.value)
		expiry, err := chisel.CacheExpiry()
		restore()
		if test.err != "" {
			c.Assert(err, ErrorMatches, test.err)
			continue
		}
		c.Assert(err, IsNil)
		c.Assert(expiry, Equals, test.expiry)
	}
}

var expireCacheTests = []struct {
	value   string
	expired bool
	log     string
}{
	{value: "", expired: true},
	{value: "7", expired: true},
	{value: "30", expired: false},
	{value: "0", expired: false},
	{value: "never", expired: false, log: "Warning: invalid CHISEL_CACHE_EXPIRY \"never\", expected a number of days; the cache is not expired\n"},
}

func (s *ChiselSuite) TestExpireCache(c *C) {
	var logs bytes.Buffer
	chisel.SetLogger(log.New(&logs, "", 0))
	defer chisel.SetLogger(nil)

	for _, test := range expireCacheTests {
		c.Logf("CHISEL_CACHE_EXPIRY=%q", test.value)
		logs.Reset()

		dir := c.MkDir()
		entry := filepath.Join(dir, "sha256", "5b41362bc82b7f3d56edc5a306db22105707d01ff4819e26faef9724a2d406c9")
		err := os.MkdirAll(filepath.Dir(entry), 0755)
		c.Assert(err, IsNil)
		err = os.WriteFile(entry, []byte("data1"), 0644)
		c.Assert(err, IsNil)
		lastUsed := time.Now().Add(-8 * 24 * time.Hour)
		err = os.Chtimes(entry, lastUsed, lastUsed)
		c.Assert(err, IsNil)

		restore := fakeEnv("CHISEL_CACHE_EXPIRY", test.value)
		chisel.ExpireCache(dir)
		restore()

		_, err = os.Stat(entry)
		c.Assert(os.IsNotExist(err), Equals, test.expired)
		c.Assert(logs.String(), Equals, test.log)
	}
}
