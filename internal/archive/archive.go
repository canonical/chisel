package archive

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/canonical/chisel/internal/cache"
	"github.com/canonical/chisel/internal/control"
	"github.com/canonical/chisel/internal/deb"
)

type Archive interface {
	Options() *Options
	Fetch(pkg string) (io.ReadCloser, error)
	Version(pkg string) string
}

type Options struct {
	Label      string
	Version    string
	Arch       string
	Suites     []string
	Components []string
	CacheDir   string
	Pro        string
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
	label   string
	baseURL string
	auth    string
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

func (a *ubuntuArchive) Version(pkg string) string {
	_, _, version := a.selectPackage(pkg)
	return version
}

func (a *ubuntuArchive) selectPackage(pkg string) (control.Section, *ubuntuIndex, string) {
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
		return nil, nil, ""
	}
	return selectedSection, selectedIndex, selectedVersion
}

func (a *ubuntuArchive) Fetch(pkg string) (io.ReadCloser, error) {
	section, index, _ := a.selectPackage(pkg)
	if section == nil {
		return nil, fmt.Errorf("cannot find package %q in archive", pkg)
	}
	suffix := section.Get("Filename")
	logf("Fetching %s...", suffix)
	reader, err := index.fetch("../../"+suffix, section.Get("SHA256"))
	if err != nil {
		return nil, err
	}
	return reader, nil
}

const ubuntuURL = "http://archive.ubuntu.com/ubuntu/"
const ubuntuPortsURL = "http://ports.ubuntu.com/ubuntu-ports/"
const ubuntuProURL = "https://esm.ubuntu.com/"

var proLabels = map[string]string{
	"fips-updates": "UbuntuFIPSUpdates",
	"fips":         "UbuntuFIPS",
}

var errNoProCredentials = errors.New("no pro credentials")

func initProArchive(pro string, archive *ubuntuArchive) error {
	baseURL := ubuntuProURL + pro + "/ubuntu/"
	creds, err := findCredentials(baseURL)
	if creds.Empty() {
		if err == nil {
			err = errNoProCredentials
		}
		return err
	}

	// Check that credentials are valid.
	// It appears that only pool/ URLs are protected.
	req, err := http.NewRequest("HEAD", baseURL+"pool/", nil)
	if err != nil {
		return fmt.Errorf("cannot create HTTP request: %w", err)
	}
	req.SetBasicAuth(creds.Username, creds.Password)

	resp, err := httpDo(req)
	if err != nil {
		return fmt.Errorf("cannot talk to the archive: %w", err)
	}
	resp.Body.Close()
	switch resp.StatusCode {
	case 200: // ok
	case 401:
		return fmt.Errorf("cannot authenticate to the archive")
	default:
		return fmt.Errorf("error from the archive: %v", resp.Status)
	}

	archive.baseURL = baseURL
	archive.auth = req.Header.Get("Authorization")
	archive.label = proLabels[pro]

	return nil
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

	archive := &ubuntuArchive{
		options: *options,
		cache: &cache.Cache{
			Dir: options.CacheDir,
		},
	}

	if options.Pro != "" {
		err := initProArchive(options.Pro, archive)
		if err != nil {
			if err != errNoProCredentials {
				logf("cannot open %s Pro archive: %s", options.Pro, err)
			}
			return nil, nil
		}
	} else {
		archive.label = "Ubuntu"
		if options.Arch == "amd64" || options.Arch == "i386" {
			archive.baseURL = ubuntuURL
		} else {
			archive.baseURL = ubuntuPortsURL
		}
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
	logf("Fetching %s %s %s suite details...", index.label, index.version, index.suite)
	reader, err := index.fetch("Release", "")
	if err != nil {
		return err
	}

	ctrl, err := control.ParseReader("Label", reader)
	if err != nil {
		return fmt.Errorf("parsing archive Release file: %v", err)
	}
	section := ctrl.Section(index.archive.label)
	if section == nil {
		return fmt.Errorf("corrupted archive Release file: no Ubuntu section")
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
	reader, err := index.fetch(packagesPath+".gz", digest)
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

func (index *ubuntuIndex) fetch(suffix, digest string) (io.ReadCloser, error) {
	reader, err := index.archive.cache.Open(digest)
	if err == nil {
		return reader, nil
	} else if err != cache.MissErr {
		return nil, err
	}

	var url string
	if strings.HasPrefix(suffix, "pool/") {
		url = index.archive.baseURL + suffix
	} else {
		url = index.archive.baseURL + "dists/" + index.suite + "/" + suffix
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP request: %v", err)
	}
	if index.archive.auth != "" {
		req.Header.Set("Authorization", index.archive.auth)
	}
	resp, err := httpDo(req)
	if err != nil {
		return nil, fmt.Errorf("cannot talk to archive: %v", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		// ok
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
