package archive

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// This file defines findCredentials() function for searching repository
// credentials in Apt configuration, see
// https://manpages.debian.org/testing/apt/apt_auth.conf.5.en.html.

// credentials represents matched Username and Password if Username is
// non-empty or unsuccessful search otherwise.
type credentials struct {
	Username string
	Password string
}

// Empty checks whether c represents unsuccessful search.
func (c credentials) Empty() bool {
	return c.Username == ""
}

// credentialsLookup contains parsed input URL data used for search.
type credentialsLookup struct {
	scheme     string
	host       string
	port       string
	path       string
	needScheme bool
}

// lookupFor parses repoUrl into credentialsLookup and fills provided credentials with
// username and password if they are specified in repoUrl.
func lookupFor(repoUrl string, creds *credentials) (*credentialsLookup, error) {
	u, err := url.Parse(repoUrl)
	if err != nil {
		return nil, err
	}
	host := u.Host
	port := u.Port()
	if port != "" {
		// u.Hostname() would remove brackets from IPv6 address but we
		// need it verbatim for string comparison in netrc file. This
		// is also a bit faster because both u.Port() and u.Hostname()
		// split u.Host into port and hostname.
		host = u.Host[0 : len(u.Host)-len(port)-1]
	}

	lookup := credentialsLookup{
		scheme:     u.Scheme,
		host:       host,
		port:       port,
		path:       u.Path,
		// If the input URL specifies unencrypted scheme, the scheme in
		// machine declarations in netrc file is not optional and must
		// also match.
		needScheme: u.Scheme != "https" && u.Scheme != "tor+https",
	}

	if creds != nil {
		creds.Username = u.User.Username()
		creds.Password, _ = u.User.Password()
	}

	return &lookup, nil
}

// findCredentials searches credentials for repoUrl in configuration files in
// directory specified by CHISEL_AUTH_DIR environment variable if it's
// non-empty or /etc/apt/auth.conf.d.
func findCredentials(repoUrl string) (credentials, error) {
	credentialsDir := "/etc/apt/auth.conf.d"
	if v := os.Getenv("CHISEL_AUTH_DIR"); v != "" {
		credentialsDir = v
	}
	return findCredentialsDir(repoUrl, credentialsDir)
}

// findCredentialsDir searches for credentials for repoUrl in configuration
// files in credentialsDir directory. If the directory does not exist, empty
// credentials structure with nil err is returned.
// Only files that do not begin with dot and have either no or ".conf"
// extension are searched. The files are searched in ascending lexicographic
// order. The first file that contains machine declaration matching repoUrl
// ends the search. If no file contain matching machine declaration, empty
// credentials structure with nil err is returned.
func findCredentialsDir(repoUrl string, credentialsDir string) (creds credentials, err error) {
	contents, err := os.ReadDir(credentialsDir)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		} else {
			err = fmt.Errorf("cannot open credentials directory: %w", err)
		}
		return
	}

	confFiles := make([]string, 0, len(contents))
	for _, entry := range contents {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		var info fs.FileInfo
		info, err = entry.Info()
		if err != nil {
			return
		}
		if !info.Mode().IsRegular() {
			continue
		}
		ext := filepath.Ext(name)
		if ext == "" || ext == ".conf" {
			confFiles = append(confFiles, name)
		}
	}
	sort.Strings(confFiles)

	lookup, err := lookupFor(repoUrl, &creds)
	if err != nil {
		return
	}

	errs := make([]error, 0, len(confFiles))

	for _, file := range confFiles {
		if !creds.Empty() {
			break
		}

		f, err := os.Open(filepath.Join(credentialsDir, file))
		if err != nil {
			errs = append(errs, fmt.Errorf("cannot read credentials file: %w", err))
			continue
		}

		if err = findCredentialsFile(lookup, f, &creds); err != nil {
			errs = append(errs, fmt.Errorf("cannot find credentials: %w", err))
		}
	}

	err = errors.Join(errs...)
	return
}

type netrcParser struct {
	lookup  *credentialsLookup
	scanner *bufio.Scanner
	creds   *credentials
}

