package setup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/juju/fslock"

	"github.com/canonical/chisel/internal/cache"
	"github.com/canonical/chisel/internal/fsutil"
)

type FetchOptions struct {
	Label    string
	Version  string
	CacheDir string
}

var bulkClient = &http.Client{
	Timeout: 5 * time.Minute,
}

const baseURL = "https://codeload.github.com/canonical/chisel-releases/tar.gz/refs/heads/"

func FetchRelease(options *FetchOptions) (*Release, error) {
	logf("Consulting release repository...")

	cacheDir := options.CacheDir
	if cacheDir == "" {
		cacheDir = cache.DefaultDir("chisel")
	}

	dirName := filepath.Join(cacheDir, "releases", options.Label + "-" + options.Version)
	err := os.MkdirAll(dirName, 0755)
	if err == nil {
		lockFile := fslock.New(filepath.Join(cacheDir, "releases", ".lock"))
		err = lockFile.LockWithTimeout(10 * time.Second)
		if err == nil {
			defer lockFile.Unlock()
		}
	}
	if err != nil {
		return nil, fmt.Errorf("cannot create cache directory: %w", err)
	}

	tagName := filepath.Join(dirName, ".etag")
	tagData, err := ioutil.ReadFile(tagName)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	req, err := http.NewRequest("GET", baseURL + options.Label + "-" + options.Version, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request for release information: %w", err)
	}
	req.Header.Add("If-None-Match", string(tagData))

	resp, err := bulkClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot talk to release repository: %w", err)
	}
	defer resp.Body.Close()

	cacheIsValid := false
	switch resp.StatusCode {
	case 200:
		// ok
	case 304:
		cacheIsValid = true
	case 401, 404:
		return nil, fmt.Errorf("no information for %s-%s release", options.Label, options.Version)
	default:
		return nil, fmt.Errorf("error from release repository: %v", resp.Status)
	}

	if cacheIsValid {
		logf("Cached %s-%s release is still up-to-date.", options.Label, options.Version)
	} else {
		logf("Fetching current %s-%s release...", options.Label, options.Version)
		if !strings.Contains(dirName, "/releases/") {
			// Better safe than sorry.
			return nil, fmt.Errorf("internal error: will not remove something unexpected: %s", dirName)
		}
		err = os.RemoveAll(dirName)
		if err != nil {
			return nil, fmt.Errorf("cannot remove previously cached release: %w", err)
		}
		err = extractTarGz(resp.Body, dirName)
		if err != nil {
			return nil, err
		}
		tag := resp.Header.Get("ETag")
		if tag != "" {
			err := ioutil.WriteFile(tagName, []byte(tag), 0644)
			if err != nil {
				return nil, fmt.Errorf("cannot write remote release tag file: %v", err)
			}
		}
	}

	return ReadRelease(dirName)
}

func extractTarGz(dataReader io.Reader, targetDir string) error {
	gzipReader, err := gzip.NewReader(dataReader)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	return extractTar(gzipReader, targetDir)
}

func extractTar(dataReader io.Reader, targetDir string) error {
	tarReader := tar.NewReader(dataReader)
	for {
		tarHeader, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		sourcePath := filepath.Clean(tarHeader.Name)
		if pos := strings.IndexByte(sourcePath, '/'); pos <= 0 || pos == len(sourcePath)-1 || sourcePath[0] == '.' {
			continue
		} else {
			sourcePath = sourcePath[pos+1:]
		}

		//debugf("Extracting header: %#v", tarHeader)

		err = fsutil.Create(&fsutil.CreateOptions{
			Path: filepath.Join(targetDir, sourcePath),
			Mode: tarHeader.FileInfo().Mode(),
			Data: tarReader,
			Link: tarHeader.Linkname,
		})
		if err != nil {
			return err
		}
	}
	return nil
}
