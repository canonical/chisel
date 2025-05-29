package main_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	. "gopkg.in/check.v1"
	"gopkg.in/yaml.v3"

	chisel "github.com/canonical/chisel/cmd/chisel"
	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/testutil"
)

type checkReleaseArchivesTest struct {
	summary string
	arch    string
	release map[string]string
	pkgs    []*testutil.TestPackage
	stdout  string
	err     string
}

var checkReleaseArchivesTests = []checkReleaseArchivesTest{{
	summary: "No issue found",
	release: map[string]string{
		"chisel.yaml": makeChiselYaml([]string{"ubuntu"}),
		"slices/mydir/pkg-a.yaml": `
			package: pkg-a
			slices:
				myslice:
					contents:
		`,
		"slices/mydir/pkg-b.yaml": `
			package: pkg-b
			slices:
				myslice:
					contents:
		`,
	},
	pkgs: []*testutil.TestPackage{{
		Name: "pkg-a",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0755, "./dir/"),
		}),
	}, {
		Name: "pkg-b",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0755, "./dir/"),
		}),
	}},
	stdout: "",
}, {
	summary: "Parent directory conflicts, all types",
	release: map[string]string{
		"chisel.yaml": makeChiselYaml([]string{"ubuntu"}),
		"slices/mydir/pkg-a.yaml": `
			package: pkg-a
			slices:
				myslice:
					contents:
		`,
		"slices/mydir/pkg-b.yaml": `
			package: pkg-b
			slices:
				myslice:
					contents:
		`,
	},
	pkgs: []*testutil.TestPackage{{
		Name: "pkg-a",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0755, "./mode/"),
			testutil.Lnk(0777, "./link", "/other"),
		}),
	}, {
		Name: "pkg-b",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0756, "./mode/"),
			testutil.Dir(0777, "./link"),
		}),
	}},
	stdout: `
		- issue: parent-directory-conflict
		  path: /link
		  entries:
			- mode: 0777
			  link: /other
			  packages: {ubuntu: [pkg-a]}
			- mode: 0777
			  link: ""
			  packages: {ubuntu: [pkg-b]}
		- issue: parent-directory-conflict
		  path: /mode
		  entries:
			- mode: 0755
			  link: ""
			  packages: {ubuntu: [pkg-a]}
			- mode: 0756
			  link: ""
			  packages: {ubuntu: [pkg-b]}
	`,
	err: "issues found in the release archives",
}, {
	summary: "Multiple archives",
	release: map[string]string{
		"chisel.yaml": makeChiselYaml([]string{"archive1", "archive2"}),
		"slices/mydir/pkg-a.yaml": `
			package: pkg-a
			slices:
				myslice:
					contents:
		`,
		"slices/mydir/pkg-b.yaml": `
			package: pkg-b
			slices:
				myslice:
					contents:
		`,
	},
	pkgs: []*testutil.TestPackage{{
		Name: "pkg-a",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0755, "./dir/"),
		}),
		Archives: []string{"archive1", "archive2"},
	}, {
		Name: "pkg-b",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0756, "./dir/"),
		}),
		Archives: []string{"archive2"},
	}},
	stdout: `
		- issue: parent-directory-conflict
		  path: /dir
		  entries:
			- mode: 0755
			  link: ""
			  packages: {archive1: [pkg-a], archive2: [pkg-a]}
			- mode: 0756
			  link: ""
			  packages: {archive2: [pkg-b]}
	`,
	err: "issues found in the release archives",
}, {
	summary: "Parent directory conflict different arch",
	release: map[string]string{
		"chisel.yaml": makeChiselYaml([]string{"ubuntu"}),
		"slices/mydir/pkg-a.yaml": `
			package: pkg-a
			slices:
				myslice:
					contents:
		`,
		"slices/mydir/pkg-b.yaml": `
			package: pkg-b
			slices:
				myslice:
					contents:
		`,
	},
	arch: "arm64",
	pkgs: []*testutil.TestPackage{{
		Name: "pkg-a",
		Arch: "arm64",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0755, "./dir/"),
		}),
	}, {
		Name: "pkg-b",
		Arch: "arm64",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Dir(0756, "./dir/"),
		}),
	}},
	stdout: `
		- issue: parent-directory-conflict
		  path: /dir
		  entries:
			- mode: 0755
			  link: ""
			  packages: {ubuntu: [pkg-a]}
			- mode: 0756
			  link: ""
			  packages: {ubuntu: [pkg-b]}
	`,
	err: "issues found in the release archives",
}}

