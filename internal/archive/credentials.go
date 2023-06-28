package archive

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// credentials contains matched non-empty Username and Password.
// Username is left empty if the search is unsuccessful.
type credentials struct {
	Username string
	Password string
}

// Empty checks whether c represents unsuccessful search.
func (c credentials) Empty() bool {
	return c.Username == ""
}

// credentialsQuery contains parsed input URL data used for search.
type credentialsQuery struct {
	scheme     string
	host       string
	port       string
	path       string
	needScheme bool
}

// parseRepoURL parses repoURL into credentialsQuery and fills provided
// credentials with username and password if they are specified in repoURL.
func parseRepoURL(repoURL string) (creds credentials, query *credentialsQuery, err error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return
	}

	creds.Username = u.User.Username()
	creds.Password, _ = u.User.Password()

	if !creds.Empty() {
		return
	}

	host := u.Host
	port := u.Port()
	if port != "" {
		// u.Hostname() would remove brackets from IPv6 address but we
		// need it verbatim for string search in netrc file. This is
		// also faster because both u.Port() and u.Hostname() parse
		// u.Host into port and hostname.
		host = u.Host[0 : len(u.Host)-len(port)-1]
	}

	query = &credentialsQuery{
		scheme: u.Scheme,
		host:   host,
		port:   port,
		path:   u.Path,
		// If the input URL specifies unencrypted scheme, the scheme in
		// machine declarations in netrc file is not optional and must
		// also match.
		needScheme: u.Scheme != "https" && u.Scheme != "tor+https",
	}

	return
}

// findCredentials searches credentials for repoURL in configuration files in
// directory specified by CHISEL_AUTH_DIR environment variable if it's
// non-empty or /etc/apt/auth.conf.d.
func findCredentials(repoURL string) (credentials, error) {
	credsDir := "/etc/apt/auth.conf.d"
	if v := os.Getenv("CHISEL_AUTH_DIR"); v != "" {
		credsDir = v
	}
	return findCredentialsInDir(repoURL, credsDir)
}

// findCredentialsInDir searches for credentials for repoURL in configuration
// files in credsDir directory. If the directory does not exist, empty
// credentials structure with nil err is returned.
// Only files that do not begin with dot and have either no or ".conf"
// extension are searched. The files are searched in ascending lexicographic
// order. The first file that contains machine declaration matching repoURL
// ends the search. If no file contain matching machine declaration, empty
// credentials structure with nil err is returned.
func findCredentialsInDir(repoURL string, credsDir string) (creds credentials, err error) {
	contents, err := os.ReadDir(credsDir)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		} else {
			err = fmt.Errorf("cannot open credentials directory: %w", err)
		}
		return
	}

	creds, query, err := parseRepoURL(repoURL)
	if err != nil {
		err = fmt.Errorf("cannot parse archive URL: %w", err)
		return
	}
	if query == nil { // creds.Empty() == false
		return
	}

	errs := make([]error, 0)

	confFiles := make([]string, 0, len(contents))
	for _, entry := range contents {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			errs = append(errs, fmt.Errorf("cannot stat credentials file: %w", err))
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		ext := filepath.Ext(name)
		if ext == "" || ext == ".conf" {
			confFiles = append(confFiles, name)
		}
	}
	if len(confFiles) == 0 {
		err = errors.Join(errs...)
		return
	}
	sort.Strings(confFiles)

	for _, file := range confFiles {
		fpath := filepath.Join(credsDir, file)
		f, err := os.Open(fpath)
		if err != nil {
			errs = append(errs, fmt.Errorf("cannot read credentials file %s: %w", fpath, err))
			continue
		}

		if err = findCredsInFile(query, f, &creds); err != nil {
			errs = append(errs, fmt.Errorf("cannot parse credentials file %s: %w", fpath, err))
		} else if !creds.Empty() {
			break
		}
	}

	err = errors.Join(errs...)
	return
}

type netrcParser struct {
	query   *credentialsQuery
	scanner *bufio.Scanner
	creds   *credentials
}

// findCredsInFile searches for credentials in netrc file matching query
// and fills creds with matched credentials if there's a match. The first match
// ends the search.
//
// The format of the netrc file is described in [1]. The parser is adapted from
// the Apt parser (see [2]). When the parser is looking for a matching machine
// declaration it disregards the current context and only considers the input
// token. For example when given the following netrc file
//
//	machine http://acme.com/foo login u1 password machine
//	machine http://acme.com/bar login u2 password p2
//
// and http://acme.com/bar input URL, the second line won't match, because the
// second "machine" will be treated as start of machine declaration. This also
// means unknown tokens are ignored, so comments are not treated specially.
//
// When a matching machine declaration is found the search stops on next
// machine token or on end of file. This means that arbitrary number of login
// and password declarations (or in fact, any tokens) can follow a machine
// declaration. The last username and password declaration overrides the
// previous ones. For example when given the following netrc file
//
//	machine http://acme.com login a foo login b password c bar login d password e
//
// and the input URL is http://acme.com, the matched username and password will
// be "d" and "e" respectively. Tokens foo and bar will be ignored.
//
// This parser diverges from the Apt parser in the following ways:
//  1. The port specification in machine declaration is optional whether or
//     not a path is specified. While the Apt documentation[1] implies the
//     same behavior, the code adheres to it only when the machine declaration
//     does not specify a path, see line 96 in [2].
//  2. When the input URL has unencrypted scheme and the machine declaration
//     does not specify a scheme, it is skipped silently. The Apt parser warns
//     the user about it, see line 113 in [2].
//
// References:
//
//	[1] https://manpages.debian.org/testing/apt/apt_auth.conf.5.en.html
//	[2] https://salsa.debian.org/apt-team/apt/-/blob/d9039b24/apt-pkg/contrib/netrc.cc
//	[3] https://salsa.debian.org/apt-team/apt/-/blob/4e04cbaf/methods/aptmethod.h#L560
//	[4] https://www.gnu.org/software/inetutils/manual/html_node/The-_002enetrc-file.html
//	[5] https://daniel.haxx.se/blog/2022/05/31/netrc-pains/
func findCredsInFile(query *credentialsQuery, netrc io.Reader, creds *credentials) error {
	s := bufio.NewScanner(netrc)
	s.Split(bufio.ScanWords)
	p := netrcParser{
		query:   query,
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
		if token[0:i] != p.query.scheme {
			return netrcInvalid, nil
		}
		token = token[i+3:]
	} else if p.query.needScheme {
		return netrcInvalid, nil
	}
	if !strings.HasPrefix(token, p.query.host) {
		return netrcInvalid, nil
	}
	token = token[len(p.query.host):]
	if len(token) > 0 {
		if token[0] == ':' {
			if p.query.port == "" {
				return netrcInvalid, nil
			}
			token = token[1:]
			if !strings.HasPrefix(token, p.query.port) {
				return netrcInvalid, nil
			}
			token = token[len(p.query.port):]
		}
		if !strings.HasPrefix(p.query.path, token) {
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
