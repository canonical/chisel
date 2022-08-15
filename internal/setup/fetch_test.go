package setup_test

import (
	. "gopkg.in/check.v1"

	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/canonical/chisel/internal/setup"
)

// TODO Implement local test server instead of using live repository.

func (s *S) TestFetch(c *C) {
	options := &setup.FetchOptions{
		Label: "ubuntu",
		Version: "22.04",
		CacheDir: c.MkDir(),
	}

	for fetch := 0; fetch < 3; fetch++ {
		release, err := setup.FetchRelease(options)
		c.Assert(err, IsNil)

		c.Assert(release.Path, Equals, filepath.Join(options.CacheDir, "releases", "ubuntu-22.04"))

		archive := release.Archives["ubuntu"]
		c.Assert(archive.Name, Equals, "ubuntu")
		c.Assert(archive.Version, Equals, "22.04")

		// Fetch multiple times and use a marker file inside
		// the release directory to check if caching is both
		// preserving and cleaning it when appropriate.
		markerPath := filepath.Join(release.Path, "test.marker")
		switch fetch {
		case 0:
			err := ioutil.WriteFile(markerPath, nil, 0644)
			c.Assert(err, IsNil)
		case 1:
			_, err := ioutil.ReadFile(markerPath)
			c.Assert(err, IsNil)

			err = ioutil.WriteFile(filepath.Join(release.Path, ".etag"), []byte("wrong"), 0644)
			c.Assert(err, IsNil)
		case 2:
			_, err := ioutil.ReadFile(markerPath)
			c.Assert(os.IsNotExist(err), Equals, true)
		}
	}
}
