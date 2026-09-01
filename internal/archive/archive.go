package archive

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
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

func (p *PackageInfo) PkgName() string                 { return p.Name }
func (p *PackageInfo) PkgVersion() string              { return p.Version }
func (p *PackageInfo) PkgRevision() int                { return 0 }
func (p *PackageInfo) PkgArch() string                 { return p.Arch }
func (p *PackageInfo) PkgDigestKind() cache.DigestKind { return cache.SHA256 }
func (p *PackageInfo) PkgDigest() string               { return p.SHA256 }

type Options struct {
	Label      string
	Version    string
	Arch       string
	Suites     []string
	Components []string
	Pro        string
	CacheDir   string
	PubKeys    []*packet.PublicKey
	// Maintained is set when the archive is still being updated.
	Maintained bool
	// OldRelease is set for Ubuntu releases which are moved from the regular
	// archive which happens after the release's end of life date.
	OldRelease bool
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
	fetchBulk fetchFlags = 1 << iota
	fetchGzip
	fetchDefault fetchFlags = 0
)

var errNotFound = fmt.Errorf("cannot find archive data")

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
	path := section.Get("Filename")
	digest, digestKind := packageDigest(section)
	logf("Fetching %s...", path)
	reader, err := index.fetch(path, digest, digestKind, fetchBulk)
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
const ubuntuOldReleasesURL = "http://old-releases.ubuntu.com/ubuntu/"
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

func archiveURL(pro, arch string, oldRelease bool) (string, *credentials, error) {
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

	if oldRelease {
		return ubuntuOldReleasesURL, nil, nil
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

	baseURL, creds, err := archiveURL(options.Pro, options.Arch, options.OldRelease)
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
	// InRelease has no digest to check against (it is verified by its PGP
	// signature below), so the digest kind here is arbitrary.
	reader, err := index.fetch(index.distPath("InRelease"), "", cache.SHA256, fetchDefault)
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

// digestField is an archive checksum field Chisel can verify. Its name
// doubles as the by-hash directory name in the archive layout.
type digestField struct {
	name string
	kind cache.DigestKind
}

// digestFields lists the checksum fields Chisel can verify, in order of
// preference: strongest first. The order also matches the by-hash archive
// layout, where only the by-hash directory of the strongest advertised hash
// is guaranteed to exist.
var digestFields = []digestField{
	{"SHA512", cache.SHA512},
	{"SHA256", cache.SHA256},
}

// findDigest returns the digest recorded for path in the release, along with
// the field it was found in, trying the given fields in order.
func findDigest(release control.Section, path string, order []digestField) (digest string, field digestField) {
	for _, f := range order {
		if d, _, ok := control.ParsePathInfo(release.Get(f.name), path); ok {
			return d, f
		}
	}
	return "", digestField{}
}

func packageDigest(section control.Section) (digest string, kind cache.DigestKind) {
	for _, f := range digestFields {
		if d := section.Get(f.name); d != "" {
			return d, f.kind
		}
	}
	// No digest advertised; fall back to SHA256 so the package can still be
	// cached and retrieved by its computed digest.
	return "", cache.SHA256
}

func (index *ubuntuIndex) fetchIndex() error {
	packagesPath := fmt.Sprintf("%s/binary-%s/Packages", index.component, index.arch)
	packagesDigest, field := findDigest(index.release, packagesPath, digestFields)
	if packagesDigest == "" {
		return fmt.Errorf("%s is missing from %s %s component digests", packagesPath, index.suite, index.component)
	}

	logf("Fetching index for %s %s %s %s component...", index.displayName(), index.version, index.suite, index.component)

	// Prefer acquire-by-hash when the archive advertises it. By-hash URLs
	// are content-addressed and so are immune to the inconsistent view of
	// InRelease vs Packages.gz that mirrors and CDNs can serve while a
	// publication is propagating. See https://wiki.ubuntu.com/AptByHash.
	packagesGzPath := packagesPath + ".gz"
	var reader io.ReadSeekCloser
	if index.release.Get("Acquire-By-Hash") == "yes" {
		// By-hash directories are only guaranteed to exist for the strongest
		// hash the archive advertises, which is what findDigest prefers. If
		// the archive advertises a hash stronger than any Chisel knows, the
		// URL may 404 and the named-path fallback below applies.
		packagesGzDigest, byHashField := findDigest(index.release, packagesGzPath, digestFields)
		if packagesGzDigest != "" {
			packagesByHashPath := fmt.Sprintf("%s/binary-%s/by-hash/%s/%s", index.component, index.arch, byHashField.name, packagesGzDigest)
			r, err := index.fetch(index.distPath(packagesByHashPath), packagesDigest, field.kind, fetchBulk|fetchGzip)
			if err != nil && err != errNotFound {
				return err
			}
			// On 404 fall through to the named-path fetch below: the hash
			// may have been garbage-collected on the mirror.
			reader = r
		}
	}
	if reader == nil {
		r, err := index.fetch(index.distPath(packagesGzPath), packagesDigest, field.kind, fetchBulk|fetchGzip)
		if err != nil {
			return err
		}
		reader = r
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
	for _, c := range components {
		found := slices.Contains(releaseComponents, c)
		if !found {
			return fmt.Errorf("archive has no component %q", c)
		}
	}
	return nil
}

func (index *ubuntuIndex) distPath(suffix string) string {
	return "dists/" + index.suite + "/" + suffix
}

func (index *ubuntuIndex) fetch(path, digest string, digestKind cache.DigestKind, flags fetchFlags) (io.ReadSeekCloser, error) {
	reader, err := index.archive.cache.Open(digestKind, digest)
	if err == nil {
		return reader, nil
	} else if err != cache.ErrMiss {
		return nil, err
	}

	cleanURL, err := url.JoinPath(index.archive.baseURL, path)
	if err != nil {
		return nil, fmt.Errorf("internal error: cannot construct URL: %v", err)
	}
	req, err := http.NewRequest("GET", cleanURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP request: %v", err)
	}
	creds := index.archive.creds
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
		return nil, errNotFound
	default:
		return nil, fmt.Errorf("error from archive: %v", resp.Status)
	}

	body := resp.Body
	if flags&fetchGzip != 0 {
		reader, err := gzip.NewReader(body)
		if err != nil {
			return nil, fmt.Errorf("cannot decompress data: %v", err)
		}
		defer reader.Close()
		body = reader
	}

	writer := index.archive.cache.Create(digestKind, digest)
	defer writer.Close()

	_, err = io.Copy(writer, body)
	if err == nil {
		err = writer.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("cannot fetch from archive: %v", err)
	}

	return index.archive.cache.Open(digestKind, writer.Digest())
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
