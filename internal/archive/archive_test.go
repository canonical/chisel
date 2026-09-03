package archive_test

import (
	"golang.org/x/crypto/openpgp/packet"
	. "gopkg.in/check.v1"

	"crypto/sha256"
	"crypto/sha512"
	"debug/elf"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/archive/testarchive"
	"github.com/canonical/chisel/internal/deb"
	"github.com/canonical/chisel/internal/tarball"
	"github.com/canonical/chisel/internal/testutil"
)

type requestResult struct {
	path   string
	status int
}

type httpSuite struct {
	logf           func(string, ...any)
	base           string
	request        *http.Request
	requests       []*http.Request
	requestResults []requestResult
	response       string
	responses      map[string][]byte
	err            error
	header         http.Header
	status         int
	restore        func()
	privKey        *packet.PrivateKey
	pubKey         *packet.PublicKey
}

var _ = Suite(&httpSuite{})

var (
	key1            = testutil.PGPKeys["key1"]
	key2            = testutil.PGPKeys["key2"]
	keyUbuntu2018   = testutil.PGPKeys["key-ubuntu-2018"]
	keyUbuntuFIPSv1 = testutil.PGPKeys["key-ubuntu-fips-v1"]
	keyUbuntuApps   = testutil.PGPKeys["key-ubuntu-apps"]
	keyUbuntuESMv2  = testutil.PGPKeys["key-ubuntu-esm-v2"]
)

func (s *httpSuite) SetUpTest(c *C) {
	s.logf = c.Logf
	s.err = nil
	s.base = "http://archive.ubuntu.com/ubuntu/"
	s.request = nil
	s.requests = nil
	s.requestResults = nil
	s.response = ""
	s.responses = make(map[string][]byte)
	s.header = nil
	s.status = 200
	s.restore = archive.FakeDo(s.Do)
	s.privKey = key1.PrivKey
	s.pubKey = key1.PubKey
}

func (s *httpSuite) TearDownTest(c *C) {
	s.restore()
}

func (s *httpSuite) Do(req *http.Request) (*http.Response, error) {
	if s.base != "" && !strings.HasPrefix(req.URL.String(), s.base) {
		return nil, fmt.Errorf("test expected base %q, got %q", s.base, req.URL.String())
	}

	cleanURL, err := url.JoinPath(req.URL.String())
	if err != nil {
		return nil, fmt.Errorf("cannot clean requested URL: %v", err)
	}
	if cleanURL != req.URL.String() {
		return nil, fmt.Errorf("test expected clean URL %q, got %q", cleanURL, req.URL.String())
	}

	s.request = req
	s.requests = append(s.requests, req)
	body := s.response
	status := s.status
	s.logf("Request: %s", req.URL.String())
	if response, ok := s.responses[path.Clean(req.URL.Path)]; ok {
		body = string(response)
	} else if len(s.responses) > 0 && s.status == 200 {
		// Unknown path with responses populated: behave like a real archive.
		status = 404
	}
	s.requestResults = append(s.requestResults, requestResult{path: req.URL.Path, status: status})
	rsp := &http.Response{
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     s.header,
		StatusCode: status,
	}
	return rsp, s.err
}

func (s *httpSuite) TestDoError(c *C) {
	s.err = errors.New("BAM")

	options := archive.Options{
		Label:      "ubuntu",
		Version:    "22.04",
		Arch:       "amd64",
		Suites:     []string{"jammy"},
		Components: []string{"main"},
		CacheDir:   c.MkDir(),
	}

	_, err := archive.Open(&options)
	c.Check(err, ErrorMatches, "cannot talk to archive: BAM")
}

func (s *httpSuite) prepareArchive(suite, version, arch string, components []string) *testarchive.Release {
	return s.prepareArchiveAdjustRelease(suite, version, arch, components, []string{"SHA256"}, nil)
}

func (s *httpSuite) prepareArchiveAdjustRelease(suite, version, arch string, components []string, digestKinds []string, adjustRelease func(*testarchive.Release)) *testarchive.Release {
	release := &testarchive.Release{
		Suite:       suite,
		Version:     version,
		Label:       "Ubuntu",
		PrivKey:     s.privKey,
		DigestKinds: digestKinds,
	}
	for i, component := range components {
		index := &testarchive.PackageIndex{
			Component: component,
			Arch:      arch,
		}
		for j := range 2 {
			seq := 1 + i*2 + j
			index.Packages = append(index.Packages, &testarchive.Package{
				Name:        fmt.Sprintf("mypkg%d", seq),
				Version:     fmt.Sprintf("1.%d", seq),
				Arch:        arch,
				Component:   component,
				DigestKinds: digestKinds,
			})
		}
		release.Items = append(release.Items, index)
		release.Items = append(release.Items, &testarchive.Gzip{index})
	}
	base, err := url.Parse(s.base)
	if err != nil {
		panic(err)
	}
	if adjustRelease != nil {
		adjustRelease(release)
	}
	err = release.Render(base.Path, s.responses)
	if err != nil {
		panic(err)
	}
	return release
}

type optionErrorTest struct {
	options archive.Options
	error   string
}

var optionErrorTests = []optionErrorTest{{
	options: archive.Options{
		Label:      "ubuntu",
		Version:    "22.04",
		Arch:       "amd64",
		Suites:     []string{"jammy"},
		Components: []string{"main", "other"},
	},
	error: `archive has no component "other"`,
}, {
	options: archive.Options{
		Label:   "ubuntu",
		Version: "22.04",
		Arch:    "amd64",
		Suites:  []string{"jammy"},
	},
	error: "archive options missing components",
}, {
	options: archive.Options{
		Label:      "ubuntu",
		Version:    "22.04",
		Arch:       "amd64",
		Components: []string{"main", "other"},
	},
	error: `archive options missing suites`,
}, {
	options: archive.Options{
		Label:      "ubuntu",
		Version:    "22.04",
		Arch:       "foo",
		Suites:     []string{"jammy"},
		Components: []string{"main", "other"},
	},
	error: `invalid package architecture: foo`,
}, {
	options: archive.Options{
		Label:      "ubuntu",
		Version:    "22.04",
		Arch:       "amd64",
		Suites:     []string{"jammy"},
		Components: []string{"main", "other"},
		Pro:        "invalid",
	},
	error: `invalid pro value: "invalid"`,
}}

