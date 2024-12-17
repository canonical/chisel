package archive

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/openpgp/packet"

	"github.com/canonical/chisel/internal/cache"
	"github.com/canonical/chisel/internal/control"
	"github.com/canonical/chisel/internal/deb"
	"github.com/canonical/chisel/internal/pgputil"
)

type Archive interface {
	Options() *Options
	Fetch(pkg string) (io.ReadSeekCloser, *PackageInfo, error)
	Exists(pkg string) bool
	Info(pkg string) (*PackageInfo, error)
}

type PackageInfo struct {
	Name    string
	Version string
	Arch    string
	SHA256  string
}

type Options struct {
	Label      string
	Version    string
	Arch       string
	Suites     []string
	Components []string
	Pro        string
	CacheDir   string
	PubKeys    []*packet.PublicKey
}

func Open(options *Options) (Archive, error) {
	var err error
	if options.Arch == "" {
		options.Arch, err = deb.InferArch()
	} else {
		err = deb.ValidateArch(options.Arch)
	}
	if err != nil {
		return nil, err
	}
	return openUbuntu(options)
}

type fetchFlags uint

const (
	fetchBulk    fetchFlags = 1 << iota
	fetchDefault fetchFlags = 0
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

var httpDo = httpClient.Do

var bulkClient = &http.Client{
	Timeout: 5 * time.Minute,
}

var bulkDo = bulkClient.Do

type ubuntuArchive struct {
	options Options
	indexes []*ubuntuIndex
	cache   *cache.Cache
	pubKeys []*packet.PublicKey
	baseURL string
	creds   *credentials
}

type ubuntuIndex struct {
	label     string
	version   string
	arch      string
	suite     string
	component string
	release   control.Section
	packages  control.File
	archive   *ubuntuArchive
}

func (a *ubuntuArchive) Options() *Options {
	return &a.options
}

func (a *ubuntuArchive) Exists(pkg string) bool {
	_, _, err := a.selectPackage(pkg)
	return err == nil
}

func (a *ubuntuArchive) selectPackage(pkg string) (control.Section, *ubuntuIndex, error) {
	var selectedVersion string
	var selectedSection control.Section
	var selectedIndex *ubuntuIndex
	for _, index := range a.indexes {
		section := index.packages.Section(pkg)
		if section != nil && section.Get("Filename") != "" {
			version := section.Get("Version")
			if selectedVersion == "" || deb.CompareVersions(selectedVersion, version) < 0 {
				selectedVersion = version
				selectedSection = section
				selectedIndex = index
			}
		}
	}
	if selectedVersion == "" {
		return nil, nil, fmt.Errorf("cannot find package %q in archive", pkg)
	}
	return selectedSection, selectedIndex, nil
}

func (a *ubuntuArchive) Fetch(pkg string) (io.ReadSeekCloser, *PackageInfo, error) {
	section, index, err := a.selectPackage(pkg)
	if err != nil {
		return nil, nil, err
	}
	suffix := section.Get("Filename")
	logf("Fetching %s...", suffix)
	reader, err := index.fetch("../../"+suffix, section.Get("SHA256"), fetchBulk)
	if err != nil {
		return nil, nil, err
	}
	info := sectionPackageInfo(section)
	return reader, info, nil
}

func (a *ubuntuArchive) Info(pkg string) (*PackageInfo, error) {
	section, _, err := a.selectPackage(pkg)
	if err != nil {
		return nil, err
	}
	info := sectionPackageInfo(section)
	return info, nil
}

const ubuntuURL = "http://archive.ubuntu.com/ubuntu/"
const ubuntuPortsURL = "http://ports.ubuntu.com/ubuntu-ports/"

const (
	ProFIPS        = "fips"
	ProFIPSUpdates = "fips-updates"
	ProApps        = "esm-apps"
	ProInfra       = "esm-infra"
)

var proArchiveInfo = map[string]struct {
	BaseURL, Label string
}{
	ProFIPS: {
		BaseURL: "https://esm.ubuntu.com/fips/ubuntu/",
		Label:   "UbuntuFIPS",
	},
	ProFIPSUpdates: {
		BaseURL: "https://esm.ubuntu.com/fips-updates/ubuntu/",
		Label:   "UbuntuFIPSUpdates",
	},
	ProApps: {
		BaseURL: "https://esm.ubuntu.com/apps/ubuntu/",
		Label:   "UbuntuESMApps",
	},
	ProInfra: {
		BaseURL: "https://esm.ubuntu.com/infra/ubuntu/",
		Label:   "UbuntuESM",
	},
}

func archiveURL(pro, arch string) (string, *credentials, error) {
	if pro != "" {
		archiveInfo, ok := proArchiveInfo[pro]
		if !ok {
			return "", nil, fmt.Errorf("invalid pro value: %q", pro)
		}
		url := archiveInfo.BaseURL
		creds, err := findCredentials(url)
		if err != nil {
			return "", nil, err
		}
		return url, creds, nil
	}

	if arch == "amd64" || arch == "i386" {
		return ubuntuURL, nil, nil
	}
	return ubuntuPortsURL, nil, nil
}

func openUbuntu(options *Options) (Archive, error) {
	if len(options.Components) == 0 {
		return nil, fmt.Errorf("archive options missing components")
	}
	if len(options.Suites) == 0 {
		return nil, fmt.Errorf("archive options missing suites")
	}
	if len(options.Version) == 0 {
		return nil, fmt.Errorf("archive options missing version")
	}

	baseURL, creds, err := archiveURL(options.Pro, options.Arch)
	if err != nil {
		return nil, err
	}

	archive := &ubuntuArchive{
		options: *options,
		cache: &cache.Cache{
			Dir: options.CacheDir,
		},
		pubKeys: options.PubKeys,
		baseURL: baseURL,
		creds:   creds,
	}

	for _, suite := range options.Suites {
		var release control.Section
		for _, component := range options.Components {
			index := &ubuntuIndex{
				label:     options.Label,
				version:   options.Version,
				arch:      options.Arch,
				suite:     suite,
				component: component,
				release:   release,
				archive:   archive,
			}
			if release == nil {
				err := index.fetchRelease()
				if err != nil {
					return nil, err
				}
				release = index.release
				if !index.supportsArch(options.Arch) {
					// Release does not support the specified architecture, do
					// not add any of its indexes.
					break
				}
				err = index.checkComponents(options.Components)
				if err != nil {
					return nil, err
				}
			}
			err := index.fetchIndex()
			if err != nil {
				return nil, err
			}
			archive.indexes = append(archive.indexes, index)
		}
	}

	return archive, nil
}

func (index *ubuntuIndex) fetchRelease() error {
	logf("Fetching %s %s %s suite details...", index.displayName(), index.version, index.suite)
	reader, err := index.fetch("InRelease", "", fetchDefault)
	if err != nil {
		return err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	// Decode the signature(s) and verify the InRelease file. The InRelease
	// file may have multiple signatures from different keys. Verify that at
	// least one signature is valid against the archive's set of public keys.
	// Unlike gpg --verify which ensures the verification of all signatures,
	// this is in line with what apt does internally:
	// https://salsa.debian.org/apt-team/apt/-/blob/4e344a4/methods/gpgv.cc#L553-557
	sigs, canonicalBody, err := pgputil.DecodeClearSigned(data)
	if err != nil {
		return fmt.Errorf("cannot decode clearsigned InRelease file: %v", err)
	}
	err = pgputil.VerifyAnySignature(index.archive.pubKeys, sigs, canonicalBody)
	if err != nil {
		return fmt.Errorf("cannot verify signature of the InRelease file")
	}

	// canonicalBody has <CR><LF> line endings, reverting that to match the
	// expected control file format.
	body := strings.ReplaceAll(string(canonicalBody), "\r", "")
	ctrl, err := control.ParseString("Label", body)
	if err != nil {
		return fmt.Errorf("cannot parse InRelease file: %v", err)
	}
	// Parse the appropriate section for the type of archive.
	label := "Ubuntu"
	if index.archive.options.Pro != "" {
		label = proArchiveInfo[index.archive.options.Pro].Label
	}
	section := ctrl.Section(label)
	if section == nil {
		return fmt.Errorf("corrupted archive InRelease file: no %s section", label)
	}
	logf("Release date: %s", section.Get("Date"))

	index.release = section
	return nil
}

func (index *ubuntuIndex) fetchIndex() error {
	digests := index.release.Get("SHA256")
	packagesPath := fmt.Sprintf("%s/binary-%s/Packages", index.component, index.arch)
	digest, _, _ := control.ParsePathInfo(digests, packagesPath)
	if digest == "" {
		return fmt.Errorf("%s is missing from %s %s component digests", packagesPath, index.suite, index.component)
	}

	logf("Fetching index for %s %s %s %s component...", index.displayName(), index.version, index.suite, index.component)
	reader, err := index.fetch(packagesPath+".gz", digest, fetchBulk)
	if err != nil {
		return err
	}
	ctrl, err := control.ParseReader("Package", reader)
	if err != nil {
		return fmt.Errorf("parsing archive Package file: %v", err)
	}

	index.packages = ctrl
	return nil
}

// supportsArch returns true if the Architectures field in the index release
// contains "arch". Per the Debian wiki [1], index release files should list the
// supported architectures in the "Architectures" field.
// The "ubuntuURL" archive only supports the amd64 and i386 architectures
// whereas the "ubuntuPortsURL" one supports the rest. But each of them
// (faultly) specifies all those architectures in their InRelease files.
// Reference: [1] https://wiki.debian.org/DebianRepository/Format#Architectures
func (index *ubuntuIndex) supportsArch(arch string) bool {
	return strings.Contains(index.release.Get("Architectures"), arch)
}

func (index *ubuntuIndex) checkComponents(components []string) error {
	releaseComponents := strings.Fields(index.release.Get("Components"))
	for _, c1 := range components {
		found := false
		for _, c2 := range releaseComponents {
			if c1 == c2 {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("archive has no component %q", c1)
		}
	}
	return nil
}

func (index *ubuntuIndex) fetch(suffix, digest string, flags fetchFlags) (io.ReadSeekCloser, error) {
	reader, err := index.archive.cache.Open(digest)
	if err == nil {
		return reader, nil
	} else if err != cache.MissErr {
		return nil, err
	}

	baseURL, creds := index.archive.baseURL, index.archive.creds

	var url string
	if strings.HasPrefix(suffix, "pool/") {
		url = baseURL + suffix
	} else {
		url = baseURL + "dists/" + index.suite + "/" + suffix
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP request: %v", err)
	}
	if creds != nil && !creds.Empty() {
		req.SetBasicAuth(creds.Username, creds.Password)
	}
	var resp *http.Response
	if flags&fetchBulk != 0 {
		resp, err = bulkDo(req)
	} else {
		resp, err = httpDo(req)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot talk to archive: %v", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		// ok
	case 401:
		return nil, fmt.Errorf("cannot fetch from %q: unauthorized", index.label)
	case 404:
		return nil, fmt.Errorf("cannot find archive data")
	default:
		return nil, fmt.Errorf("error from archive: %v", resp.Status)
	}

	body := resp.Body
	if strings.HasSuffix(suffix, ".gz") {
		reader, err := gzip.NewReader(body)
		if err != nil {
			return nil, fmt.Errorf("cannot decompress data: %v", err)
		}
		defer reader.Close()
		body = reader
	}

	writer := index.archive.cache.Create(digest)
	defer writer.Close()

	_, err = io.Copy(writer, body)
	if err == nil {
		err = writer.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("cannot fetch from archive: %v", err)
	}

	return index.archive.cache.Open(writer.Digest())
}

func sectionPackageInfo(section control.Section) *PackageInfo {
	return &PackageInfo{
		Name:    section.Get("Package"),
		Version: section.Get("Version"),
		Arch:    section.Get("Architecture"),
		SHA256:  section.Get("SHA256"),
	}
}

func (index *ubuntuIndex) displayName() string {
	if index.archive.options.Pro == "" {
		return index.label
	}
	return index.label + " " + index.archive.options.Pro + " (pro)"
}
