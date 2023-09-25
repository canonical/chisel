package archive_test

import (
	"os"
	"path/filepath"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/archive"
)

type matchTest struct {
	url      string
	err      string
	username string
	password string
}

type credentialsTest struct {
	summary    string
	credsFiles map[string]string
	matchTests []matchTest
}

var credentialsTests = []credentialsTest{{
	summary: "Parsing",
	credsFiles: map[string]string{
		"50test-logins": `
machine example.netter login bar password foo
machine example.net login foo password bar

machine example.org:90 login apt password apt
machine	example.org:8080
login
example	password 	 foobar

machine example.org
login anonymous
password pass

machine example.com/foo login user1 unknown token password pass1
machine example.com/bar password pass2 login user2
		unknown token
machine example.com/user login user
machine example.netter login unused password firstentry
machine socks5h://example.last/debian login debian password rules
`,
	},
	matchTests: []matchTest{
		{"https://example.net/foo", "", "foo", "bar"},
		{"https://user:pass@example.net/foo", "", "user", "pass"},
		{"https://example.org:90/foo", "", "apt", "apt"},
		{"https://example.org:8080/foo", "", "example", "foobar"},
		{"https://example.net:42/foo", "", "foo", "bar"},
		{"https://example.org/foo", "", "anonymous", "pass"},
		{"https://example.com/apt", "^credentials not found$", "", ""},
		{"https://example.com/foo", "", "user1", "pass1"},
		{"https://example.com/fooo", "", "user1", "pass1"},
		{"https://example.com/fo", "^credentials not found$", "", ""},
		{"https://example.com/bar", "", "user2", "pass2"},
		{"https://example.com/user", "", "user", ""},
		{"socks5h://example.last/debian", "", "debian", "rules"},
		{"socks5h://example.debian/", "^credentials not found$", "", ""},
		{"socks5h://user:pass@example.debian/", "", "user", "pass"},
	},
}, {
	summary: "Bad file: No machine",
	credsFiles: map[string]string{
		"50test-logins.conf": `
foo example.org login foo1 password bar
machin example.org login foo2 password bar
machine2 example.org login foo3 password bar
`,
	},
	matchTests: []matchTest{
		{"https://example.org/foo", "^credentials not found$", "", ""},
	},
}, {
	summary: "Bad file: Ends machine",
	credsFiles: map[string]string{
		"50test-logins.conf": `
machine example.org login foo1 password bar
machine`,
	},
	matchTests: []matchTest{
		{"https://example.org/foo", "", "foo1", "bar"},
		{"https://example.net/foo", "^credentials not found$", "", ""},
		{"https://foo:bar@example.net/foo", "", "foo", "bar"},
	},
}, {
	summary: "Bad file: Ends login",
	credsFiles: map[string]string{
		"50test-logins.conf": `
machine example.org login foo1 password bar
machine example.net login
`,
	},
	matchTests: []matchTest{
		{"https://example.org/foo", "", "foo1", "bar"},
		{"https://example.net/foo", "^credentials not found$", "", ""},
		{"https://foo:bar@example.net/foo", "", "foo", "bar"},
	},
}, {
	summary: "Matches only HTTPS",
	credsFiles: map[string]string{
		"50test-logins.conf": `
machine https.example login foo1 password bar
machine http://http.example login foo1 password bar
`,
	},
	matchTests: []matchTest{
		{"https://https.example/foo", "", "foo1", "bar"},
		{"http://https.example/foo", "^credentials not found$", "", ""},
		{"http://http.example/foo", "", "foo1", "bar"},
		{"https://http.example/foo", "^credentials not found$", "", ""},
	},
}, {
	summary: "Password is machine",
	credsFiles: map[string]string{
		"50test-logins.conf": `
machine http://site1.com login u1 password machine
machine http://site2.com login u2 password p2
`,
	},
	matchTests: []matchTest{
		{"http://site1.com/foo", "", "u1", "machine"},
		{"http://site2.com/bar", "^credentials not found$", "", ""},
	},
}, {
	summary: "Multiple login and password tokens",
	credsFiles: map[string]string{
		"50test-logins.conf": `
machine http://site1.com login a login b password c login d password e
machine http://site2.com login f password g
`,
	},
	matchTests: []matchTest{
		{"http://site1.com/foo", "", "d", "e"},
		{"http://site2.com/bar", "", "f", "g"},
	},
}, {
	summary:    "Empty auth dir",
	credsFiles: map[string]string{},
	matchTests: []matchTest{
		{"https://example.com/foo", "^credentials not found$", "", ""},
		{"http://zombo.com", "^credentials not found$", "", ""},
	},
}, {
	summary: "Invalid input URL",
	credsFiles: map[string]string{
		"logins": `
machine login foo password bar login baz
`,
	},
	matchTests: []matchTest{
		{":http:foo", "cannot parse archive URL: parse \":http:foo\": missing protocol scheme", "", ""},
		{"", "^credentials not found$", "", ""}, // this is fine URL apparently, but won't ever match
		{"https://login", "", "baz", "bar"},
	},
}, {
	summary: "First entry wins",
	credsFiles: map[string]string{
		"logins": `
machine http://example.com/foo login a password b
machine http://example.com/foo login c password d

machine example.com/bar login e password f
machine http://example.com/bar login g password h

machine http://example.com/baz login i password j
machine http://example.com/baz/qux login k password l
`,
	},
	matchTests: []matchTest{
		{"http://example.com/foo", "", "a", "b"},
		{"http://example.com/bar", "", "g", "h"},
		{"http://example.com/baz/qux", "", "i", "j"},
	},
}, {
	summary: "First file wins",
	credsFiles: map[string]string{
		"10first": `
machine http://example.com/foo login a password b
machine example.com/bar login e password f
machine http://example.com/baz login i password j
`,
		"50second": `
machine http://example.com/foo login b password c
machine http://example.com/bar login g password h
machine http://example.com/baz/qux login k password l
`,
	},
	matchTests: []matchTest{
		{"http://example.com/foo", "", "a", "b"},
		{"http://example.com/bar", "", "g", "h"},
		{"http://example.com/baz/qux", "", "i", "j"},
	},
}, {
	summary: "EOF while epxecting username",
	credsFiles: map[string]string{
		"nouser": `
machine http://example.com/foo login
`,
	},
	matchTests: []matchTest{
		{"http://example.com/foo", "^credentials not found$", "", ""},
	},
}, {
	summary: "EOF while epxecting password",
	credsFiles: map[string]string{
		"nopw": `
machine http://example.com/foo login a password
`,
	},
	matchTests: []matchTest{
		{"http://example.com/foo", "^credentials not found$", "a", ""},
	},
}}

