package manifest_test

import (
	"bytes"
	"io"
	"os"
	"path"
	"slices"
	"strings"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/manifest"
	"github.com/canonical/chisel/internal/setup"
)

type manifestContents struct {
	Paths    []*manifest.Path
	Packages []*manifest.Package
	Slices   []*manifest.Slice
	Contents []*manifest.Content
}

var readManifestTests = []struct {
	summary   string
	input     string
	mfest     *manifestContents
	valError  string
	readError string
}{{
	summary: "All types",
	input: `
		{"jsonwall":"1.0","schema":"1.0","count":13}
		{"kind":"content","slice":"pkg1_manifest","path":"/manifest/manifest.wall"}
		{"kind":"content","slice":"pkg1_myslice","path":"/dir/file"}
		{"kind":"content","slice":"pkg1_myslice","path":"/dir/foo/bar/"}
		{"kind":"content","slice":"pkg1_myslice","path":"/dir/link/file"}
		{"kind":"content","slice":"pkg2_myotherslice","path":"/dir/foo/bar/"}
		{"kind":"package","name":"pkg1","version":"v1","sha256":"hash1","arch":"arch1"}
		{"kind":"package","name":"pkg2","version":"v2","sha256":"hash2","arch":"arch2"}
		{"kind":"path","path":"/dir/file","mode":"0644","slices":["pkg1_myslice"],"sha256":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","final_sha256":"8067926c032c090867013d14fb0eb21ae858344f62ad07086fd32375845c91a6","size":21}
		{"kind":"path","path":"/dir/foo/bar/","mode":"01777","slices":["pkg2_myotherslice","pkg1_myslice"]}
		{"kind":"path","path":"/dir/link/file","mode":"0644","slices":["pkg1_myslice"],"link":"/dir/file"}
		{"kind":"path","path":"/manifest/manifest.wall","mode":"0644","slices":["pkg1_manifest"]}
		{"kind":"slice","name":"pkg1_manifest"}
		{"kind":"slice","name":"pkg1_myslice"}
		{"kind":"slice","name":"pkg2_myotherslice"}
	`,
	mfest: &manifestContents{
		Paths: []*manifest.Path{
			{Kind: "path", Path: "/dir/file", Mode: "0644", Slices: []string{"pkg1_myslice"}, SHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", FinalSHA256: "8067926c032c090867013d14fb0eb21ae858344f62ad07086fd32375845c91a6", Size: 0x15, Link: ""},
			{Kind: "path", Path: "/dir/foo/bar/", Mode: "01777", Slices: []string{"pkg2_myotherslice", "pkg1_myslice"}, SHA256: "", FinalSHA256: "", Size: 0x0, Link: ""},
			{Kind: "path", Path: "/dir/link/file", Mode: "0644", Slices: []string{"pkg1_myslice"}, SHA256: "", FinalSHA256: "", Size: 0x0, Link: "/dir/file"},
			{Kind: "path", Path: "/manifest/manifest.wall", Mode: "0644", Slices: []string{"pkg1_manifest"}, SHA256: "", FinalSHA256: "", Size: 0x0, Link: ""},
		},
		Packages: []*manifest.Package{
			{Kind: "package", Name: "pkg1", Version: "v1", Digest: "hash1", Arch: "arch1"},
			{Kind: "package", Name: "pkg2", Version: "v2", Digest: "hash2", Arch: "arch2"},
		},
		Slices: []*manifest.Slice{
			{Kind: "slice", Name: "pkg1_manifest"},
			{Kind: "slice", Name: "pkg1_myslice"},
			{Kind: "slice", Name: "pkg2_myotherslice"},
		},
		Contents: []*manifest.Content{
			{Kind: "content", Slice: "pkg1_manifest", Path: "/manifest/manifest.wall"},
			{Kind: "content", Slice: "pkg1_myslice", Path: "/dir/file"},
			{Kind: "content", Slice: "pkg1_myslice", Path: "/dir/foo/bar/"},
			{Kind: "content", Slice: "pkg1_myslice", Path: "/dir/link/file"},
			{Kind: "content", Slice: "pkg2_myotherslice", Path: "/dir/foo/bar/"},
		},
	},
}, {
	summary: "Slice not found",
	input: `
		{"jsonwall":"1.0","schema":"1.0","count":1}
		{"kind":"content","slice":"pkg1_manifest","path":"/manifest/manifest.wall"}
	`,
	valError: `invalid manifest: slice pkg1_manifest not found in slices`,
}, {
	summary: "Package not found",
	input: `
		{"jsonwall":"1.0","schema":"1.0","count":1}
		{"kind":"slice","name":"pkg1_manifest"}
	`,
	valError: `invalid manifest: package "pkg1" not found in packages`,
}, {
	summary: "Path not found in contents",
	input: `
		{"jsonwall":"1.0","schema":"1.0","count":1}
		{"kind":"path","path":"/dir/","mode":"01777","slices":["pkg1_myslice"]}
	`,
	valError: `invalid manifest: path /dir/ has no matching entry in contents`,
}, {
	summary: "Content and path have different slices",
	input: `
		{"jsonwall":"1.0","schema":"1.0","count":3}
		{"kind":"content","slice":"pkg1_myotherslice","path":"/dir/"}
		{"kind":"package","name":"pkg1","version":"v1","sha256":"hash1","arch":"arch1"}
		{"kind":"path","path":"/dir/","mode":"01777","slices":["pkg1_myslice"]}
		{"kind":"slice","name":"pkg1_myotherslice"}
	`,
	valError: `invalid manifest: path /dir/ and content have diverging slices: \["pkg1_myslice"\] != \["pkg1_myotherslice"\]`,
}, {
	summary: "Content not found in paths",
	input: `
		{"jsonwall":"1.0","schema":"1.0","count":3}
		{"kind":"content","slice":"pkg1_myslice","path":"/dir/"}
		{"kind":"package","name":"pkg1","version":"v1","sha256":"hash1","arch":"arch1"}
		{"kind":"slice","name":"pkg1_myslice"}
	`,
	valError: `invalid manifest: content path /dir/ has no matching entry in paths`,
}, {
	summary: "Malformed jsonwall",
	input: `
		{"jsonwall":"1.0","schema":"1.0","count":1}
		{"kind":"content", "not valid json"
	`,
	valError: `invalid manifest: cannot read manifest: unexpected end of JSON input`,
}, {
	summary: "Unknown schema",
	input: `
		{"jsonwall":"1.0","schema":"2.0","count":1}
		{"kind":"package","name":"pkg1","version":"v1","sha256":"hash1","arch":"arch1"}
	`,
	readError: `cannot read manifest: unknown schema version "2.0"`,
}}

func (s *S) TestManifestReadValidate(c *C) {
	for _, test := range readManifestTests {
		c.Logf("Summary: %s", test.summary)

		// Reindent the jsonwall to remove leading tabs in each line.
		lines := strings.Split(strings.TrimSpace(test.input), "\n")
		trimmedLines := make([]string, 0, len(lines))
		for _, line := range lines {
			trimmedLines = append(trimmedLines, strings.TrimLeft(line, "\t"))
		}
		test.input = strings.Join(trimmedLines, "\n")
		// Assert that the jsonwall is valid, for the test to be meaningful.
		slices.Sort(trimmedLines)
		orderedInput := strings.Join(trimmedLines, "\n")
		c.Assert(test.input, DeepEquals, orderedInput, Commentf("input jsonwall lines should be ordered"))

		tmpDir := c.MkDir()
		manifestPath := path.Join(tmpDir, "manifest.wall")
		w, err := os.OpenFile(manifestPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		c.Assert(err, IsNil)
		_, err = w.Write([]byte(test.input))
		c.Assert(err, IsNil)
		w.Close()

		r, err := os.OpenFile(manifestPath, os.O_RDONLY, 0644)
		c.Assert(err, IsNil)
		defer r.Close()

		mfest, err := manifest.Read(r)
		if test.readError != "" {
			c.Assert(err, ErrorMatches, test.readError)
			continue
		}
		c.Assert(err, IsNil)
		err = manifest.Validate(mfest)
		if test.valError != "" {
			c.Assert(err, ErrorMatches, test.valError)
			continue
		}
		c.Assert(err, IsNil)
		if test.mfest != nil {
			c.Assert(dumpManifestContents(c, mfest), DeepEquals, test.mfest)
		}
	}
}

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

		manifestSlices := manifest.FindPaths(test.slices)

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

func (s *S) TestGenerateManifests(c *C) {
	slice1 := &setup.Slice{
		Package: "package1",
		Name:    "slice1",
	}
	slice2 := &setup.Slice{
		Package: "package2",
		Name:    "slice2",
	}
	report := &manifest.Report{
		Root: "/",
		Entries: map[string]manifest.ReportEntry{
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
				Mode:   0567,
				Link:   "/target",
				Slices: map[*setup.Slice]bool{slice1: true, slice2: true},
			},
		},
	}
	packageInfo := []*archive.PackageInfo{{
		Name:    "package1",
		Version: "v1",
		Arch:    "a1",
		SHA256:  "s1",
	}, {
		Name:    "package2",
		Version: "v2",
		Arch:    "a2",
		SHA256:  "s2",
	}}

	expected := &manifestContents{
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
	}

	options := &manifest.WriteOptions{
		PackageInfo: packageInfo,
		Selection:   []*setup.Slice{slice1, slice2},
		Report:      report,
	}
	var buffer bytes.Buffer
	err := manifest.Write(options, &buffer)
	c.Assert(err, IsNil)
	mfest, err := manifest.Read(&buffer)
	c.Assert(err, IsNil)
	err = manifest.Validate(mfest)
	c.Assert(err, IsNil)
	contents := dumpManifestContents(c, mfest)
	c.Assert(contents, DeepEquals, expected)
}

func (s *S) TestGenerateNoManifests(c *C) {
	report, err := manifest.NewReport("/")
	c.Assert(err, IsNil)
	options := &manifest.WriteOptions{
		Report: report,
	}
	var buffer bytes.Buffer
	err = manifest.Write(options, &buffer)
	c.Assert(err, IsNil)

	var reader io.Reader = &buffer
	var bs []byte
	n, err := reader.Read(bs)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 0)
}

func dumpManifestContents(c *C, mfest *manifest.Manifest) *manifestContents {
	var slices []*manifest.Slice
	err := mfest.IterateSlices("", func(slice *manifest.Slice) error {
		slices = append(slices, slice)
		return nil
	})
	c.Assert(err, IsNil)

	var pkgs []*manifest.Package
	err = mfest.IteratePackages(func(pkg *manifest.Package) error {
		pkgs = append(pkgs, pkg)
		return nil
	})
	c.Assert(err, IsNil)

	var paths []*manifest.Path
	err = mfest.IteratePaths("", func(path *manifest.Path) error {
		paths = append(paths, path)
		return nil
	})
	c.Assert(err, IsNil)

	var contents []*manifest.Content
	err = mfest.IterateContents("", func(content *manifest.Content) error {
		contents = append(contents, content)
		return nil
	})
	c.Assert(err, IsNil)

	mc := manifestContents{
		Paths:    paths,
		Packages: pkgs,
		Slices:   slices,
		Contents: contents,
	}
	return &mc
}