func (s *ChiselSuite) TestRun(c *C) {
	for _, test := range checkReleaseArchivesTests {
		c.Logf("Summary: %s", test.summary)
		s.ResetStdStreams()

		releaseDir := c.MkDir()
		for path, data := range test.release {
			fpath := filepath.Join(releaseDir, path)
			err := os.MkdirAll(filepath.Dir(fpath), 0755)
			c.Assert(err, IsNil)
			err = os.WriteFile(fpath, testutil.Reindent(data), 0644)
			c.Assert(err, IsNil)
		}

		release, err := setup.ReadRelease(releaseDir)
		c.Assert(err, IsNil)

		archives := map[string]archive.Archive{}
		for name, setupArchive := range release.Archives {
			pkgs := make(map[string]*testutil.TestPackage)
			for _, pkg := range test.pkgs {
				if len(pkg.Archives) == 0 || slices.Contains(pkg.Archives, name) {
					pkgs[pkg.Name] = pkg
				}
			}
			archive := &testutil.TestArchive{
				Opts: archive.Options{
					Label:      setupArchive.Name,
					Version:    setupArchive.Version,
					Suites:     setupArchive.Suites,
					Components: setupArchive.Components,
					Pro:        setupArchive.Pro,
					Arch:       test.arch,
				},
				Packages: pkgs,
			}
			archives[name] = archive
		}

		restore := chisel.FakeArchiveOpen(func(options *archive.Options) (archive.Archive, error) {
			archive, ok := archives[options.Label]
			c.Assert(ok, Equals, true)
			c.Assert(archive.Options().Arch, Equals, options.Arch)
			c.Assert(archive.Options().Pro, Equals, options.Pro)
			c.Assert(archive.Options().Label, Equals, options.Label)
			c.Assert(archive.Options().Version, Equals, options.Version)
			c.Assert(archive.Options().Components, DeepEquals, options.Components)
			c.Assert(archive.Options().Suites, DeepEquals, options.Suites)
			return archive, nil
		})
		defer restore()

		cliArgs := []string{"debug", "check-release-archives", "--release", releaseDir}
		if test.arch != "" {
			cliArgs = slices.Concat(cliArgs, []string{"--arch", test.arch})
		}

		_, err = chisel.Parser().ParseArgs(cliArgs)
		if test.err != "" {
			c.Assert(err, ErrorMatches, test.err)
		} else {
			c.Assert(err, IsNil)
		}
		if test.stdout != "" {
			test.stdout = string(testutil.Reindent(test.stdout))
			test.stdout = strings.TrimSpace(test.stdout) + "\n"
		}
		c.Assert(s.Stdout(), Equals, test.stdout)
	}
}

// makeChiselYaml returns a valid chisel.yaml that contains the archives
// supplied.
func makeChiselYaml(archives []string) string {
	archiveKey := testutil.PGPKeys["key-ubuntu-2018"]
	rawChiselYaml := testutil.Reindent(`
		format: v1
		archives:
			ubuntu:
				version: 22.04
				components: [main, universe]
				suites: [jammy]
				public-keys: [test-key]
		public-keys:
			test-key:
				id: ` + archiveKey.ID + `
				armor: |` + "\n" + testutil.PrefixEachLine(archiveKey.PubKeyArmor, "\t\t\t\t\t\t"))

	chiselYaml := map[string]any{}
	err := yaml.Unmarshal([]byte(rawChiselYaml), chiselYaml)
	if err != nil {
		panic(err)
	}

	archivesYaml := chiselYaml["archives"].(map[string]any)
	// Use the ubuntuArchive as a "template".
	ubuntuArchive := archivesYaml["ubuntu"].(map[string]any)
	delete(archivesYaml, "ubuntu")

	for i, archiveName := range archives {
		archive := deepCopyYAML(ubuntuArchive)
		// Valid chisel.yaml has different priorities.
		archive["priority"] = i + 1
		archivesYaml[archiveName] = archive
	}

	bs, err := yaml.Marshal(chiselYaml)
	if err != nil {
		panic(err)
	}
	return string(bs)
}

func deepCopyYAML(src map[string]any) map[string]any {
	dest := map[string]any{}
	for key, value := range src {
		switch src[key].(type) {
		case map[string]any:
			dest[key] = map[string]any{}
			dest[key] = deepCopyYAML(src[key].(map[string]any))
		default:
			dest[key] = value
		}
	}
	return dest
}
