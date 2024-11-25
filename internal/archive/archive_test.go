package archive_test

import (
	"golang.org/x/crypto/openpgp/packet"
	. "gopkg.in/check.v1"

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
	"github.com/canonical/chisel/internal/testutil"
)

type httpSuite struct {
	logf      func(string, ...interface{})
	base      string
	request   *http.Request
	requests  []*http.Request
	response  string
	responses map[string][]byte
	err       error
	header    http.Header
	status    int
	restore   func()
	privKey   *packet.PrivateKey
	pubKey    *packet.PublicKey
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

	s.request = req
	s.requests = append(s.requests, req)
	body := s.response
	s.logf("Request: %s", req.URL.String())
	if response, ok := s.responses[path.Clean(req.URL.Path)]; ok {
		body = string(response)
	}
	rsp := &http.Response{
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     s.header,
		StatusCode: s.status,
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
	return s.prepareArchiveAdjustRelease(suite, version, arch, components, nil)
}

func (s *httpSuite) prepareArchiveAdjustRelease(suite, version, arch string, components []string, adjustRelease func(*testarchive.Release)) *testarchive.Release {
	release := &testarchive.Release{
		Suite:   suite,
		Version: version,
		Label:   "Ubuntu",
		PrivKey: s.privKey,
	}
	for i, component := range components {
		index := &testarchive.PackageIndex{
			Component: component,
			Arch:      arch,
		}
		for j := 0; j < 2; j++ {
			seq := 1 + i*2 + j
			index.Packages = append(index.Packages, &testarchive.Package{
				Name:      fmt.Sprintf("mypkg%d", seq),
				Version:   fmt.Sprintf("1.%d", seq),
				Arch:      arch,
				Component: component,
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
	release.Render(base.Path, s.responses)
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
		release.Walk(func(item testarchive.Item) error {
			if p, ok := item.(*testarchive.Package); ok && p.Name == "mypkg1" {
				p.Version = fmt.Sprintf("%s.%d", p.Version, i)
				p.Data = []byte("package from " + suite)
			}
			return nil
		})
		release.Render("/ubuntu", s.responses)
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
		s.prepareArchiveAdjustRelease("jammy", "22.04", "amd64", []string{"main", "universe"}, adjust)

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
		s.prepareArchiveAdjustRelease("focal", "20.04", "amd64", []string{"main"}, setLabel(info.Label))

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
		s.prepareArchiveAdjustRelease("focal", "20.04", "amd64", []string{"main"}, setLabel(info.Label))

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
	archivePubKeys []*packet.PublicKey
	archs          []string
	pkg            string
	path           string
}

var realArchiveTests = []realArchiveTest{{
	name:           "focal",
	version:        "20.04",
	suites:         []string{"focal"},
	components:     []string{"main", "universe"},
	archivePubKeys: []*packet.PublicKey{keyUbuntu2018.PubKey},
	pkg:            "hostname",
	path:           "/bin/hostname",
}, {
	name:           "jammy",
	version:        "22.04",
	suites:         []string{"jammy"},
	components:     []string{"main", "universe"},
	archivePubKeys: []*packet.PublicKey{keyUbuntu2018.PubKey},
	pkg:            "hostname",
	path:           "/bin/hostname",
}, {
	name:           "noble",
	version:        "24.04",
	suites:         []string{"noble"},
	components:     []string{"main", "universe"},
	archivePubKeys: []*packet.PublicKey{keyUbuntu2018.PubKey},
	pkg:            "hostname",
	path:           "/usr/bin/hostname",
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
	}

	testArchive, err := archive.Open(&options)
	c.Assert(err, IsNil)

	extractDir := c.MkDir()

	pkg, info, err := testArchive.Fetch(test.pkg)
	c.Assert(err, IsNil)
	c.Assert(info.Name, DeepEquals, test.pkg)
	c.Assert(info.Arch, DeepEquals, arch)

	err = deb.Extract(pkg, &deb.ExtractOptions{
		Package:   test.pkg,
		TargetDir: extractDir,
		Extract: map[string][]deb.ExtractInfo{
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
