package archive_test

import (
	"io/ioutil"
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
		{"https://example.com/apt", "", "", ""},
		{"https://example.com/foo", "", "user1", "pass1"},
		{"https://example.com/fooo", "", "user1", "pass1"},
		{"https://example.com/fo", "", "", ""},
		{"https://example.com/bar", "", "user2", "pass2"},
		{"https://example.com/user", "", "user", ""},
		{"socks5h://example.last/debian", "", "debian", "rules"},
		{"socks5h://example.debian/", "", "", ""},
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
		{"https://example.org/foo", "", "", ""},
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
		{"https://example.net/foo", ".*\\breached end of file while expecting machine text\\b.*", "", ""},
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
		{"https://example.net/foo", ".*\\breached end of file while expecting username text\\b.*", "", ""},
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
		{"http://https.example/foo", "", "", ""},
		{"http://http.example/foo", "", "foo1", "bar"},
		{"https://http.example/foo", "", "", ""},
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
		{"http://site2.com/bar", "", "", ""},
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
		{"https://example.com/foo", "", "", ""},
		{"http://zombo.com", "", "", ""},
	},
}, {
	summary: "Invalid input URL",
	credsFiles: map[string]string{
		"logins": `
machine login foo password bar login baz
`,
	},
	matchTests: []matchTest{
		{":http:foo", "parse \":http:foo\": missing protocol scheme", "", ""},
		{"", "", "", ""}, // this is fine URL apparently, but won't ever match
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
}}

func (s *S) TestFindCredentials(c *C) {
	for _, t := range credentialsTests {
		s.runFindCredentialsTest(c, &t)
	}
}

func (s *S) runFindCredentialsTest(c *C, t *credentialsTest) {
	credsDir := c.MkDir()
	restore := archive.FakeCredentialsDir(credsDir)
	defer restore()

	for filename, data := range t.credsFiles {
		fpath := filepath.Join(credsDir, filename)
		err := os.MkdirAll(filepath.Dir(fpath), 0755)
		c.Assert(err, IsNil)
		err = ioutil.WriteFile(fpath, []byte(data), 0644)
		c.Assert(err, IsNil)
	}

	for _, matchTest := range t.matchTests {
		c.Logf("Summary: %s for URL %s", t.summary, matchTest.url)
		creds, err := archive.FindCredentials(matchTest.url)
		if matchTest.err != "" {
			c.Assert(err, ErrorMatches, matchTest.err)
		} else {
			c.Assert(err, IsNil)
		}
		c.Assert(creds.Username, Equals, matchTest.username)
		c.Assert(creds.Password, Equals, matchTest.password)
	}
}

func (s *S) TestFindCredentialsMissingDir(c *C) {
	var creds, emptyCreds archive.Credentials
	var err error

	workDir := c.MkDir()
	credsDir := filepath.Join(workDir, "auth.conf.d")
	restore := archive.FakeCredentialsDir(credsDir)
	defer restore()

	creds, err = archive.FindCredentials("https://example.com/foo/bar")
	c.Assert(err, IsNil)
	c.Assert(creds, Equals, emptyCreds)

	err = os.Mkdir(credsDir, 0755)
	c.Assert(err, IsNil)

	creds, err = archive.FindCredentials("https://example.com/foo/bar")
	c.Assert(err, IsNil)
	c.Assert(creds, Equals, emptyCreds)

	confFile := filepath.Join(credsDir, "example")
	err = os.WriteFile(confFile, []byte("machine example.com login admin password swordfish"), 0600)
	c.Assert(err, IsNil)

	creds, err = archive.FindCredentials("https://example.com/foo/bar")
	c.Assert(err, IsNil)
	c.Assert(creds, Not(Equals), emptyCreds)
	c.Assert(creds.Username, Equals, "admin")
	c.Assert(creds.Password, Equals, "swordfish")
}