// findCredentialsFile searches for credentials in netrc file matching lookup
// and fills creds with matched credentials if there's a match. The first match
// ends the search.
//
// The format of the netrc file is described in [1]. The parser is adapted from
// the Apt parser (see [2]). When the parser is looking for a matching machine
// declaration it disregards the current context and only considers the input
// token. For example when given the following netrc file
//
//   machine http://acme.com/foo login u1 password machine
//   machine http://acme.com/bar login u2 password p2
//
// and http://acme.com/bar input URL, the second line won't match, because the
// second "machine" will be treated as start of machine declaration. This also
// means unknown tokens are ignored, so comments are not treated specially.
//
// When a matching machine declaration is encountered the search stops when a
// next machine declaration is encountered or when end of file is reached. This
// means that arbitrary number of login and password declarations (or in fact,
// any tokens) can follow a machine declaration. The last declaration overrides
// the previous ones. For example when given the following netrc file
//
//   machine http://acme.com login a login b password c login d password e
//
// and http://acme.com input URL, the matched username and password will be "d"
// and "e" respectively.
//
// This parser diverges from the Apt parser in the following ways:
//   1. The port specification in machine declaration is optional whether or
//      not a path is specified. While the Apt documentation[1] implies the
//      same behavior, the code adheres to it only when the machine declaration
//      does not specify a path, see line 96 in [2].
//   2. When the input URL has unencrypted scheme and the machine declaration
//      does not specify a scheme, it is skipped silently. The Apt parser warns
//      the user about it, see line 113 in [2].
//
// References:
//   [1] https://manpages.debian.org/testing/apt/apt_auth.conf.5.en.html
//   [2] https://salsa.debian.org/apt-team/apt/-/blob/d9039b24/apt-pkg/contrib/netrc.cc
//   [3] https://salsa.debian.org/apt-team/apt/-/blob/4e04cbaf/methods/aptmethod.h#L560
//   [4] https://www.gnu.org/software/inetutils/manual/html_node/The-_002enetrc-file.html
//   [5] https://daniel.haxx.se/blog/2022/05/31/netrc-pains/

func findCredentialsFile(lookup *credentialsLookup, netrc io.Reader, creds *credentials) error {
	s := bufio.NewScanner(netrc)
	s.Split(bufio.ScanWords)
	p := netrcParser{
		lookup:  lookup,
		scanner: s,
		creds:   creds,
	}
	var err error
	for state := netrcInvalid; state != nil; {
		state, err = state(&p)
	}
	if err := p.scanner.Err(); err != nil {
		return err
	}
	return err
}

type netrcState func(*netrcParser) (netrcState, error)

func netrcInvalid(p *netrcParser) (netrcState, error) {
	for p.scanner.Scan() {
		if p.scanner.Text() == "machine" {
			return netrcMachine, nil
		}
	}
	return nil, nil
}

func netrcMachine(p *netrcParser) (netrcState, error) {
	if !p.scanner.Scan() {
		return nil, errors.New("syntax error: reached end of file while expecting machine text")
	}
	token := p.scanner.Text()
	if i := strings.Index(token, "://"); i != -1 {
		if token[0:i] != p.lookup.scheme {
			return netrcInvalid, nil
		}
		token = token[i+3:]
	} else if p.lookup.needScheme {
		return netrcInvalid, nil
	}
	if !strings.HasPrefix(token, p.lookup.host) {
		return netrcInvalid, nil
	}
	token = token[len(p.lookup.host):]
	if len(token) > 0 {
		if token[0] == ':' {
			if p.lookup.port == "" {
				return netrcInvalid, nil
			}
			token = token[1:]
			if !strings.HasPrefix(token, p.lookup.port) {
				return netrcInvalid, nil
			}
			token = token[len(p.lookup.port):]
		}
		if !strings.HasPrefix(p.lookup.path, token) {
			return netrcInvalid, nil
		}
	}
	return netrcGoodMachine, nil
}

func netrcGoodMachine(p *netrcParser) (netrcState, error) {
loop:
	for p.scanner.Scan() {
		switch p.scanner.Text() {
		case "login":
			return netrcUsername, nil
		case "password":
			return netrcPassword, nil
		case "machine":
			break loop
		}
	}
	return nil, nil
}

func netrcUsername(p *netrcParser) (netrcState, error) {
	if !p.scanner.Scan() {
		return nil, errors.New("syntax error: reached end of file while expecting username text")
	}
	p.creds.Username = p.scanner.Text()
	return netrcGoodMachine, nil
}

func netrcPassword(p *netrcParser) (netrcState, error) {
	if !p.scanner.Scan() {
		return nil, errors.New("syntax error: reached end of file while expecting password text")
	}
	p.creds.Password = p.scanner.Text()
	return netrcGoodMachine, nil
}