func (s *S) TestFindCredentialsInDir(c *C) {
	for _, t := range credentialsTests {
		s.runFindCredentialsInDirTest(c, &t)
	}
}

func (s *S) runFindCredentialsInDirTest(c *C, t *credentialsTest) {
	credsDir := c.MkDir()

	for filename, data := range t.credsFiles {
		fpath := filepath.Join(credsDir, filename)
		err := os.MkdirAll(filepath.Dir(fpath), 0755)
		c.Assert(err, IsNil)
		err = os.WriteFile(fpath, []byte(data), 0644)
		c.Assert(err, IsNil)
	}

	for _, matchTest := range t.matchTests {
		c.Logf("Summary: %s for URL %s", t.summary, matchTest.url)
		creds, err := archive.FindCredentialsInDir(matchTest.url, credsDir)
		if matchTest.err != "" {
			c.Assert(err, ErrorMatches, matchTest.err)
		} else {
			c.Assert(err, IsNil)
			c.Assert(creds, NotNil)
			c.Assert(creds.Username, Equals, matchTest.username)
			c.Assert(creds.Password, Equals, matchTest.password)
		}
	}
}

func (s *S) TestFindCredentialsInDirMissingDir(c *C) {
	var creds *archive.Credentials
	var err error

	workDir := c.MkDir()
	credsDir := filepath.Join(workDir, "auth.conf.d")

	creds, err = archive.FindCredentialsInDir("https://example.com/foo/bar", credsDir)
	c.Assert(err, ErrorMatches, "^credentials not found$")
	c.Assert(creds, IsNil)

	err = os.Mkdir(credsDir, 0755)
	c.Assert(err, IsNil)

	creds, err = archive.FindCredentialsInDir("https://example.com/foo/bar", credsDir)
	c.Assert(err, ErrorMatches, "^credentials not found$")
	c.Assert(creds, IsNil)

	confFile := filepath.Join(credsDir, "example")
	err = os.WriteFile(confFile, []byte("machine example.com login admin password swordfish"), 0600)
	c.Assert(err, IsNil)

	creds, err = archive.FindCredentialsInDir("https://example.com/foo/bar", credsDir)
	c.Assert(err, IsNil)
	c.Assert(creds, NotNil)
	c.Assert(creds.Username, Equals, "admin")
	c.Assert(creds.Password, Equals, "swordfish")
}

func fakeEnv(name, value string) (restore func()) {
	origValue, origSet := os.LookupEnv(name)
	os.Setenv(name, value)
	return func() {
		if origSet {
			os.Setenv(name, origValue)
		} else {
			os.Unsetenv(name)
		}
	}
}

func (s *S) TestFindCredentials(c *C) {
	var creds *archive.Credentials
	var err error

	workDir := c.MkDir()
	credsDir := filepath.Join(workDir, "auth.conf.d")

	restore := fakeEnv("CHISEL_AUTH_DIR", credsDir)
	defer restore()

	creds, err = archive.FindCredentials("http://example.com/my/site")
	c.Assert(err, ErrorMatches, "^credentials not found$")
	c.Assert(creds, IsNil)

	err = os.Mkdir(credsDir, 0755)
	c.Assert(err, IsNil)

	creds, err = archive.FindCredentials("http://example.com/my/site")
	c.Assert(err, ErrorMatches, "^credentials not found$")
	c.Assert(creds, IsNil)

	confFile := filepath.Join(credsDir, "mysite")
	err = os.WriteFile(confFile, []byte("machine http://example.com/my login johndoe password 12345"), 0600)
	c.Assert(err, IsNil)

	creds, err = archive.FindCredentials("http://example.com/my/site")
	c.Assert(err, IsNil)
	c.Assert(creds, NotNil)
	c.Assert(creds.Username, Equals, "johndoe")
	c.Assert(creds.Password, Equals, "12345")
}
