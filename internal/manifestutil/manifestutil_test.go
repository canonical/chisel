package manifestutil_test

import (
	"bytes"
	"io"
	"io/fs"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/apachetestutil"
	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/manifestutil"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/pkg/manifest"
)

var findPathsTests = []struct {
	summary  string
	slices   []*setup.Slice
	expected map[string][]string
}{{
	summary: "Single slice",
	slices: []*setup.Slice{{
		Name: "slice1",
		Contents: map[string]setup.PathInfo{
			"/folder/**": {
				Kind:     "generate",
				Generate: "manifest",
			},
		},
	}},
	expected: map[string][]string{
		"/folder/manifest.wall": []string{"slice1"},
	},
}, {
	summary: "No slice matched",
	slices: []*setup.Slice{{
		Name:     "slice1",
		Contents: map[string]setup.PathInfo{},
	}},
	expected: map[string][]string{},
}, {
	summary: "Several matches with several groups",
	slices: []*setup.Slice{{
		Name: "slice1",
		Contents: map[string]setup.PathInfo{
			"/folder/**": {
				Kind:     "generate",
				Generate: "manifest",
			},
		},
	}, {
		Name: "slice2",
		Contents: map[string]setup.PathInfo{
			"/folder/**": {
				Kind:     "generate",
				Generate: "manifest",
			},
		},
	}, {
		Name:     "slice3",
		Contents: map[string]setup.PathInfo{},
	}, {
		Name: "slice4",
		Contents: map[string]setup.PathInfo{
			"/other-folder/**": {
				Kind:     "generate",
				Generate: "manifest",
			},
		},
	}, {
		Name: "slice5",
		Contents: map[string]setup.PathInfo{
			"/other-folder/**": {
				Kind:     "generate",
				Generate: "manifest",
			},
		},
	}},
	expected: map[string][]string{
		"/folder/manifest.wall":       {"slice1", "slice2"},
		"/other-folder/manifest.wall": {"slice4", "slice5"},
	},
}}

func (s *S) TestFindPaths(c *C) {
	for _, test := range findPathsTests {
		c.Logf("Summary: %s", test.summary)

		manifestSlices := manifestutil.FindPaths(test.slices)

		slicesByName := map[string]*setup.Slice{}
		for _, slice := range test.slices {
			_, ok := slicesByName[slice.Name]
			c.Assert(ok, Equals, false, Commentf("duplicated slice name"))
			slicesByName[slice.Name] = slice
		}

		c.Assert(manifestSlices, HasLen, len(test.expected))
		for path, slices := range manifestSlices {
			c.Assert(slices, HasLen, len(test.expected[path]))
			for i, sliceName := range test.expected[path] {
				c.Assert(slicesByName[sliceName], DeepEquals, slices[i])
			}
		}
	}
}

var slice1 = &setup.Slice{
	Package: "package1",
	Name:    "slice1",
}
var slice2 = &setup.Slice{
	Package: "package2",
	Name:    "slice2",
}