func (s *httpSuite) TestOptionErrors(c *C) {
	s.prepareArchive("jammy", "22.04", "arm64", []string{"main", "universe"})
	cacheDir := c.MkDir()
	for _, test := range optionErrorTests {
		test.options.CacheDir = cacheDir
		test.options.PubKeys = append(test.options.PubKeys, s.pubKey)
		_, err := archive.Open(&test.options)
		c.Assert(err, ErrorMatches, test.error)
	}
}

func (s *httpSuite) TestFetchPackage(c *C) {

	s.prepareArchive("jammy", "22.04", "amd64", []string{"main", "universe"})

	options := archive.Options{
		Label:      "ubuntu",
		Version:    "22.04",
		Arch:       "amd64",
		Suites:     []string{"jammy"},
		Components: []string{"main", "universe"},
		CacheDir:   c.MkDir(),
		PubKeys:    []*packet.PublicKey{s.pubKey},
	}

	testArchive, err := archive.Open(&options)
	c.Assert(err, IsNil)

	// First on component main.
	pkg, info, err := testArchive.Fetch("mypkg1")
	c.Assert(err, IsNil)
	c.Assert(info, DeepEquals, &archive.PackageInfo{
		Name:    "mypkg1",
		Version: "1.1",
		Arch:    "amd64",
		SHA256:  "1f08ef04cfe7a8087ee38a1ea35fa1810246648136c3c42d5a61ad6503d85e05",
	})
	c.Assert(read(pkg), Equals, "mypkg1 1.1 data")

	// Last on component universe.
	pkg, info, err = testArchive.Fetch("mypkg4")
	c.Assert(err, IsNil)
	c.Assert(info, DeepEquals, &archive.PackageInfo{
		Name:    "mypkg4",
		Version: "1.4",
		Arch:    "amd64",
		SHA256:  "54af70097b30b33cfcbb6911ad3d0df86c2d458928169e348fa7873e4fc678e4",
	})
	c.Assert(read(pkg), Equals, "mypkg4 1.4 data")
}

func (s *httpSuite) TestFetchSHA512Digests(c *C) {
	// Ubuntu 26.10+ publishes SHA512-only indices (no SHA256 section), so both
	// the index digest and the package digest must be read from SHA512.
	s.prepareArchiveAdjustRelease("stonking", "25.10", "amd64", []string{"main", "universe"},
		[]string{"SHA512"}, nil)

	options := archive.Options{
		Label:      "ubuntu",
		Version:    "25.10",
		Arch:       "amd64",
		Suites:     []string{"stonking"},
		Components: []string{"main", "universe"},
		CacheDir:   c.MkDir(),
		PubKeys:    []*packet.PublicKey{s.pubKey},
	}

	testArchive, err := archive.Open(&options)
	c.Assert(err, IsNil)

	pkg, _, err := testArchive.Fetch("mypkg1")
	c.Assert(err, IsNil)
	c.Assert(read(pkg), Equals, "mypkg1 1.1 data")
}

func (s *httpSuite) TestFetchBothDigests(c *C) {
	// An archive publishing both SHA256 and SHA512 sections (index table and
	// package fields) must be handled, with the strongest digest preferred
	// for verification and caching. PackageInfo.SHA256 still surfaces: it is
	// read from the package section directly, not from the preference order.
	s.prepareArchiveAdjustRelease("stonking", "25.10", "amd64", []string{"main", "universe"},
		[]string{"SHA256", "SHA512"}, nil)

	options := archive.Options{
		Label:      "ubuntu",
		Version:    "25.10",
		Arch:       "amd64",
		Suites:     []string{"stonking"},
		Components: []string{"main", "universe"},
		CacheDir:   c.MkDir(),
		PubKeys:    []*packet.PublicKey{s.pubKey},
	}

	testArchive, err := archive.Open(&options)
	c.Assert(err, IsNil)

	pkg, info, err := testArchive.Fetch("mypkg1")
	c.Assert(err, IsNil)
	c.Assert(info, DeepEquals, &archive.PackageInfo{
		Name:    "mypkg1",
		Version: "1.1",
		Arch:    "amd64",
		SHA256:  "1f08ef04cfe7a8087ee38a1ea35fa1810246648136c3c42d5a61ad6503d85e05",
	})
	c.Assert(read(pkg), Equals, "mypkg1 1.1 data")

	// Pin the cache key: with both digests advertised, the package is cached
	// under its strongest digest.
	sha512Digest := fmt.Sprintf("%x", sha512.Sum512([]byte("mypkg1 1.1 data")))
	_, err = os.Stat(filepath.Join(options.CacheDir, "sha512", sha512Digest))
	c.Assert(err, IsNil)
}

