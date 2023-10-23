package archive

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	pgperrors "github.com/ProtonMail/go-crypto/openpgp/errors"

	"github.com/canonical/chisel/internal/cache"
	"github.com/canonical/chisel/internal/control"
	"github.com/canonical/chisel/internal/deb"
)

type Archive interface {
	Options() *Options
	Fetch(pkg string) (io.ReadCloser, error)
	Exists(pkg string) bool
}

type Options struct {
	Label      string
	Version    string
	Arch       string
	Suites     []string
	Components []string
	CacheDir   string
	PublicKeys []openpgp.KeyRing
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

func (a *ubuntuArchive) Fetch(pkg string) (io.ReadCloser, error) {
	section, index, err := a.selectPackage(pkg)
	if err != nil {
		return nil, err
	}
	suffix := section.Get("Filename")
	logf("Fetching %s...", suffix)
	reader, err := index.fetch("../../"+suffix, section.Get("SHA256"), fetchBulk)
	if err != nil {
		return nil, err
	}
	return reader, nil
}

const ubuntuURL = "http://archive.ubuntu.com/ubuntu/"
const ubuntuPortsURL = "http://ports.ubuntu.com/ubuntu-ports/"

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

	archive := &ubuntuArchive{
		options: *options,
		cache: &cache.Cache{
			Dir: options.CacheDir,
		},
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
				err := index.parseRelease()
				if err != nil {
					return nil, err
				}
				release = index.release
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

func (index *ubuntuIndex) fetchRelease() ([]byte, error) {
	reader, err := index.fetch("InRelease", "", fetchDefault)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func (index *ubuntuIndex) verifyRelease(signedData []byte) ([]byte, error) {
	block, _ := clearsign.Decode(signedData)
	if block == nil {
		return nil, fmt.Errorf("cannot decode InRelease clear-sign block")
	}
	err := pgperrors.ErrUnknownIssuer
	for _, pubkey := range index.archive.options.PublicKeys {
		_, err = block.VerifySignature(pubkey, nil)
		if err == nil || !errors.Is(err, pgperrors.ErrUnknownIssuer) {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}
	return block.Plaintext, nil
}

func (index *ubuntuIndex) parseRelease() error {
	logf("Fetching %s %s %s suite details...", index.label, index.version, index.suite)
	signed, err := index.fetchRelease()
	if err != nil {
		return fmt.Errorf("cannot fetch release: %w", err)
	}
	content, err := index.verifyRelease(signed)
	if err != nil {
		return fmt.Errorf("cannot verify release: %w", err)
	}
	ctrl, err := control.ParseReader("Label", bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("parsing archive Release file: %v", err)
	}
	section := ctrl.Section("Ubuntu")
	if section == nil {
		section = ctrl.Section("UbuntuProFIPS")
		if section == nil {
			return fmt.Errorf("corrupted archive Release file: no Ubuntu section")
		}
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

	logf("Fetching index for %s %s %s %s component...", index.label, index.version, index.suite, index.component)
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

func (index *ubuntuIndex) fetch(suffix, digest string, flags fetchFlags) (io.ReadCloser, error) {
	reader, err := index.archive.cache.Open(digest)
	if err == nil {
		return reader, nil
	} else if err != cache.MissErr {
		return nil, err
	}

	baseURL := ubuntuURL
	if index.arch != "amd64" && index.arch != "i386" {
		baseURL = ubuntuPortsURL
	}

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
	case 401, 404:
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