var generateManifestTests = []struct {
	summary     string
	report      *manifestutil.Report
	packageInfo []*archive.PackageInfo
	selection   []*setup.Slice
	expected    *apachetestutil.ManifestContents
	error       string
}{{
	summary:   "Basic",
	selection: []*setup.Slice{slice1, slice2},
	report: &manifestutil.Report{
		Root: "/",
		Entries: map[string]manifestutil.ReportEntry{
			"/file": {
				Path:        "/file",
				Mode:        0456,
				SHA256:      "hash",
				Size:        1234,
				Slices:      map[*setup.Slice]bool{slice1: true},
				FinalSHA256: "final-hash",
			},
			"/link": {
				Path:   "/link",
				Mode:   0567 | fs.ModeSymlink,
				Link:   "/target",
				Slices: map[*setup.Slice]bool{slice1: true, slice2: true},
			},
		},
	},
	packageInfo: []*archive.PackageInfo{{
		Name:    "package1",
		Version: "v1",
		Arch:    "a1",
		SHA256:  "s1",
	}, {
		Name:    "package2",
		Version: "v2",
		Arch:    "a2",
		SHA256:  "s2",
	}},
	expected: &apachetestutil.ManifestContents{
		Paths: []*manifest.Path{{
			Kind:        "path",
			Path:        "/file",
			Mode:        "0456",
			Slices:      []string{"package1_slice1"},
			Size:        1234,
			SHA256:      "hash",
			FinalSHA256: "final-hash",
		}, {
			Kind:   "path",
			Path:   "/link",
			Link:   "/target",
			Mode:   "0567",
			Slices: []string{"package1_slice1", "package2_slice2"},
		}},
		Packages: []*manifest.Package{{
			Kind:    "package",
			Name:    "package1",
			Version: "v1",
			Digest:  "s1",
			Arch:    "a1",
		}, {
			Kind:    "package",
			Name:    "package2",
			Version: "v2",
			Digest:  "s2",
			Arch:    "a2",
		}},
		Slices: []*manifest.Slice{{
			Kind: "slice",
			Name: "package1_slice1",
		}, {
			Kind: "slice",
			Name: "package2_slice2",
		}},
		Contents: []*manifest.Content{{
			Kind:  "content",
			Slice: "package1_slice1",
			Path:  "/file",
		}, {
			Kind:  "content",
			Slice: "package1_slice1",
			Path:  "/link",
		}, {
			Kind:  "content",
			Slice: "package2_slice2",
			Path:  "/link",
		}},
	},
}, {
	summary: "Missing slice",
	report: &manifestutil.Report{
		Root: "/",
		Entries: map[string]manifestutil.ReportEntry{
			"/file": {
				Path:        "/file",
				Mode:        0456,
				SHA256:      "hash",
				Size:        1234,
				Slices:      map[*setup.Slice]bool{slice1: true},
				FinalSHA256: "final-hash",
			},
		},
	},
	selection: []*setup.Slice{},
	error:     `internal error: invalid manifest: path "/file" refers to missing slice package1_slice1`,
}, {
	summary: "Missing package",
	report: &manifestutil.Report{
		Root: "/",
		Entries: map[string]manifestutil.ReportEntry{
			"/file": {
				Path:        "/file",
				Mode:        0456,
				SHA256:      "hash",
				Size:        1234,
				Slices:      map[*setup.Slice]bool{slice1: true},
				FinalSHA256: "final-hash",
			},
		},
	},
	packageInfo: []*archive.PackageInfo{},
	error:       `internal error: invalid manifest: slice package1_slice1 refers to missing package "package1"`,
}, {
	summary: "Invalid path: slices is empty",
	report: &manifestutil.Report{
		Root: "/",
		Entries: map[string]manifestutil.ReportEntry{
			"/file": {
				Path: "/file",
				Mode: 0456,
			},
		},
	},
	error: `internal error: invalid manifest: path "/file" has invalid options: slices is empty`,
}, {
	summary: "Invalid path: link set for symlink",
	report: &manifestutil.Report{
		Root: "/",
		Entries: map[string]manifestutil.ReportEntry{
			"/link": {
				Path:   "/link",
				Mode:   0456 | fs.ModeSymlink,
				Slices: map[*setup.Slice]bool{slice1: true},
			},
		},
	},
	error: `internal error: invalid manifest: path "/link" has invalid options: link not set for symlink`,
}, {
	summary: "Invalid path: sha256 set for symlink",
	report: &manifestutil.Report{
		Root: "/",
		Entries: map[string]manifestutil.ReportEntry{
			"/link": {
				Path:   "/link",
				Mode:   0456 | fs.ModeSymlink,
				Slices: map[*setup.Slice]bool{slice1: true},
				Link:   "valid",
				SHA256: "not-empty",
			},
		},
	},
	error: `internal error: invalid manifest: path "/link" has invalid options: sha256 set for symlink`,
}, {
	summary: "Invalid path: final_sha256 set for symlink",
	report: &manifestutil.Report{
		Root: "/",
		Entries: map[string]manifestutil.ReportEntry{
			"/link": {
				Path:        "/link",
				Mode:        0456 | fs.ModeSymlink,
				Slices:      map[*setup.Slice]bool{slice1: true},
				Link:        "valid",
				FinalSHA256: "not-empty",
			},
		},
	},
	error: `internal error: invalid manifest: path "/link" has invalid options: final_sha256 set for symlink`,
}, {
	summary: "Invalid path: size set for symlink",
	report: &manifestutil.Report{
		Root: "/",
		Entries: map[string]manifestutil.ReportEntry{
			"/link": {
				Path:   "/link",
				Mode:   0456 | fs.ModeSymlink,
				Slices: map[*setup.Slice]bool{slice1: true},
				Link:   "valid",
				Size:   1234,
			},
		},
	},
	error: `internal error: invalid manifest: path "/link" has invalid options: size set for symlink`,
}, {
	summary: "Invalid path: link set for directory",
	report: &manifestutil.Report{
		Root: "/",
		Entries: map[string]manifestutil.ReportEntry{
			"/dir": {
				Path:   "/dir",
				Mode:   0456 | fs.ModeDir,
				Slices: map[*setup.Slice]bool{slice1: true},
				Link:   "not-empty",
			},
		},
	},
	error: `internal error: invalid manifest: path "/dir" has invalid options: link set for directory`,
}, {
	summary: "Invalid path: sha256 set for directory",
	report: &manifestutil.Report{
		Root: "/",
		Entries: map[string]manifestutil.ReportEntry{
			"/dir": {
				Path:   "/dir",
				Mode:   0456 | fs.ModeDir,
				Slices: map[*setup.Slice]bool{slice1: true},
				SHA256: "not-empty",
			},
		},
	},
	error: `internal error: invalid manifest: path "/dir" has invalid options: sha256 set for directory`,
}, {
	summary: "Invalid path: final_sha256 set for directory",
	report: &manifestutil.Report{
		Root: "/",
		Entries: map[string]manifestutil.ReportEntry{
			"/dir": {
				Path:        "/dir",
				Mode:        0456 | fs.ModeDir,
				Slices:      map[*setup.Slice]bool{slice1: true},
				FinalSHA256: "not-empty",
			},
		},
	},
	error: `internal error: invalid manifest: path "/dir" has invalid options: final_sha256 set for directory`,
}, {
	summary: "Invalid path: size set for directory",
	report: &manifestutil.Report{
		Root: "/",
		Entries: map[string]manifestutil.ReportEntry{
			"/dir": {
				Path:   "/dir",
				Mode:   0456 | fs.ModeDir,
				Slices: map[*setup.Slice]bool{slice1: true},
				Size:   1234,
			},
		},
	},
	error: `internal error: invalid manifest: path "/dir" has invalid options: size set for directory`,
}, {
	summary:   "Basic hard link",
	selection: []*setup.Slice{slice1},
	report: &manifestutil.Report{
		Root: "/",
		Entries: map[string]manifestutil.ReportEntry{
			"/file": {
				Path:        "/file",
				Mode:        0456,
				SHA256:      "hash",
				Size:        1234,
				Slices:      map[*setup.Slice]bool{slice1: true},
				FinalSHA256: "final-hash",
				Inode:       1,
			},
			"/hardlink": {
				Path:        "/hardlink",
				Mode:        0456,
				SHA256:      "hash",
				Size:        1234,
				Slices:      map[*setup.Slice]bool{slice1: true},
				FinalSHA256: "final-hash",
				Inode:       1,
			},
		},
	},
	packageInfo: []*archive.PackageInfo{{
		Name:    "package1",
		Version: "v1",
		Arch:    "a1",
		SHA256:  "s1",
	}},
	expected: &apachetestutil.ManifestContents{
		Paths: []*manifest.Path{{
			Kind:        "path",
			Path:        "/file",
			Mode:        "0456",
			Slices:      []string{"package1_slice1"},
			Size:        1234,
			SHA256:      "hash",
			FinalSHA256: "final-hash",
			Inode:       1,
		}, {
			Kind:        "path",
			Path:        "/hardlink",
			Mode:        "0456",
			Slices:      []string{"package1_slice1"},
			Size:        1234,
			SHA256:      "hash",
			FinalSHA256: "final-hash",
			Inode:       1,
		}},
		Packages: []*manifest.Package{{
			Kind:    "package",
			Name:    "package1",
			Version: "v1",
			Digest:  "s1",
			Arch:    "a1",
		}},
		Slices: []*manifest.Slice{{
			Kind: "slice",
			Name: "package1_slice1",
		}},
		Contents: []*manifest.Content{{
			Kind:  "content",
			Slice: "package1_slice1",
			Path:  "/file",
		}, {
			Kind:  "content",
			Slice: "package1_slice1",
			Path:  "/hardlink",
		}},
	},
}, {
	summary: "Skipped hard link id",
	report: &manifestutil.Report{
		Root: "/",
		Entries: map[string]manifestutil.ReportEntry{
			"/file": {
				Path:   "/file",
				Slices: map[*setup.Slice]bool{slice1: true},
				Inode:  2,
			},
		},
	},
	error: `internal error: invalid manifest: cannot find hard link id 1`,
}, {
	summary: "Hard link group has only one path",
	report: &manifestutil.Report{
		Root: "/",
		Entries: map[string]manifestutil.ReportEntry{
			"/file": {
				Path:   "/file",
				Slices: map[*setup.Slice]bool{slice1: true},
				Inode:  1,
			},
		},
	},
	error: `internal error: invalid manifest: hard link group 1 has only one path: /file`,
}, {
	summary: "Hard linked paths differ",
	report: &manifestutil.Report{
		Root: "/",
		Entries: map[string]manifestutil.ReportEntry{
			"/file": {
				Path:   "/file",
				Mode:   0456,
				SHA256: "hash",
				Size:   1234,
				Slices: map[*setup.Slice]bool{slice1: true},
				Inode:  1,
			},
			"/hardlink": {
				Path:   "/hardlink",
				Mode:   0456,
				SHA256: "different-hash",
				Size:   1234,
				Slices: map[*setup.Slice]bool{slice1: true},
				Inode:  1,
			},
		},
	},
	error: `internal error: invalid manifest: hard linked paths "/file" and "/hardlink" have diverging contents`,
}, {
	summary: "Invalid package: missing name",
	packageInfo: []*archive.PackageInfo{{
		Version: "v1",
		Arch:    "a1",
		SHA256:  "s1",
	}},
	error: `internal error: invalid manifest: package name not set`,
}, {
	summary: "Invalid package: missing version",
	packageInfo: []*archive.PackageInfo{{
		Name:   "package-1",
		Arch:   "a1",
		SHA256: "s1",
	}},
	error: `internal error: invalid manifest: package "package-1" missing version`,
}, {
	summary: "Invalid package: missing arch",
	packageInfo: []*archive.PackageInfo{{
		Name:    "package-1",
		Version: "v1",
		SHA256:  "s1",
	}},
	error: `internal error: invalid manifest: package "package-1" missing arch`,
}, {
	summary: "Invalid package: missing sha256",
	packageInfo: []*archive.PackageInfo{{
		Name:    "package-1",
		Version: "v1",
		Arch:    "a1",
	}},
	error: `internal error: invalid manifest: package "package-1" missing sha256`,
}}