func (s *httpSuite) TestFetchPortsPackage(c *C) {

	s.base = "http://ports.ubuntu.com/ubuntu-ports/"

	s.prepareArchive("jammy", "22.04", "arm64", []string{"main", "universe"})

	options := archive.Options{
		Label:      "ubuntu",
		Version:    "22.04",
		Arch:       "arm64",
		Suites:     []string{"jammy"},
		Components: []string{"main", "universe"},
		CacheDir:   c.MkDir(),
		PubKeys:    []*packet.PublicKey{s.pubKey},
	}

	testArchive, err := archive.Open(&options)
	c.Assert(err, IsNil)

	// First on component main.
	pkg, info, err := testArchive.Fetch("mypkg1")
	c.Assert(err, IsNil)
	c.Assert(info, DeepEquals, &archive.PackageInfo{
		Name:    "mypkg1",
		Version: "1.1",
		Arch:    "arm64",
		SHA256:  "1f08ef04cfe7a8087ee38a1ea35fa1810246648136c3c42d5a61ad6503d85e05",
	})
	c.Assert(read(pkg), Equals, "mypkg1 1.1 data")

	// Last on component universe.
	pkg, info, err = testArchive.Fetch("mypkg4")
	c.Assert(err, IsNil)
	c.Assert(info, DeepEquals, &archive.PackageInfo{
		Name:    "mypkg4",
		Version: "1.4",
		Arch:    "arm64",
		SHA256:  "54af70097b30b33cfcbb6911ad3d0df86c2d458928169e348fa7873e4fc678e4",
	})
	c.Assert(read(pkg), Equals, "mypkg4 1.4 data")
}

func (s *httpSuite) TestFetchSecurityPackage(c *C) {

	for i, suite := range []string{"jammy", "jammy-updates", "jammy-security"} {
		release := s.prepareArchive(suite, "22.04", "amd64", []string{"main", "universe"})
		err := release.Walk(func(item testarchive.Item) error {
			if p, ok := item.(*testarchive.Package); ok && p.Name == "mypkg1" {
				p.Version = fmt.Sprintf("%s.%d", p.Version, i)
				p.Data = []byte("package from " + suite)
			}
			return nil
		})
		c.Assert(err, IsNil)
		err = release.Render("/ubuntu", s.responses)
		c.Assert(err, IsNil)
	}

	options := archive.Options{
		Label:      "ubuntu",
		Version:    "22.04",
		CacheDir:   c.MkDir(),
		Arch:       "amd64",
		Suites:     []string{"jammy", "jammy-security", "jammy-updates"},
		Components: []string{"main", "universe"},
		PubKeys:    []*packet.PublicKey{s.pubKey},
	}

	testArchive, err := archive.Open(&options)
	c.Assert(err, IsNil)

	pkg, info, err := testArchive.Fetch("mypkg1")
	c.Assert(err, IsNil)
	c.Assert(info, DeepEquals, &archive.PackageInfo{
		Name:    "mypkg1",
		Version: "1.1.2.2",
		Arch:    "amd64",
		SHA256:  "5448585bdd916e5023eff2bc1bc3b30bcc6ee9db9c03e531375a6a11ddf0913c",
	})
	c.Assert(read(pkg), Equals, "package from jammy-security")

	pkg, info, err = testArchive.Fetch("mypkg2")
	c.Assert(err, IsNil)
	c.Assert(info, DeepEquals, &archive.PackageInfo{
		Name:    "mypkg2",
		Version: "1.2",
		Arch:    "amd64",
		SHA256:  "a4b4f3f3a8fa09b69e3ba23c60a41a1f8144691fd371a2455812572fd02e6f79",
	})
	c.Assert(read(pkg), Equals, "mypkg2 1.2 data")
}

func (s *httpSuite) TestArchiveLabels(c *C) {
	setLabel := func(label string) func(*testarchive.Release) {
		return func(r *testarchive.Release) {
			r.Label = label
		}
	}

	tests := []struct {
		summary string
		label   string
		err     string
	}{{
		summary: "Ubuntu label",
		label:   "Ubuntu",
	}, {
		summary: "Unknown label",
		label:   "Unknown",
		err:     "corrupted archive InRelease file: no Ubuntu section",
	}}

	for _, test := range tests {
		c.Logf("Summary: %s", test.summary)

		var adjust func(*testarchive.Release)
		if test.label != "" {
			adjust = setLabel(test.label)
		}
		s.prepareArchiveAdjustRelease("jammy", "22.04", "amd64", []string{"main", "universe"}, []string{"SHA256"}, adjust)

		options := archive.Options{
			Label:      "ubuntu",
			Version:    "22.04",
			Arch:       "amd64",
			Suites:     []string{"jammy"},
			Components: []string{"main", "universe"},
			CacheDir:   c.MkDir(),
			PubKeys:    []*packet.PublicKey{s.pubKey},
		}

		_, err := archive.Open(&options)
		if test.err != "" {
			c.Assert(err, ErrorMatches, test.err)
		} else {
			c.Assert(err, IsNil)
		}
	}
}

