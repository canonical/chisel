package archive

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/canonical/chisel/internal/cache"
	"github.com/canonical/chisel/internal/control"
)

type Archive interface {
	Fetch(pkg string) (io.ReadCloser, error)
	Exists(pkg string) bool
}

type Options struct {
	Label    string
	Version  string
	CacheDir string
	Arch     string
}

func Open(options *Options) (Archive, error) {
	if options.Label != "ubuntu" {
		return nil, fmt.Errorf("non-ubuntu archives are not supported yet")
	}
	if options.Arch == "" {
		return nil, fmt.Errorf("archive architecture is missing")
	}
	return openUbuntu(options)
}

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

var bulkClient = &http.Client{
	Timeout: 5 * time.Minute,
}

var ubuntuAnimals = map[string]string{
	"18.04": "bionic",
	"20.04": "focal",
	"22.04": "jammy",
	"22.10": "kinetic",
}

type ubuntuArchive struct {
	animal     string
	options    Options
	baseURL    string
	release    control.Section
	components []string
	packages   map[string]control.File
	cache      cache.Cache
}

const ubuntuURL = "http://archive.ubuntu.com/ubuntu/"
const ubuntuPortsURL = "http://ports.ubuntu.com/ubuntu-ports/"

func openUbuntu(options *Options) (Archive, error) {
	animal := ubuntuAnimals[options.Version]
	if animal == "" {
		return nil, fmt.Errorf("no data about Ubuntu version %s", options.Version)
	}

	var baseURL string
	switch options.Arch {
	case "amd64", "i386":
		baseURL = ubuntuURL
	default:
		baseURL = ubuntuPortsURL
	}

	archive := &ubuntuArchive{
		animal:  animal,
		options: *options,
		baseURL: baseURL,
		cache: cache.Cache{
			Dir: options.CacheDir,
		},
	}

	logf("Fetching %s %s archive details...", options.Label, options.Version)
	reader, err := archive.fetch("Release", "")
	if err != nil {
		return nil, err
	}

	ctrl, err := control.ParseReader("Label", reader)
	if err != nil {
		return nil, fmt.Errorf("parsing archive Release file: %v", err)
	}
	section := ctrl.Section("Ubuntu")
	if section == nil {
		return nil, fmt.Errorf("corrupted archive Release file: no Ubuntu section")
	}
	logf("Release date: %s", section.Get("Date"))

	archive.release = section
	archive.packages = make(map[string]control.File)
	//archive.components = strings.Fields(section.Get("Components"))
	archive.components = []string{"main", "universe"}

	digests := archive.release.Get("SHA256")
	for _, component := range archive.components {
		packagesPath := fmt.Sprintf("%s/binary-%s/Packages", component, options.Arch)
		digest, _, _ := control.ParsePathInfo(digests, packagesPath)
		logf("Fetching %s %s %s component...", options.Label, options.Version, component)
		reader, err = archive.fetch(packagesPath+".gz", digest)
		if err != nil {
			return nil, err
		}

		ctrl, err := control.ParseReader("Package", reader)
		if err != nil {
			return nil, fmt.Errorf("parsing archive Package file: %v", err)
		}
		archive.packages[component] = ctrl
	}

	return archive, nil
}

func (a *ubuntuArchive) Exists(pkg string) bool {
	for _, component := range a.components {
		section := a.packages[component].Section(pkg)
		if section != nil && section.Get("Filename") != "" {
			return true
		}
	}
	return false
}

func (a *ubuntuArchive) Fetch(pkg string) (io.ReadCloser, error) {
	for _, component := range a.components {
		section := a.packages[component].Section(pkg)
		if section != nil {
			suffix := section.Get("Filename")
			if suffix == "" {
				return nil, fmt.Errorf("package %q has no filename in archive", pkg)
			}
			logf("Fetching %s...", suffix)
			reader, err := a.fetch("../../"+suffix, section.Get("SHA256"))
			if err != nil {
				return nil, err
			}
			return reader, nil
		}
	}
	return nil, fmt.Errorf("cannot find package %q in archive", pkg)
}

func (a *ubuntuArchive) fetch(suffix, digest string) (io.ReadCloser, error) {
	reader, err := a.cache.Open(digest)
	if err == nil {
		return reader, nil
	} else if err != cache.MissErr {
		return nil, err
	}

	var url string
	if strings.HasPrefix(suffix, "pool/") {
		url = a.baseURL + suffix
	} else {
		url = a.baseURL + "dists/" + a.animal + "/" + suffix
	}

	resp, err := httpClient.Get(url)
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

	writer := a.cache.Create(digest)
	defer writer.Close()

	_, err = io.Copy(writer, body)
	if err == nil {
		err = writer.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("cannot fetch from archive: %v", err)
	}

	return a.cache.Open(writer.Digest())
}