func (s *S) TestGenerateManifests(c *C) {
	for _, test := range generateManifestTests {
		c.Logf(test.summary)
		if test.selection == nil {
			test.selection = []*setup.Slice{slice1}
		}
		if test.packageInfo == nil {
			test.packageInfo = []*archive.PackageInfo{{
				Name:    "package1",
				Version: "v1",
				Arch:    "a1",
				SHA256:  "s1",
			}}
		}

		options := &manifestutil.WriteOptions{
			PackageInfo: test.packageInfo,
			Selection:   test.selection,
			Report:      test.report,
		}
		var buffer bytes.Buffer
		err := manifestutil.Write(options, &buffer)
		if test.error != "" {
			c.Assert(err, ErrorMatches, test.error)
			continue
		}
		c.Assert(err, IsNil)
		mfest, err := manifest.Read(&buffer)
		c.Assert(err, IsNil)
		err = manifest.Validate(mfest)
		c.Assert(err, IsNil)
		contents := apachetestutil.DumpManifestContents(c, mfest)
		c.Assert(contents, DeepEquals, test.expected)
	}
}

func (s *S) TestGenerateNoManifests(c *C) {
	report, err := manifestutil.NewReport("/")
	c.Assert(err, IsNil)
	options := &manifestutil.WriteOptions{
		Report: report,
	}
	var buffer bytes.Buffer
	err = manifestutil.Write(options, &buffer)
	c.Assert(err, IsNil)

	var reader io.Reader = &buffer
	var bs []byte
	n, err := reader.Read(bs)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 0)
}