func (s *httpSuite) TestProArchives(c *C) {
	setLabel := func(label string) func(*testarchive.Release) {
		return func(r *testarchive.Release) {
			r.Label = label
		}
	}

	credsDir := c.MkDir()
	restore := fakeEnv("CHISEL_AUTH_DIR", credsDir)
	defer restore()

	confFile := filepath.Join(credsDir, "credentials")
	contents := ""
	for _, info := range archive.ProArchiveInfo {
		contents += fmt.Sprintf("machine %s login foo password bar\n", info.BaseURL)
	}
	err := os.WriteFile(confFile, []byte(contents), 0600)
	c.Assert(err, IsNil)

	do := func(req *http.Request) (*http.Response, error) {
		auth, ok := req.Header["Authorization"]
		c.Assert(ok, Equals, true)
		c.Assert(auth, DeepEquals, []string{"Basic Zm9vOmJhcg=="})
		return s.Do(req)
	}
	restoreDo := archive.FakeDo(do)
	defer restoreDo()

	for pro, info := range archive.ProArchiveInfo {
		s.base = info.BaseURL
		s.prepareArchiveAdjustRelease("focal", "20.04", "amd64", []string{"main"}, []string{"SHA256"}, setLabel(info.Label))

		options := archive.Options{
			Label:      "ubuntu",
			Version:    "20.04",
			Arch:       "amd64",
			Suites:     []string{"focal"},
			Components: []string{"main"},
			CacheDir:   c.MkDir(),
			Pro:        pro,
			PubKeys:    []*packet.PublicKey{s.pubKey},
		}

		_, err = archive.Open(&options)
		c.Assert(err, IsNil)
	}

	// Test non-pro archives.
	do = func(req *http.Request) (*http.Response, error) {
		_, ok := req.Header["Authorization"]
		c.Assert(ok, Equals, false, Commentf("Non-pro archives should not have any authorization header"))
		return s.Do(req)
	}
	restoreDo = archive.FakeDo(do)
	defer restoreDo()

	s.base = "http://archive.ubuntu.com/ubuntu/"
	s.prepareArchive("focal", "20.04", "amd64", []string{"main"})

	options := archive.Options{
		Label:      "ubuntu",
		Version:    "20.04",
		Arch:       "amd64",
		Suites:     []string{"focal"},
		Components: []string{"main"},
		CacheDir:   c.MkDir(),
		PubKeys:    []*packet.PublicKey{s.pubKey},
	}

	_, err = archive.Open(&options)
	c.Assert(err, IsNil)

	// Test Pro archives with bad credentials.
	do = func(req *http.Request) (*http.Response, error) {
		_, ok := req.Header["Authorization"]
		c.Assert(ok, Equals, true)
		if strings.Contains(req.URL.String(), "/pool/") {
			s.status = 401
		} else {
			s.status = 200
		}
		return s.Do(req)
	}
	restoreDo = archive.FakeDo(do)
	defer restoreDo()

	for pro, info := range archive.ProArchiveInfo {
		s.base = info.BaseURL
		s.prepareArchiveAdjustRelease("focal", "20.04", "amd64", []string{"main"}, []string{"SHA256"}, setLabel(info.Label))

		options := archive.Options{
			Label:      "ubuntu",
			Version:    "20.04",
			Arch:       "amd64",
			Suites:     []string{"focal"},
			Components: []string{"main"},
			CacheDir:   c.MkDir(),
			Pro:        pro,
			PubKeys:    []*packet.PublicKey{s.pubKey},
		}

		testArchive, err := archive.Open(&options)
		c.Assert(err, IsNil)

		_, _, err = testArchive.Fetch("mypkg1")
		c.Assert(err, ErrorMatches, `cannot fetch from "ubuntu": unauthorized`)
	}
}

func (s *httpSuite) TestOpenUnmaintainedArchives(c *C) {
	s.base = "http://old-releases.ubuntu.com/ubuntu/"
	s.prepareArchive("jammy", "22.04", "amd64", []string{"main", "universe"})

	options := archive.Options{
		Label:      "ubuntu",
		Version:    "22.04",
		Arch:       "amd64",
		Suites:     []string{"jammy"},
		Components: []string{"main", "universe"},
		CacheDir:   c.MkDir(),
		PubKeys:    []*packet.PublicKey{s.pubKey},
		OldRelease: false,
	}

	_, err := archive.Open(&options)
	// Fails when OldRelease is not set because it attempts to contact the
	// default ubuntu archive where the release is no longer available.
	c.Assert(err, Not(IsNil))

	options.OldRelease = true
	_, err = archive.Open(&options)
	c.Assert(err, IsNil)
}

type verifyArchiveReleaseTest struct {
	summary string
	pubKeys []*packet.PublicKey
	error   string
}

var verifyArchiveReleaseTests = []verifyArchiveReleaseTest{{
	summary: "A valid public key",
	pubKeys: []*packet.PublicKey{key1.PubKey},
}, {
	summary: "No public key to verify with",
	error:   `cannot verify signature of the InRelease file`,
}, {
	summary: "Wrong public key",
	pubKeys: []*packet.PublicKey{key2.PubKey},
	error:   `cannot verify signature of the InRelease file`,
}, {
	summary: "Multiple public keys (invalid, valid)",
	pubKeys: []*packet.PublicKey{key2.PubKey, key1.PubKey},
}}

func (s *httpSuite) TestVerifyArchiveRelease(c *C) {
	for _, test := range verifyArchiveReleaseTests {
		c.Logf("Summary: %s", test.summary)

		s.prepareArchive("jammy", "22.04", "amd64", []string{"main", "universe"})

		options := archive.Options{
			Label:      "ubuntu",
			Version:    "22.04",
			Arch:       "amd64",
			Suites:     []string{"jammy"},
			Components: []string{"main", "universe"},
			CacheDir:   c.MkDir(),
			PubKeys:    test.pubKeys,
		}

		_, err := archive.Open(&options)
		if test.error != "" {
			c.Assert(err, ErrorMatches, test.error)
		} else {
			c.Assert(err, IsNil)
		}
	}
}

var packageInfoTests = []struct {
	summary string
	pkg     string
	info    *archive.PackageInfo
	error   string
}{{
	summary: "Basic",
	pkg:     "mypkg1",
	info: &archive.PackageInfo{
		Name:    "mypkg1",
		Version: "1.1",
		Arch:    "amd64",
		SHA256:  "1f08ef04cfe7a8087ee38a1ea35fa1810246648136c3c42d5a61ad6503d85e05",
	},
}, {
	summary: "Package not found in archive",
	pkg:     "mypkg99",
	error:   `cannot find package "mypkg99" in archive`,
}}

