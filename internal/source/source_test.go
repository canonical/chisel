package source_test

import (
	"os"
	"path/filepath"
	"slices"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/source"
	"github.com/canonical/chisel/internal/testutil"
)

var testKey = testutil.PGPKeys["key1"]

type sourceTest struct {
	summary     string
	arch        string
	release     map[string]string
	pkgs        []*testutil.TestPackage
	slices      []setup.SliceKey
	archs       map[string]string
	fetchErrors map[string]string
	error       string
}

var sourceTests = []sourceTest{{
	summary: "Highest priority archive is selected",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name:    "test-package",
		Hash:    "h1",
		Version: "v1",
		Arch:    "amd64",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Reg(0644, "./file", "from foo"),
		}),
		Archives: []string{"foo"},
	}},
	arch: "amd64",
	release: map[string]string{
		"chisel.yaml": testutil.DefaultChiselYamlTwoArchives,
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/file:
		`,
	},
	archs: map[string]string{"test-package": "amd64"},
}, {
	summary: "Pinned archive bypasses higher priority",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name:    "test-package",
		Hash:    "h1",
		Version: "v1",
		Arch:    "amd64",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Reg(0644, "./file", "from foo"),
		}),
		Archives: []string{"foo"},
	}, {
		Name:    "test-package",
		Hash:    "h2",
		Version: "v2",
		Arch:    "amd64",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Reg(0644, "./file", "from bar"),
		}),
		Archives: []string{"bar"},
	}},
	arch: "amd64",
	release: map[string]string{
		"chisel.yaml": testutil.DefaultChiselYamlTwoArchives,
		"slices/mydir/test-package.yaml": `
			package: test-package
			archive: bar
			slices:
				myslice:
					contents:
						/file:
		`,
	},
	archs: map[string]string{"test-package": "amd64"},
}, {
	summary: "Pinned archive not available fails",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name:    "test-package",
		Hash:    "h1",
		Version: "v1",
		Arch:    "amd64",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Reg(0644, "./file", "from foo"),
		}),
		Archives: []string{"foo"},
	}},
	arch: "amd64",
	release: map[string]string{
		"chisel.yaml": testutil.DefaultChiselYamlTwoArchives,
		"slices/mydir/test-package.yaml": `
			package: test-package
			archive: bar
			slices:
				myslice:
					contents:
						/file:
		`,
	},
	error: `cannot find package "test-package" in archive\(s\)`,
}, {
	summary: "No archives have the package",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	pkgs:    []*testutil.TestPackage{},
	arch:    "amd64",
	release: map[string]string{
		"chisel.yaml": testutil.DefaultChiselYamlTwoArchives,
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/file:
		`,
	},
	error: `cannot find package "test-package" in archive\(s\)`,
}, {
	summary: "Negative priority archives are ignored when not explicitly pinned",
	slices:  []setup.SliceKey{{"test-package", "myslice"}},
	pkgs: []*testutil.TestPackage{{
		Name: "test-package",
		Data: testutil.MustMakeDeb([]testutil.TarEntry{
			testutil.Reg(0644, "./file", "from foo"),
		}),
		Archives: []string{"foo"},
	}},
	arch: "amd64",
	release: map[string]string{
		"chisel.yaml": `
			format: v1
			maintenance:
				standard: 2025-01-01
				end-of-life: 2100-01-01
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					priority: -20
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/file:
		`,
	},
	error: `cannot find package "test-package" in archive\(s\)`,
}, {
	summary: "Store package fetching not yet implemented",
	slices:  []setup.SliceKey{{"test-package", "myslice"}, {"bin-store-pkg", "myslice"}},
	arch:    "amd64",
	release: map[string]string{
		"chisel.yaml": testutil.DefaultChiselYamlWithStores,
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/file:
		`,
		"slices/mydir/store-pkg.yaml": `
			package: store-pkg
			store: bin
			default-track: 3.1
			slices:
				myslice:
					contents:
						/dir/store-file:
		`,
	},
	fetchErrors: map[string]string{
		"bin-store-pkg": `cannot fetch package "bin-store-pkg" from store "bin": not implemented`,
	},
}}

func (s *S) TestResolve(c *C) {
	for _, test := range sourceTests {
		c.Logf("Summary: %s", test.summary)

		if _, ok := test.release["chisel.yaml"]; !ok {
			test.release["chisel.yaml"] = testutil.DefaultChiselYaml
		}
		if test.pkgs == nil {
			test.pkgs = []*testutil.TestPackage{{
				Name: "test-package",
				Data: testutil.PackageData["test-package"],
			}}
		}
		for _, pkg := range test.pkgs {
			if pkg.Arch == "" {
				pkg.Arch = "arch"
			}
			if pkg.Hash == "" {
				pkg.Hash = "hash"
			}
			if pkg.Version == "" {
				pkg.Version = "version"
			}
		}

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

		selection, err := setup.Select(release, test.slices, test.arch)
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

		sources, err := source.Resolve(archives, selection)
		if test.error != "" {
			c.Assert(err, ErrorMatches, test.error)
			continue
		}
		c.Assert(err, IsNil)

		for pkgName, arch := range test.archs {
			c.Assert(sources[pkgName].Arch(), Equals, arch)
		}

		for pkgName, fetchErr := range test.fetchErrors {
			_, _, err := sources[pkgName].Fetch()
			c.Assert(err, ErrorMatches, fetchErr)
		}
	}
}
