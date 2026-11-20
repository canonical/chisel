package main_test

import (
	"os"
	"path/filepath"
	"strings"

	. "gopkg.in/check.v1"

	chisel "github.com/canonical/chisel/cmd/chisel"
)

var parseOSReleaseTests = []struct {
	summary string
	data    string
	label   string
	version string
}{{
	summary: "Ubuntu os-release",
	data: `PRETTY_NAME="Ubuntu 24.04.1 LTS"
NAME="Ubuntu"
VERSION_ID="24.04"
VERSION="24.04.1 LTS (Noble Numbat)"
VERSION_CODENAME=noble
ID=ubuntu
ID_LIKE=debian
UBUNTU_CODENAME=noble
`,
	label:   "ubuntu",
	version: "24.04",
}, {
	summary: "Unquoted values and mixed case ID",
	data:    "ID=Ubuntu\nVERSION_ID=26.10\n",
	label:   "ubuntu",
	version: "26.10",
}, {
	summary: "Single quoted values",
	data:    "ID='ubuntu'\nVERSION_ID='26.10'\n",
	label:   "ubuntu",
	version: "26.10",
}, {
	summary: "Missing VERSION_ID",
	data:    "ID=ubuntu\nVERSION=\"26.10 (Stonking Stoat)\"\n",
	label:   "ubuntu",
	version: "",
}, {
	summary: "Empty file",
	data:    "",
	label:   "",
	version: "",
}}

func (s *ChiselSuite) TestParseOSRelease(c *C) {
	for _, test := range parseOSReleaseTests {
		c.Logf("Summary: %s", test.summary)
		label, version := chisel.ParseOSRelease([]byte(test.data))
		c.Assert(label, Equals, test.label)
		c.Assert(version, Equals, test.version)
	}
}

const (
	hostOSRelease = "ID=ubuntu\nVERSION_ID=\"26.10\"\n"
	baseOSRelease = "ID=ubuntu-core\nVERSION_ID=\"22\"\n"
)

var readOSReleaseTests = []struct {
	summary string
	// Entries with a "symlink:" prefix are created as symlinks to the rest
	// of the value, all others as regular files with the value as content.
	files   map[string]string
	label   string
	version string
	error   string
}{{
	summary: "Regular file in /etc",
	files: map[string]string{
		"/etc/os-release": hostOSRelease,
	},
	label:   "ubuntu",
	version: "26.10",
}, {
	summary: "Relative symlink from /etc to /usr/lib",
	files: map[string]string{
		"/etc/os-release":     "symlink:../usr/lib/os-release",
		"/usr/lib/os-release": hostOSRelease,
	},
	label:   "ubuntu",
	version: "26.10",
}, {
	summary: "Absolute symlink is resolved against root",
	files: map[string]string{
		"/etc/os-release":     "symlink:/usr/lib/os-release",
		"/usr/lib/os-release": hostOSRelease,
	},
	label:   "ubuntu",
	version: "26.10",
}, {
	summary: "Relative symlink cannot climb out of root",
	files: map[string]string{
		"/etc/os-release":     "symlink:../../../../../../usr/lib/os-release",
		"/usr/lib/os-release": hostOSRelease,
	},
	label:   "ubuntu",
	version: "26.10",
}, {
	summary: "Chained symlinks",
	files: map[string]string{
		"/etc/os-release":           "symlink:os-release.d/current",
		"/etc/os-release.d/current": "symlink:/usr/lib/os-release",
		"/usr/lib/os-release":       hostOSRelease,
	},
	label:   "ubuntu",
	version: "26.10",
}, {
	summary: "Missing /etc falls back to /usr/lib",
	files: map[string]string{
		"/usr/lib/os-release": hostOSRelease,
	},
	label:   "ubuntu",
	version: "26.10",
}, {
	summary: "Dangling /etc symlink falls back to /usr/lib",
	files: map[string]string{
		"/etc/os-release":     "symlink:/usr/lib/os-release.d/missing",
		"/usr/lib/os-release": hostOSRelease,
	},
	label:   "ubuntu",
	version: "26.10",
}, {
	summary: "Incomplete /etc does not fall back to /usr/lib",
	files: map[string]string{
		"/etc/os-release":     "ID=ubuntu\n",
		"/usr/lib/os-release": baseOSRelease,
	},
	error: `cannot find ID and VERSION_ID in /etc/os-release`,
}, {
	summary: "Symlink loop",
	files: map[string]string{
		"/etc/os-release":     "symlink:/etc/os-release",
		"/usr/lib/os-release": "symlink:/usr/lib/os-release",
	},
	error: `too many levels of symbolic links: .*/usr/lib/os-release`,
}, {
	summary: "No os-release",
	files:   map[string]string{},
	error:   `open .*/usr/lib/os-release: no such file or directory`,
}}

func (s *ChiselSuite) TestReadOSRelease(c *C) {
	for _, test := range readOSReleaseTests {
		c.Logf("Summary: %s", test.summary)
		root := c.MkDir()
		for path, value := range test.files {
			fullPath := filepath.Join(root, path)
			err := os.MkdirAll(filepath.Dir(fullPath), 0o755)
			c.Assert(err, IsNil)
			if target, ok := strings.CutPrefix(value, "symlink:"); ok {
				err = os.Symlink(target, fullPath)
			} else {
				err = os.WriteFile(fullPath, []byte(value), 0o644)
			}
			c.Assert(err, IsNil)
		}

		label, version, err := chisel.ReadOSRelease(root)
		if test.error != "" {
			c.Assert(err, ErrorMatches, test.error)
			continue
		}
		c.Assert(err, IsNil)
		c.Assert(label, Equals, test.label)
		c.Assert(version, Equals, test.version)
	}
}