func (s *httpSuite) TestPackageInfo(c *C) {
	s.prepareArchive("jammy", "22.04", "amd64", []string{"main", "universe"})

	options := archive.Options{
		Label:      "ubuntu",
		Version:    "22.04",
		Arch:       "amd64",
		Suites:     []string{"jammy"},
		Components: []string{"main", "universe"},
		CacheDir:   c.MkDir(),
		PubKeys:    []*packet.PublicKey{s.pubKey},
	}

	testArchive, err := archive.Open(&options)
	c.Assert(err, IsNil)

	for _, test := range packageInfoTests {
		info, err := testArchive.Info(test.pkg)
		if test.error != "" {
			c.Assert(err, ErrorMatches, test.error)
			continue
		}
		c.Assert(err, IsNil)
		c.Assert(info, DeepEquals, test.info)
	}
}

func read(r io.Reader) string {
	data, err := io.ReadAll(r)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// fetchRequestStatus checks whether a request was made whose URL path
// contains the given substring. If so, it returns true and the HTTP status
// code from the most recent matching request. If not, it returns false, 0.
func (s *httpSuite) fetchRequestStatus(pathSubstring string) (bool, int) {
	for i := len(s.requestResults) - 1; i >= 0; i-- {
		if strings.Contains(s.requestResults[i].path, pathSubstring) {
			return true, s.requestResults[i].status
		}
	}
	return false, 0
}

func (s *httpSuite) TestFetchByHashSucceedsWhenNamedPathIsStale(c *C) {
	s.prepareArchiveAdjustRelease("jammy", "22.04", "amd64", []string{"main"}, []string{"SHA256"}, func(r *testarchive.Release) {
		r.ByHash = true
	})

	// Simulate a mirror serving stale content at the named Packages.gz path
	// while the by-hash path still has the correct data.
	for p := range s.responses {
		if strings.Contains(p, "Packages.gz") && !strings.Contains(p, "/by-hash/") {
			s.responses[p] = testarchive.MakeGzip([]byte("stale Packages from previous publication"))
		}
	}

	options := archive.Options{
		Label:      "ubuntu",
		Version:    "22.04",
		Arch:       "amd64",
		Suites:     []string{"jammy"},
		Components: []string{"main"},
		CacheDir:   c.MkDir(),
		PubKeys:    []*packet.PublicKey{s.pubKey},
	}

	testArchive, err := archive.Open(&options)
	c.Assert(err, IsNil)

	pkg, _, err := testArchive.Fetch("mypkg1")
	c.Assert(err, IsNil)
	c.Assert(read(pkg), Equals, "mypkg1 1.1 data")

	// The by-hash request must have been attempted and succeeded with 200,
	// since the named path has stale content and only by-hash has the
	// correct data.
	attempted, status := s.fetchRequestStatus("/by-hash/SHA256/")
	c.Assert(attempted, Equals, true)
	c.Assert(status, Equals, 200)
}

func (s *httpSuite) TestFetchByHashSHA512(c *C) {
	// Ubuntu 26.10+ advertises Acquire-By-Hash with SHA512-only indices, so
	// the by-hash URL must be built under the SHA512 directory.
	s.prepareArchiveAdjustRelease("stonking", "26.10", "amd64", []string{"main"}, []string{"SHA512"}, func(release *testarchive.Release) {
		release.ByHash = true
	})

	// Stale content at the named Packages.gz path, so a fallback would fail
	// the digest check -- only the by-hash path serves the correct bytes.
	for p := range s.responses {
		if strings.Contains(p, "Packages.gz") && !strings.Contains(p, "/by-hash/") {
			s.responses[p] = testarchive.MakeGzip([]byte("stale Packages from previous publication"))
		}
	}

	options := archive.Options{
		Label:      "ubuntu",
		Version:    "26.10",
		Arch:       "amd64",
		Suites:     []string{"stonking"},
		Components: []string{"main"},
		CacheDir:   c.MkDir(),
		PubKeys:    []*packet.PublicKey{s.pubKey},
	}

	testArchive, err := archive.Open(&options)
	c.Assert(err, IsNil)

	pkg, _, err := testArchive.Fetch("mypkg1")
	c.Assert(err, IsNil)
	c.Assert(read(pkg), Equals, "mypkg1 1.1 data")

	// The SHA512 by-hash request must have been attempted and succeeded;
	// the named path only has stale content.
	attempted, status := s.fetchRequestStatus("/by-hash/SHA512/")
	c.Assert(attempted, Equals, true)
	c.Assert(status, Equals, 200)
}

func (s *httpSuite) TestFetchByHashBothDigests(c *C) {
	// When a by-hash archive publishes both digests, the by-hash URL must be
	// built under SHA512: archives only guarantee a by-hash directory for the
	// strongest hash they advertise. SHA256 by-hash must not be requested.
	s.prepareArchiveAdjustRelease("stonking", "26.10", "amd64", []string{"main"}, []string{"SHA256", "SHA512"}, func(release *testarchive.Release) {
		release.ByHash = true
	})

	// Stale content at the named Packages.gz path, so a fallback would fail
	// the digest check -- only the by-hash path serves the correct bytes.
	for p := range s.responses {
		if strings.Contains(p, "Packages.gz") && !strings.Contains(p, "/by-hash/") {
			s.responses[p] = testarchive.MakeGzip([]byte("stale Packages from previous publication"))
		}
	}

	options := archive.Options{
		Label:      "ubuntu",
		Version:    "26.10",
		Arch:       "amd64",
		Suites:     []string{"stonking"},
		Components: []string{"main"},
		CacheDir:   c.MkDir(),
		PubKeys:    []*packet.PublicKey{s.pubKey},
	}

	testArchive, err := archive.Open(&options)
	c.Assert(err, IsNil)

	pkg, _, err := testArchive.Fetch("mypkg1")
	c.Assert(err, IsNil)
	c.Assert(read(pkg), Equals, "mypkg1 1.1 data")

	// The SHA512 by-hash request must have been made and succeeded; the SHA256
	// by-hash directory must never be touched.
	attempted, status := s.fetchRequestStatus("/by-hash/SHA512/")
	c.Assert(attempted, Equals, true)
	c.Assert(status, Equals, 200)
	attempted, _ = s.fetchRequestStatus("/by-hash/SHA256/")
	c.Assert(attempted, Equals, false)
}

// addSha256ByHashDirs mirrors every published by-hash/SHA512 entry under a
// by-hash/SHA256 path, like archives publishing by-hash directories for
// more hashes than just the strongest one (e.g. stonking).
func (s *httpSuite) addSha256ByHashDirs() {
	mirrored := make(map[string][]byte)
	for p, content := range s.responses {
		prefix, _, found := strings.Cut(p, "/by-hash/SHA512/")
		if !found {
			continue
		}
		mirrored[fmt.Sprintf("%s/by-hash/SHA256/%x", prefix, sha256.Sum256(content))] = content
	}
	for p, content := range mirrored {
		s.responses[p] = content
	}
}

func (s *httpSuite) TestFetchByHashManyDigestDirs(c *C) {
	// An archive may publish by-hash directories for more hashes than the
	// strongest one it advertises. The by-hash URL must still be built under
	// SHA512, and the SHA256 directory left alone, even though a request
	// there would now succeed.
	s.prepareArchiveAdjustRelease("stonking", "26.10", "amd64", []string{"main"}, []string{"SHA256", "SHA512"}, func(release *testarchive.Release) {
		release.ByHash = true
	})
	s.addSha256ByHashDirs()

	// Stale content at the named Packages.gz path, so a fallback would fail
	// the digest check -- only the by-hash paths serve the correct bytes.
	for p := range s.responses {
		if strings.Contains(p, "Packages.gz") && !strings.Contains(p, "/by-hash/") {
			s.responses[p] = testarchive.MakeGzip([]byte("stale Packages from previous publication"))
		}
	}

	options := archive.Options{
		Label:      "ubuntu",
		Version:    "26.10",
		Arch:       "amd64",
		Suites:     []string{"stonking"},
		Components: []string{"main"},
		CacheDir:   c.MkDir(),
		PubKeys:    []*packet.PublicKey{s.pubKey},
	}

	testArchive, err := archive.Open(&options)
	c.Assert(err, IsNil)

	pkg, _, err := testArchive.Fetch("mypkg1")
	c.Assert(err, IsNil)
	c.Assert(read(pkg), Equals, "mypkg1 1.1 data")

	attempted, status := s.fetchRequestStatus("/by-hash/SHA512/")
	c.Assert(attempted, Equals, true)
	c.Assert(status, Equals, 200)
	attempted, _ = s.fetchRequestStatus("/by-hash/SHA256/")
	c.Assert(attempted, Equals, false)
}

func (s *httpSuite) TestFetchByHashOnlyWeakerDigestDir(c *C) {
	// A buggy archive advertising both digests but publishing a by-hash
	// directory only for the weaker one. The SHA512 by-hash request 404s and
	// the named path takes over; the SHA256 directory must not be tried.
	s.prepareArchiveAdjustRelease("stonking", "26.10", "amd64", []string{"main"}, []string{"SHA256", "SHA512"}, func(release *testarchive.Release) {
		release.ByHash = true
	})
	s.addSha256ByHashDirs()
	for p := range s.responses {
		if strings.Contains(p, "/by-hash/SHA512/") {
			delete(s.responses, p)
		}
	}

	options := archive.Options{
		Label:      "ubuntu",
		Version:    "26.10",
		Arch:       "amd64",
		Suites:     []string{"stonking"},
		Components: []string{"main"},
		CacheDir:   c.MkDir(),
		PubKeys:    []*packet.PublicKey{s.pubKey},
	}

	testArchive, err := archive.Open(&options)
	c.Assert(err, IsNil)

	pkg, _, err := testArchive.Fetch("mypkg1")
	c.Assert(err, IsNil)
	c.Assert(read(pkg), Equals, "mypkg1 1.1 data")

	attempted, status := s.fetchRequestStatus("/by-hash/SHA512/")
	c.Assert(attempted, Equals, true)
	c.Assert(status, Equals, 404)
	attempted, _ = s.fetchRequestStatus("/by-hash/SHA256/")
	c.Assert(attempted, Equals, false)
	attempted, status = s.fetchRequestStatus("Packages.gz")
	c.Assert(attempted, Equals, true)
	c.Assert(status, Equals, 200)
}

func (s *httpSuite) TestFetchByHashFallsBackOnNotFound(c *C) {
	s.prepareArchiveAdjustRelease("jammy", "22.04", "amd64", []string{"main"}, []string{"SHA256"}, func(r *testarchive.Release) {
		r.ByHash = true
	})

	// Simulate a mirror that garbage-collected the by-hash entries.
	for p := range s.responses {
		if strings.Contains(p, "/by-hash/") {
			delete(s.responses, p)
		}
	}

	options := archive.Options{
		Label:      "ubuntu",
		Version:    "22.04",
		Arch:       "amd64",
		Suites:     []string{"jammy"},
		Components: []string{"main"},
		CacheDir:   c.MkDir(),
		PubKeys:    []*packet.PublicKey{s.pubKey},
	}

	testArchive, err := archive.Open(&options)
	c.Assert(err, IsNil)

	pkg, _, err := testArchive.Fetch("mypkg1")
	c.Assert(err, IsNil)
	c.Assert(read(pkg), Equals, "mypkg1 1.1 data")

	// The by-hash request must have been attempted but got 404 (the hash
	// was garbage-collected), so we fell back to the named path which
	// returned 200 with the correct data.
	attempted, status := s.fetchRequestStatus("/by-hash/SHA256/")
	c.Assert(attempted, Equals, true)
	c.Assert(status, Equals, 404)
	attempted, status = s.fetchRequestStatus("Packages.gz")
	c.Assert(attempted, Equals, true)
	c.Assert(status, Equals, 200)
}

func (s *httpSuite) TestFetchSkipsByHashWhenNotAdvertised(c *C) {
	s.prepareArchive("jammy", "22.04", "amd64", []string{"main"})

	options := archive.Options{
		Label:      "ubuntu",
		Version:    "22.04",
		Arch:       "amd64",
		Suites:     []string{"jammy"},
		Components: []string{"main"},
		CacheDir:   c.MkDir(),
		PubKeys:    []*packet.PublicKey{s.pubKey},
	}

	_, err := archive.Open(&options)
	c.Assert(err, IsNil)

	// When by-hash is not advertised, no by-hash request should be
	// attempted; only the named Packages.gz path is fetched.
	attempted, _ := s.fetchRequestStatus("/by-hash/SHA256/")
	c.Assert(attempted, Equals, false)
	attempted, status := s.fetchRequestStatus("Packages.gz")
	c.Assert(attempted, Equals, true)
	c.Assert(status, Equals, 200)
}

// ----------------------------------------------------------------------------------------
// Real archive tests, only enabled via:
//   1. --real-archive      for non-Pro archives (e.g. standard jammy archive),
//   2. --real-pro-archive  for Ubuntu Pro archives (e.g. FIPS archives).
//
// To run the tests for Ubuntu Pro archives, the host machine must be Pro
// enabled and relevant Pro services must be enabled. The following commands
// might help:
//   sudo pro attach <pro-token> --no-auto-enable
//   sudo pro enable fips-updates esm-apps esm-infra --assume-yes

var realArchiveFlag = flag.Bool("real-archive", false, "Perform tests against real archive")
var proArchiveFlag = flag.Bool("real-pro-archive", false, "Perform tests against real Ubuntu Pro archive")

func (s *S) TestRealArchive(c *C) {
	if !*realArchiveFlag {
		c.Skip("--real-archive not provided")
	}
	s.runRealArchiveTests(c, realArchiveTests)
}

func (s *S) TestRealProArchives(c *C) {
	if !*proArchiveFlag {
		c.Skip("--real-pro-archive not provided")
	}
	s.runRealArchiveTests(c, proArchiveTests)
	s.testRealProArchiveBadCreds(c)
}

func (s *S) runRealArchiveTests(c *C, tests []realArchiveTest) {
	allArch := make([]string, 0, len(elfToDebArch))
	for _, arch := range elfToDebArch {
		allArch = append(allArch, arch)
	}
	for _, test := range tests {
		if len(test.archs) == 0 {
			test.archs = allArch
		}
		for _, arch := range test.archs {
			s.testOpenArchiveArch(c, test, arch)
		}
	}
}

type realArchiveTest struct {
	name           string
	version        string
	suites         []string
	components     []string
	pro            string
	oldRelease     bool
	archivePubKeys []*packet.PublicKey
	archs          []string
	pkg            string
	path           string
}

var realArchiveTests = []realArchiveTest{{
	name:           "focal",
	version:        "20.04",
	oldRelease:     false,
	suites:         []string{"focal"},
	components:     []string{"main", "universe"},
	archivePubKeys: []*packet.PublicKey{keyUbuntu2018.PubKey},
	pkg:            "hostname",
	path:           "/bin/hostname",
}, {
	name:           "jammy",
	version:        "22.04",
	oldRelease:     false,
	suites:         []string{"jammy"},
	components:     []string{"main", "universe"},
	archivePubKeys: []*packet.PublicKey{keyUbuntu2018.PubKey},
	pkg:            "hostname",
	path:           "/bin/hostname",
}, {
	name:           "noble",
	version:        "24.04",
	oldRelease:     false,
	suites:         []string{"noble"},
	components:     []string{"main", "universe"},
	archivePubKeys: []*packet.PublicKey{keyUbuntu2018.PubKey},
	pkg:            "hostname",
	path:           "/usr/bin/hostname",
}, {
	name:           "mantic",
	version:        "23.10",
	oldRelease:     true,
	suites:         []string{"mantic"},
	components:     []string{"main", "universe"},
	archivePubKeys: []*packet.PublicKey{keyUbuntu2018.PubKey},
	pkg:            "hostname",
	path:           "/bin/hostname",
}}

var proArchiveTests = []realArchiveTest{{
	name:           "focal-fips",
	version:        "20.04",
	suites:         []string{"focal"},
	components:     []string{"main"},
	pro:            "fips",
	archivePubKeys: []*packet.PublicKey{keyUbuntuFIPSv1.PubKey},
	archs:          []string{"amd64"},
	pkg:            "openssh-client",
	path:           "/usr/bin/ssh",
}, {
	name:           "focal-fips-updates",
	version:        "20.04",
	suites:         []string{"focal-updates"},
	components:     []string{"main"},
	pro:            "fips-updates",
	archivePubKeys: []*packet.PublicKey{keyUbuntuFIPSv1.PubKey},
	archs:          []string{"amd64"},
	pkg:            "openssh-client",
	path:           "/usr/bin/ssh",
}, {
	name:           "focal-esm-apps",
	version:        "20.04",
	suites:         []string{"focal-apps-security", "focal-apps-updates"},
	components:     []string{"main"},
	pro:            "esm-apps",
	archivePubKeys: []*packet.PublicKey{keyUbuntuApps.PubKey},
	archs:          []string{"amd64"},
	pkg:            "hello",
	path:           "/usr/bin/hello",
}, {
	name:           "focal-esm-infra",
	version:        "20.04",
	suites:         []string{"focal-infra-security", "focal-infra-updates"},
	components:     []string{"main"},
	pro:            "esm-infra",
	archivePubKeys: []*packet.PublicKey{keyUbuntuESMv2.PubKey},
	archs:          []string{"amd64"},
	pkg:            "hello",
	path:           "/usr/bin/hello",
}, {
	name:           "jammy-fips-updates",
	version:        "22.04",
	suites:         []string{"jammy-updates"},
	components:     []string{"main"},
	pro:            "fips-updates",
	archivePubKeys: []*packet.PublicKey{keyUbuntuFIPSv1.PubKey},
	archs:          []string{"amd64"},
	pkg:            "openssh-client",
	path:           "/usr/bin/ssh",
}, {
	name:           "jammy-esm-apps",
	version:        "22.04",
	suites:         []string{"jammy-apps-security", "jammy-apps-updates"},
	components:     []string{"main"},
	pro:            "esm-apps",
	archivePubKeys: []*packet.PublicKey{keyUbuntuApps.PubKey},
	archs:          []string{"amd64"},
	pkg:            "hello",
	path:           "/usr/bin/hello",
}, {
	name:           "jammy-esm-infra",
	version:        "22.04",
	suites:         []string{"jammy-infra-security", "jammy-infra-updates"},
	components:     []string{"main"},
	pro:            "esm-infra",
	archivePubKeys: []*packet.PublicKey{keyUbuntuESMv2.PubKey},
	archs:          []string{"amd64"},
	pkg:            "hello",
	path:           "/usr/bin/hello",
}, {
	name:           "noble-esm-apps",
	version:        "24.04",
	suites:         []string{"noble-apps-security", "noble-apps-updates"},
	components:     []string{"main"},
	pro:            "esm-apps",
	archivePubKeys: []*packet.PublicKey{keyUbuntuApps.PubKey},
	archs:          []string{"amd64"},
	pkg:            "hello",
	path:           "/usr/bin/hello",
}, {
	name:           "noble-esm-infra",
	version:        "24.04",
	suites:         []string{"noble-infra-security", "noble-infra-updates"},
	components:     []string{"main"},
	pro:            "esm-infra",
	archivePubKeys: []*packet.PublicKey{keyUbuntuESMv2.PubKey},
	archs:          []string{"amd64"},
	pkg:            "hello",
	path:           "/usr/bin/hello",
}}

var elfToDebArch = map[elf.Machine]string{
	elf.EM_386:     "i386",
	elf.EM_AARCH64: "arm64",
	elf.EM_ARM:     "armhf",
	elf.EM_PPC64:   "ppc64el",
	elf.EM_RISCV:   "riscv64",
	elf.EM_S390:    "s390x",
	elf.EM_X86_64:  "amd64",
}

func (s *S) checkArchitecture(c *C, arch string, binaryPath string) {
	file, err := elf.Open(binaryPath)
	c.Assert(err, IsNil)
	defer file.Close()

	binaryArch := elfToDebArch[file.Machine]
	c.Assert(binaryArch, Equals, arch)
}

func (s *S) testOpenArchiveArch(c *C, test realArchiveTest, arch string) {
	c.Logf("Checking ubuntu archive %s %s...", test.name, arch)

	options := archive.Options{
		Label:      "ubuntu",
		Version:    test.version,
		Arch:       arch,
		Suites:     test.suites,
		Components: test.components,
		CacheDir:   c.MkDir(),
		Pro:        test.pro,
		PubKeys:    test.archivePubKeys,
		OldRelease: test.oldRelease,
	}

	testArchive, err := archive.Open(&options)
	c.Assert(err, IsNil)

	extractDir := c.MkDir()

	pkg, info, err := testArchive.Fetch(test.pkg)
	c.Assert(err, IsNil)
	c.Assert(info.Name, DeepEquals, test.pkg)
	c.Assert(info.Arch, DeepEquals, arch)

	err = tarball.Extract(pkg, deb.OpenTar, &tarball.ExtractOptions{
		Package:   test.pkg,
		TargetDir: extractDir,
		Extract: map[string][]tarball.ExtractInfo{
			fmt.Sprintf("/usr/share/doc/%s/copyright", test.pkg): {
				{Path: "/copyright"},
			},
			test.path: {
				{Path: "/binary"},
			},
		},
	})
	c.Assert(err, IsNil)

	s.checkArchitecture(c, arch, filepath.Join(extractDir, "binary"))
}

func (s *S) testRealProArchiveBadCreds(c *C) {
	c.Logf("Cannot fetch Pro packages with bad credentials")

	credsDir := c.MkDir()
	restore := fakeEnv("CHISEL_AUTH_DIR", credsDir)
	defer restore()

	confFile := filepath.Join(credsDir, "credentials")
	contents := "machine esm.ubuntu.com/fips/ubuntu/ login bearer password invalid"
	err := os.WriteFile(confFile, []byte(contents), 0600)
	c.Assert(err, IsNil)

	options := archive.Options{
		Label:      "ubuntu",
		Version:    "20.04",
		Arch:       "amd64",
		Suites:     []string{"focal"},
		Components: []string{"main"},
		CacheDir:   c.MkDir(),
		Pro:        "fips",
		PubKeys:    []*packet.PublicKey{keyUbuntuFIPSv1.PubKey},
	}

	// The archive can be "opened" without any credentials since the dists/ path
	// containing InRelease files, does not require any credentials.
	testArchive, err := archive.Open(&options)
	c.Assert(err, IsNil)

	_, _, err = testArchive.Fetch("openssh-client")
	c.Assert(err, ErrorMatches, `cannot fetch from "ubuntu": unauthorized`)
}
