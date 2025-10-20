package setup_test

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/openpgp/packet"
	. "gopkg.in/check.v1"
	"gopkg.in/yaml.v3"

	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/testutil"
)

var (
	testKey      = testutil.PGPKeys["key1"]
	extraTestKey = testutil.PGPKeys["key2"]
)

type setupTest struct {
	summary   string
	input     map[string]string
	release   *setup.Release
	relerror  string
	prefers   map[string]string
	selslices []setup.SliceKey
	selection *setup.Selection
	selerror  string
}

var setupTests = []setupTest{{
	summary: "Ensure file format is expected",
	input: map[string]string{
		"chisel.yaml": `
			format: foobar
		`,
	},
	relerror: `chisel.yaml: unknown format "foobar"`,
}, {
	summary: "Missing archives",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
		`,
	},
	relerror: `chisel.yaml: no archives defined`,
}, {
	summary: "Enforce matching filename and package name",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: myotherpkg
		`,
	},
	relerror: `slices/mydir/mypkg.yaml: filename and 'package' field \("myotherpkg"\) disagree`,
}, {
	summary: "Archive with multiple suites",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			maintenance:
				standard: 2025-01-01
				end-of-life: 2100-01-01
			archives:
				ubuntu:
					version: 22.04
					components: [main, other]
					suites: [jammy, jammy-security]
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy", "jammy-security"},
				Components: []string{"main", "other"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Coverage of multiple path kinds",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice1:
					contents:
						/file/path1:
						/file/path2: {copy: /other/path}
						/file/path3: {symlink: /other/path}
						/file/path4: {text: content, until: mutate}
						/file/path5: {mode: 0755, mutable: true}
						/file/path6/: {make: true}
				myslice2:
					essential:
						- mypkg_myslice1
					contents:
						/another/path:
				myslice3:
					mutate: something
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{
					"myslice1": {
						Package: "mypkg",
						Name:    "myslice1",
						Contents: map[string]setup.PathInfo{
							"/file/path1":  {Kind: "copy"},
							"/file/path2":  {Kind: "copy", Info: "/other/path"},
							"/file/path3":  {Kind: "symlink", Info: "/other/path"},
							"/file/path4":  {Kind: "text", Info: "content", Until: "mutate"},
							"/file/path5":  {Kind: "copy", Mode: 0755, Mutable: true},
							"/file/path6/": {Kind: "dir"},
						},
					},
					"myslice2": {
						Package: "mypkg",
						Name:    "myslice2",
						Essential: map[setup.SliceKey]setup.EssentialInfo{
							{"mypkg", "myslice1"}: {},
						},
						Contents: map[string]setup.PathInfo{
							"/another/path": {Kind: "copy"},
						},
					},
					"myslice3": {
						Package: "mypkg",
						Name:    "myslice3",
						Scripts: setup.SliceScripts{
							Mutate: "something",
						},
					},
				},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Empty contents",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice1:
				myslice2:
					contents:
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{
					"myslice1": {
						Package: "mypkg",
						Name:    "myslice1",
					},
					"myslice2": {
						Package: "mypkg",
						Name:    "myslice2",
					},
				},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Cycles are detected within packages",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice1:
					essential:
						- mypkg_myslice2
				myslice2:
					essential:
						- mypkg_myslice3
				myslice3:
					essential:
						- mypkg_myslice1
		`,
	},
	relerror: `essential loop detected: mypkg_myslice1, mypkg_myslice2, mypkg_myslice3`,
}, {
	summary: "Cycles are detected across packages",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice:
					essential:
						- mypkg2_myslice
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice:
					essential:
						- mypkg3_myslice
		`,
		"slices/mydir/mypkg3.yaml": `
			package: mypkg3
			slices:
				myslice:
					essential:
						- mypkg1_myslice
		`,
	},
	relerror: `essential loop detected: mypkg1_myslice, mypkg2_myslice, mypkg3_myslice`,
}, {
	summary: "Missing package dependency",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice:
					essential:
						- mypkg2_myslice
		`,
	},
	relerror: `mypkg1_myslice requires mypkg2_myslice, but slice is missing`,
}, {
	summary: "Missing slice dependency",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice1:
					essential:
						- mypkg_myslice2
		`,
	},
	relerror: `mypkg_myslice1 requires mypkg_myslice2, but slice is missing`,
}, {
	summary: "Selection with no dependencies",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice1: {}
				myslice2: {essential: [mypkg2_myslice1]}
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice1: {}
				myslice2: {essential: [mypkg1_myslice1]}
		`,
	},
	selslices: []setup.SliceKey{{"mypkg1", "myslice1"}},
	selection: &setup.Selection{
		Slices: []*setup.Slice{{
			Package: "mypkg1",
			Name:    "myslice1",
		}},
	},
}, {
	summary: "Selection with dependencies",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice1: {}
				myslice2: {essential: [mypkg2_myslice1]}
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice1: {}
				myslice2: {essential: [mypkg1_myslice1]}
		`,
	},
	selslices: []setup.SliceKey{{"mypkg2", "myslice2"}},
	selection: &setup.Selection{
		Slices: []*setup.Slice{{
			Package: "mypkg1",
			Name:    "myslice1",
		}, {
			Package: "mypkg2",
			Name:    "myslice2",
			Essential: map[setup.SliceKey]setup.EssentialInfo{
				{"mypkg1", "myslice1"}: {},
			},
		}},
	},
}, {
	summary: "Selection with matching paths don't conflict",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice1:
					contents:
						/path1:
						/path2: {text: same}
						/path3: {symlink: /link}
				myslice2:
					contents:
						/path1: {copy: /path1}
						/path2: {text: same}
						/path3: {symlink: /link}
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice1:
					contents:
						/path2: {text: same}
						/path3: {symlink: /link}
		`,
	},
	selslices: []setup.SliceKey{{"mypkg1", "myslice1"}, {"mypkg1", "myslice2"}, {"mypkg2", "myslice1"}},
}, {
	summary: "Conflicting paths across slices",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice1:
					contents:
						/path1:
				myslice2:
					contents:
						/path1: {copy: /other}
		`,
	},
	relerror: "slices mypkg1_myslice1 and mypkg1_myslice2 conflict on /path1",
}, {
	summary: "Conflicting paths across packages",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice1:
					contents:
						/path1:
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice1:
					contents:
						/path1:
		`,
	},
	relerror: "slices mypkg1_myslice1 and mypkg2_myslice1 conflict on /path1",
}, {
	summary: "Directories must be suffixed with /",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/foo: {make: true}
		`,
	},
	relerror: `slice mypkg_myslice path /foo must end in / for 'make' to be valid`,
}, {
	summary: "Slice path must be clean",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/foo/../:
		`,
	},
	relerror: `slice mypkg_myslice has invalid content path: /foo/../`,
}, {
	summary: "Slice path must be absolute",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						./foo/:
		`,
	},
	relerror: `slice mypkg_myslice has invalid content path: ./foo/`,
}, {
	summary: "Globbing support",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice1:
					contents:
						/file/*:
				myslice2:
					contents:
						/another/**:
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{
					"myslice1": {
						Package: "mypkg",
						Name:    "myslice1",
						Contents: map[string]setup.PathInfo{
							"/file/*": {Kind: "glob"},
						},
					},
					"myslice2": {
						Package: "mypkg",
						Name:    "myslice2",
						Contents: map[string]setup.PathInfo{
							"/another/**": {Kind: "glob"},
						},
					},
				},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Conflicting globs",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice:
					contents:
						/file/f*obar:
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice:
					contents:
						/file/foob*r:
		`,
	},
	relerror: `slices mypkg1_myslice and mypkg2_myslice conflict on /file/f\*obar and /file/foob\*r`,
}, {
	summary: "Conflicting globs and plain copies",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice:
					contents:
						/file/foobar:
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice:
					contents:
						/file/foob*r:
		`,
	},
	relerror: `slices mypkg1_myslice and mypkg2_myslice conflict on /file/foobar and /file/foob\*r`,
}, {
	summary: "Conflicting matching globs",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice:
					contents:
						/file/foob*r:
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice:
					contents:
						/file/foob*r:
		`,
	},
	relerror: `slices mypkg1_myslice and mypkg2_myslice conflict on /file/foob\*r`,
}, {
	summary: "Conflicting globs in same package is okay",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice1:
					contents:
						/file/foob*r:
						/file/f*r:
				myslice2:
					contents:
						/file/foob*r:
						/file/f*obar:
		`,
	},
}, {
	summary: "Invalid glob options",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/file/foob*r: {text: foo}
		`,
	},
	relerror: `slice mypkg_myslice path /file/foob\*r has invalid wildcard options`,
}, {
	summary: "Until is an okay option for globs",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/file/foob*r: {until: mutate}
		`,
	},
}, {
	summary: "Mutable does not work for directories extractions",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/path/: {mutable: true}
		`,
	},
	relerror: `slice mypkg_myslice mutable is not a regular file: /path/`,
}, {
	summary: "Mutable does not work for directory making",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/path/: {make: true, mutable: true}
		`,
	},
	relerror: `slice mypkg_myslice mutable is not a regular file: /path/`,
}, {
	summary: "Mutable does not work for symlinks",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/path: {symlink: /other, mutable: true}
		`,
	},
	relerror: `slice mypkg_myslice mutable is not a regular file: /path`,
}, {
	summary: "Until checks its value for validity",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/path: {until: foo}
		`,
	},
	relerror: `slice mypkg_myslice has invalid 'until' for path /path: "foo"`,
}, {
	summary: "Arch checks its value for validity",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/path: {arch: foo}
		`,
	},
	relerror: `slice mypkg_myslice has invalid 'arch' for path /path: "foo"`,
}, {
	summary: "Arch checks its value for validity",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/path: {arch: [i386, foo]}
		`,
	},
	relerror: `slice mypkg_myslice has invalid 'arch' for path /path: "foo"`,
}, {
	summary: "Single architecture selection",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/path: {arch: i386}
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{
					"myslice": {
						Package: "mypkg",
						Name:    "myslice",
						Contents: map[string]setup.PathInfo{
							"/path": {Kind: "copy", Arch: []string{"i386"}},
						},
					},
				},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Multiple architecture selection",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/path: {arch: [i386, amd64]}
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{
					"myslice": {
						Package: "mypkg",
						Name:    "myslice",
						Contents: map[string]setup.PathInfo{
							"/path": {Kind: "copy", Arch: []string{"i386", "amd64"}},
						},
					},
				},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Text can be empty",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/nonempty: {text: "foo"}
						/empty: {text: ""}
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{
					"myslice": {
						Package: "mypkg",
						Name:    "myslice",
						Contents: map[string]setup.PathInfo{
							"/nonempty": {Kind: "text", Info: "foo"},
							"/empty":    {Kind: "text", Info: ""},
						},
					},
				},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Multiple archives with priorities",
	input: map[string]string{
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
					priority: 20
					public-keys: [test-key]
				bar:
					version: 22.04
					components: [universe]
					suites: [jammy-updates]
					priority: -10
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"foo": {
				Name:       "foo",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				Priority:   20,
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
			"bar": {
				Name:       "bar",
				Version:    "22.04",
				Suites:     []string{"jammy-updates"},
				Components: []string{"universe"},
				Priority:   -10,
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Multiple archives inconsistent use of priorities",
	input: map[string]string{
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
					priority: 20
					public-keys: [test-key]
				bar:
					version: 22.04
					components: [universe]
					suites: [jammy-updates]
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	relerror: `chisel.yaml: archive "bar" is missing the priority setting`,
}, {
	summary: "Multiple archives with no priorities",
	input: map[string]string{
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
					public-keys: [test-key]
				bar:
					version: 22.04
					components: [universe]
					suites: [jammy-updates]
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	relerror: `chisel.yaml: archive "bar" is missing the priority setting`,
}, {
	summary: "Archive with suites unset",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			maintenance:
				standard: 2025-01-01
				end-of-life: 2100-01-01
			archives:
				ubuntu:
					version: 22.04
					components: [main, other]
		`,
	},
	relerror: `chisel.yaml: archive "ubuntu" missing suites field`,
}, {
	summary: "Two archives cannot have same priority",
	input: map[string]string{
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
					priority: 20
					public-keys: [test-key]
				bar:
					version: 22.04
					components: [universe]
					suites: [jammy-updates]
					priority: 20
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	relerror: `chisel.yaml: archives "bar" and "foo" have the same priority value of 20`,
}, {
	summary: "Invalid archive priority",
	input: map[string]string{
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
					priority: 10000
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
	},
	relerror: `chisel.yaml: archive "foo" has invalid priority value of 10000`,
}, {
	summary: "Invalid archive priority of 0",
	input: map[string]string{
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
					priority: 0
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
	},
	relerror: `chisel.yaml: archive "foo" has invalid priority value of 0`,
}, {
	summary: "Extra fields in YAML are ignored (necessary for forward compatibility)",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			maintenance:
			    standard: 2025-01-01
			    end-of-life: 2100-01-01
				madeUpKey7: whatever
			archives:
				ubuntu:
					version: 22.04
					components: [main, other]
					suites: [jammy, jammy-security]
					public-keys: [test-key]
					madeUpKey1: whatever
			madeUpKey2: whatever
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
					madeUpKey6: whatever
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			madeUpKey3: whatever
			slices:
				myslice:
					madeUpKey4: whatever
					contents:
						/path: {madeUpKey5: whatever}
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy", "jammy-security"},
				Components: []string{"main", "other"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{
					"myslice": {
						Package: "mypkg",
						Name:    "myslice",
						Contents: map[string]setup.PathInfo{
							"/path": {Kind: "copy"},
						},
					},
				},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Archives with public keys",
	input: map[string]string{
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
					public-keys: [extra-key]
					priority: 20
				bar:
					version: 22.04
					components: [universe]
					suites: [jammy-updates]
					public-keys: [test-key, extra-key]
					priority: 10
			public-keys:
				extra-key:
					id: ` + extraTestKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(extraTestKey.PubKeyArmor, "\t\t\t\t\t\t") + `
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"foo": {
				Name:       "foo",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				Priority:   20,
				PubKeys:    []*packet.PublicKey{extraTestKey.PubKey},
				Maintained: true,
			},
			"bar": {
				Name:       "bar",
				Version:    "22.04",
				Suites:     []string{"jammy-updates"},
				Components: []string{"universe"},
				Priority:   10,
				PubKeys:    []*packet.PublicKey{testKey.PubKey, extraTestKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Archive without public keys",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
		`,
	},
	relerror: `chisel.yaml: archive "foo" missing public-keys field`,
}, {
	summary: "Unknown public key",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					public-keys: [extra-key]
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	relerror: `chisel.yaml: archive "foo" refers to undefined public key "extra-key"`,
}, {
	summary: "Invalid public key",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					public-keys: [extra-key]
			public-keys:
				extra-key:
					id: foo
					armor: |
						G. B. Shaw's Law:
							Those who can -- do.
							Those who can't -- teach.

						Martin's Extension:
							Those who cannot teach -- administrate.
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	relerror: `chisel.yaml: cannot decode public key "extra-key": cannot decode armored data`,
}, {
	summary: "Mismatched public key ID",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					public-keys: [extra-key]
			public-keys:
				extra-key:
					id: ` + extraTestKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	relerror: `chisel.yaml: public key "extra-key" armor has incorrect ID: expected "9568570379BF1F43", got "854BAF1AA9D76600"`,
}, {
	summary: "Short package name",
	input: map[string]string{
		"slices/mydir/jq.yaml": `
			package: jq
			slices:
				bins:
					contents:
						/usr/bin/jq:
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"jq": {
				Format: "v1",
				Name:   "jq",
				Path:   "slices/mydir/jq.yaml",
				Slices: map[string]*setup.Slice{
					"bins": {
						Package: "jq",
						Name:    "bins",
						Contents: map[string]setup.PathInfo{
							"/usr/bin/jq": {Kind: "copy"},
						},
					},
				},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Very short, invalid package name",
	input: map[string]string{
		"slices/mydir/a.yaml": `
			package: a
		`,
	},
	relerror: `invalid slice definition filename: "a.yaml"`,
}, {
	summary: "Invalid slice name - too short",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				cc:
					contents:
						/usr/bin/cc:
		`,
	},
	relerror: `invalid slice name "cc" in slices/mydir/mypkg.yaml \(start with a-z, len >= 3, only a-z / 0-9 / -\)`,
}, {
	summary: "Package essentials with same package slice",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			essential:
				- mypkg_slice2
			slices:
				slice1:
				slice2:
				slice3:
					essential:
						- mypkg_slice1
						- mypkg_slice4
				slice4:
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{
					"slice1": {
						Package: "mypkg",
						Name:    "slice1",
						Essential: map[setup.SliceKey]setup.EssentialInfo{
							{"mypkg", "slice2"}: {},
						},
					},
					"slice2": {
						Package: "mypkg",
						Name:    "slice2",
					},
					"slice3": {
						Package: "mypkg",
						Name:    "slice3",
						Essential: map[setup.SliceKey]setup.EssentialInfo{
							{"mypkg", "slice2"}: {},
							{"mypkg", "slice1"}: {},
							{"mypkg", "slice4"}: {},
						},
					},
					"slice4": {
						Package: "mypkg",
						Name:    "slice4",
						Essential: map[setup.SliceKey]setup.EssentialInfo{
							{"mypkg", "slice2"}: {},
						},
					},
				},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Package essentials with slices from other packages",
	input: map[string]string{
		"slices/mydir/myotherpkg.yaml": `
			package: myotherpkg
			slices:
				slice1:
				slice2:
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			essential:
				- myotherpkg_slice2
				- mypkg_slice2
			slices:
				slice1:
					essential:
						- myotherpkg_slice1
				slice2:
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{
					"slice1": {
						Package: "mypkg",
						Name:    "slice1",
						Essential: map[setup.SliceKey]setup.EssentialInfo{
							{"myotherpkg", "slice2"}: {},
							{"mypkg", "slice2"}:      {},
							{"myotherpkg", "slice1"}: {},
						},
					},
					"slice2": {
						Package: "mypkg",
						Name:    "slice2",
						Essential: map[setup.SliceKey]setup.EssentialInfo{
							{"myotherpkg", "slice2"}: {},
						},
					},
				},
			},
			"myotherpkg": {
				Format: "v1",
				Name:   "myotherpkg",
				Path:   "slices/mydir/myotherpkg.yaml",
				Slices: map[string]*setup.Slice{
					"slice1": {
						Package: "myotherpkg",
						Name:    "slice1",
					},
					"slice2": {
						Package: "myotherpkg",
						Name:    "slice2",
					},
				},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Package essentials loop",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			essential:
				- mypkg_slice1
				- mypkg_slice2
			slices:
				slice1:
				slice2:
		`,
	},
	relerror: "essential loop detected: mypkg_slice1, mypkg_slice2",
}, {
	summary: "Cannot add slice to itself as essential",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				slice1:
					essential:
						- mypkg_slice1
		`,
	},
	relerror: `cannot add slice to itself as essential "mypkg_slice1" in slices/mydir/mypkg.yaml`,
}, {
	summary: "Package essentials clashes with slice essentials",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			essential:
				- mypkg_slice2
			slices:
				slice1:
					essential:
						- mypkg_slice2
				slice2:
		`,
	},
	relerror: `slice mypkg_slice1 repeats mypkg_slice2 in essential fields`,
}, {
	summary: "Duplicated slice essentials",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				slice1:
					essential:
						- mypkg_slice2
						- mypkg_slice2
				slice2:
		`,
	},
	relerror: `slice mypkg_slice1 repeats mypkg_slice2 in essential fields`,
}, {
	summary: "Duplicated package essentials",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			essential:
				- mypkg_slice1
				- mypkg_slice1
			slices:
				slice1:
				slice2:
		`,
	},
	relerror: `package "mypkg" repeats mypkg_slice1 in essential fields`,
}, {
	summary: "Bad slice reference in slice essential",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				slice1:
					essential:
						- mypkg-slice
		`,
	},
	relerror: `package "mypkg" has invalid essential slice reference: "mypkg-slice"`,
}, {
	summary: "Bad slice reference in package essential",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			essential:
				- mypkg-slice
			slices:
				slice1:
		`,
	},
	relerror: `package "mypkg" has invalid essential slice reference: "mypkg-slice"`,
}, {
	summary: "Glob clashes within same package",
	input: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice1:
					contents:
						/dir/**:
				myslice2:
					contents:
						/dir/file: {text: "foo"}
		`,
	},
	relerror: `slices test-package_myslice1 and test-package_myslice2 conflict on /dir/\*\* and /dir/file`,
}, {
	summary: "Pinned archive is not defined",
	input: map[string]string{
		"slices/test-package.yaml": `
			package: test-package
			archive: non-existing
		`,
	},
	relerror: `slices/test-package.yaml: package refers to undefined archive "non-existing"`,
}, {
	summary: "Specify generate: manifest",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/dir/**: {generate: "manifest"}
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{
					"myslice": {
						Package: "mypkg",
						Name:    "myslice",
						Contents: map[string]setup.PathInfo{
							"/dir/**": {Kind: "generate", Generate: "manifest"},
						},
					},
				},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
	selslices: []setup.SliceKey{{"mypkg", "myslice"}},
	selection: &setup.Selection{
		Slices: []*setup.Slice{{
			Package: "mypkg",
			Name:    "myslice",
			Contents: map[string]setup.PathInfo{
				"/dir/**": {Kind: "generate", Generate: "manifest"},
			},
		}},
	},
}, {
	summary: "Can specify generate with bogus value but cannot select those slices",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/dir/**: {generate: "foo"}
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{
					"myslice": {
						Package: "mypkg",
						Name:    "myslice",
						Contents: map[string]setup.PathInfo{
							"/dir/**": {Kind: "generate", Generate: "foo"},
						},
					},
				},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
	selslices: []setup.SliceKey{{"mypkg", "myslice"}},
	selerror:  `slice mypkg_myslice has invalid 'generate' for path /dir/\*\*: "foo"`,
}, {
	summary: "Paths with generate: manifest must have trailing /**",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/path/: {generate: "manifest"}
		`,
	},
	relerror: `slice mypkg_myslice has invalid generate path: /path/ does not end with /\*\*`,
}, {
	summary: "Paths with generate: manifest must not have any other wildcard except the trailing **",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/pat*h/to/dir/**: {generate: "manifest"}
		`,
	},
	relerror: `slice mypkg_myslice has invalid generate path: /pat\*h/to/dir/\*\* contains wildcard characters in addition to trailing \*\*`,
}, {
	summary: "Same paths conflict if one is generate and the other is not",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/path/**: {generate: "manifest"}
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice:
					contents:
						/path/**:
		`,
	},
	relerror: `slices mypkg_myslice and mypkg2_myslice conflict on /path/\*\*`,
}, {
	summary: "Generate paths can be the same across packages",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/path/**: {generate: manifest}
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice:
					contents:
						/path/**: {generate: manifest}
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{
					"myslice": {
						Package: "mypkg",
						Name:    "myslice",
						Contents: map[string]setup.PathInfo{
							"/path/**": {Kind: "generate", Generate: "manifest"},
						},
					},
				},
			},
			"mypkg2": {
				Format: "v1",
				Name:   "mypkg2",
				Path:   "slices/mydir/mypkg2.yaml",
				Slices: map[string]*setup.Slice{
					"myslice": {
						Package: "mypkg2",
						Name:    "myslice",
						Contents: map[string]setup.PathInfo{
							"/path/**": {Kind: "generate", Generate: "manifest"},
						},
					},
				},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Generate paths cannot conflict with any other path",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/path/**: {generate: manifest}
						/path/file:
		`,
	},
	relerror: `slices mypkg_myslice and mypkg_myslice conflict on /path/\*\* and /path/file`,
}, {
	summary: "Generate paths cannot conflict with any other path across slices",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice1:
					contents:
						/path/file:
				myslice2:
					contents:
						/path/**: {generate: manifest}
		`,
	},
	relerror: `slices mypkg_myslice1 and mypkg_myslice2 conflict on /path/file and /path/\*\*`,
}, {
	summary: "Generate paths conflict with other generate paths",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice1:
					contents:
						/path/subdir/**: {generate: manifest}
				myslice2:
					contents:
						/path/**: {generate: manifest}
		`,
	},
	relerror: `slices mypkg_myslice1 and mypkg_myslice2 conflict on /path/subdir/\*\* and /path/\*\*`,
}, {
	summary: `No other options in "generate" paths`,
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/path/**: {generate: "manifest", until: mutate}
		`,
	},
	relerror: `slice mypkg_myslice path /path/\*\* has invalid generate options`,
}, {
	summary: "chisel-v1 is deprecated",
	input: map[string]string{
		"chisel.yaml": `
			format: chisel-v1
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					v1-public-keys: [test-key]
			v1-public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
	},
	relerror: `chisel.yaml: unknown format "chisel-v1"`,
}, {
	summary: "Default archive compatibility",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			maintenance:
				standard: 2025-01-01
				end-of-life: 2100-01-01
			archives:
				default:
					default: true
					version: 22.04
					components: [main]
					suites: [jammy]
					public-keys: [test-key]
				other-1:
					version: 22.04
					components: [main]
					suites: [jammy]
					public-keys: [test-key]
				other-2:
					version: 22.04
					components: [main]
					suites: [jammy]
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"default": {
				Name:       "default",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
				Priority:   1,
			},
			"other-1": {
				Name:       "other-1",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
				Priority:   -2,
			},
			"other-2": {
				Name:       "other-2",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
				Priority:   -3,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Pro values in archives",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			maintenance:
				standard:    2025-01-01
				expanded:    2100-01-01
				legacy:      2100-01-01
				end-of-life: 2100-01-01
			archives:
				ubuntu:
					version: 20.04
					components: [main]
					suites: [focal]
					priority: 10
					public-keys: [test-key]
				fips:
					version: 20.04
					components: [main]
					suites: [focal]
					pro: fips
					priority: 20
					public-keys: [test-key]
				fips-updates:
					version: 20.04
					components: [main]
					suites: [focal-updates]
					pro: fips-updates
					priority: 21
					public-keys: [test-key]
				esm-apps:
					version: 20.04
					components: [main]
					suites: [focal-apps-security]
					pro: esm-apps
					priority: 16
					public-keys: [test-key]
				esm-infra:
					version: 20.04
					components: [main]
					suites: [focal-infra-security]
					pro: esm-infra
					priority: 15
					public-keys: [test-key]
				ignored:
					version: 20.04
					components: [main]
					suites: [foo]
					pro: unknown-value
					priority: 10
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "20.04",
				Suites:     []string{"focal"},
				Components: []string{"main"},
				Priority:   10,
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
			"fips": {
				Name:       "fips",
				Version:    "20.04",
				Suites:     []string{"focal"},
				Components: []string{"main"},
				Pro:        "fips",
				Priority:   20,
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
			"fips-updates": {
				Name:       "fips-updates",
				Version:    "20.04",
				Suites:     []string{"focal-updates"},
				Components: []string{"main"},
				Pro:        "fips-updates",
				Priority:   21,
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
			"esm-apps": {
				Name:       "esm-apps",
				Version:    "20.04",
				Suites:     []string{"focal-apps-security"},
				Components: []string{"main"},
				Pro:        "esm-apps",
				Priority:   16,
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
			"esm-infra": {
				Name:       "esm-infra",
				Version:    "20.04",
				Suites:     []string{"focal-infra-security"},
				Components: []string{"main"},
				Pro:        "esm-infra",
				Priority:   15,
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			Expanded:  time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
			Legacy:    time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Default is ignored",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			maintenance:
				standard: 2025-01-01
				end-of-life: 2100-01-01
			archives:
				default:
					default: true
					priority: 10
					version: 22.04
					components: [main]
					suites: [jammy]
					public-keys: [test-key]
				other:
					priority: 20
					version: 22.04
					components: [main]
					suites: [jammy]
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"default": {
				Name:       "default",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
				Priority:   10,
			},
			"other": {
				Name:       "other",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
				Priority:   20,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Multiple default archives",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			archives:
				foo:
					default: true
					version: 22.04
					components: [main]
					suites: [jammy]
					public-keys: [test-key]
				bar:
					default: true
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	relerror: `chisel.yaml: more than one default archive: bar, foo`,
}, {
	summary: "Additional v2-archives are merged with regular archives",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			maintenance:
				standard: 2025-01-01
				end-of-life: 2100-01-01
			archives:
				ubuntu:
					version: 20.04
					components: [main]
					suites: [focal]
					priority: 10
					public-keys: [test-key]
			v2-archives:
				fips:
					version: 20.04
					components: [main]
					suites: [focal]
					pro: fips
					priority: 20
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "20.04",
				Suites:     []string{"focal"},
				Components: []string{"main"},
				Priority:   10,
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
			"fips": {
				Name:       "fips",
				Version:    "20.04",
				Suites:     []string{"focal"},
				Components: []string{"main"},
				Pro:        "fips",
				Priority:   20,
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Cannot define same archive name in archives and v2-archives",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			archives:
				ubuntu:
					version: 20.04
					components: [main]
					suites: [focal]
					priority: 10
					public-keys: [test-key]
			v2-archives:
				ubuntu:
					version: 20.04
					components: [main]
					suites: [focal]
					priority: 20
					pro: fips
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	relerror: `chisel.yaml: archive "ubuntu" defined twice`,
}, {
	summary: "Cannot use prefer with generate",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice1:
					contents:
						/**: {generate: manifest, prefer: mypkg2}
		`,
	},
	relerror: `slice mypkg1_myslice1 path /\*\* has invalid generate options`,
}, {
	summary: "Cannot use prefer with wildcard",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice1:
					contents:
						/**: {prefer: mypkg2}
		`,
	},
	relerror: `slice mypkg1_myslice1 path /\*\* has invalid wildcard options`,
}, {
	summary: "Cannot use prefer its own package",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice1:
					contents:
						/file: {prefer: mypkg1}
		`,
	},
	relerror: "slice mypkg1_myslice1 cannot 'prefer' its own package for path /file",
}, {
	summary: "Path conflicts with 'prefer'",
	selslices: []setup.SliceKey{
		{"mypkg1", "myslice1"},
		{"mypkg1", "myslice2"},
		{"mypkg2", "myslice1"},
		{"mypkg3", "myslice1"},
	},
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice1:
					contents:
						/path: {prefer: mypkg2}
						/link: {symlink: /file1}
				myslice2:
					contents:
						/path: {prefer: mypkg2}
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice1:
					contents:
						/path: {prefer: mypkg3}
						/link: {symlink: /file2, prefer: mypkg1}
		`,
		"slices/mydir/mypkg3.yaml": `
			package: mypkg3
			slices:
				myslice1:
					contents:
						/path:
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg1": {
				Format: "v1",
				Name:   "mypkg1",
				Path:   "slices/mydir/mypkg1.yaml",
				Slices: map[string]*setup.Slice{
					"myslice1": {
						Package: "mypkg1",
						Name:    "myslice1",
						Contents: map[string]setup.PathInfo{
							"/path": {Kind: "copy", Prefer: "mypkg2"},
							"/link": {Kind: "symlink", Info: "/file1"},
						},
					},
					"myslice2": {
						Package: "mypkg1",
						Name:    "myslice2",
						Contents: map[string]setup.PathInfo{
							"/path": {Kind: "copy", Prefer: "mypkg2"},
						},
					},
				},
			},
			"mypkg2": {
				Format: "v1",
				Name:   "mypkg2",
				Path:   "slices/mydir/mypkg2.yaml",
				Slices: map[string]*setup.Slice{
					"myslice1": {
						Package: "mypkg2",
						Name:    "myslice1",
						Contents: map[string]setup.PathInfo{
							"/path": {Kind: "copy", Prefer: "mypkg3"},
							"/link": {Kind: "symlink", Info: "/file2", Prefer: "mypkg1"},
						},
					},
				},
			},
			"mypkg3": {
				Format: "v1",
				Name:   "mypkg3",
				Path:   "slices/mydir/mypkg3.yaml",
				Slices: map[string]*setup.Slice{
					"myslice1": {
						Package: "mypkg3",
						Name:    "myslice1",
						Contents: map[string]setup.PathInfo{
							"/path": {Kind: "copy"},
						},
					},
				},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
	prefers: map[string]string{
		"/path": "mypkg3",
		"/link": "mypkg1",
	},
}, {
	summary: "Path conflicts with 'prefer' depends on selection",
	selslices: []setup.SliceKey{
		{"mypkg1", "myslice1"},
		{"mypkg1", "myslice2"},
		{"mypkg2", "myslice1"},
	},
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice1:
					contents:
						/path: {prefer: mypkg2}
						/link: {symlink: /file1}
				myslice2:
					contents:
						/path: {prefer: mypkg2}
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice1:
					contents:
						/path: {prefer: mypkg3}
						/link: {symlink: /file2, prefer: mypkg1}
		`,
		"slices/mydir/mypkg3.yaml": `
			package: mypkg3
			slices:
				myslice1:
					contents:
						/path:
		`,
	},
	prefers: map[string]string{
		"/path": "mypkg2",
		"/link": "mypkg1",
	},
}, {
	summary: "Cannot specify same package in 'prefer'",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/path: {prefer: mypkg}
		`,
	},
	relerror: `slice mypkg_myslice cannot 'prefer' its own package for path /path`,
}, {
	summary: "Cannot specify non-existent package in 'prefer'",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice:
					contents:
						/path: {prefer: non-existent}
		`,
	},
	relerror: `slice mypkg_myslice path /path 'prefer' refers to undefined package "non-existent"`,
}, {
	summary: "Path prefers package, but package does not have path",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice:
					contents:
						/path: {prefer: mypkg2}
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
		`,
	},
	relerror: `package mypkg1 prefers package "mypkg2" which does not contain path /path`,
}, {
	summary: "Path has 'prefer' cycle",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice:
					contents:
						/path: {prefer: mypkg2}
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice:
					contents:
						/path: {prefer: mypkg3}
		`,
		"slices/mydir/mypkg3.yaml": `
			package: mypkg3
			slices:
				myslice:
					contents:
						/path: {prefer: mypkg1}
		`,
	},
	relerror: `package "mypkg[1-3]" is part of a prefer loop on /path`,
}, {
	summary: "Path has 'prefer' cycle and not all nodes are part of the cycle",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice:
					contents:
						/path: {prefer: mypkg2}
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice:
					contents:
						/path: {prefer: mypkg3}
		`,
		"slices/mydir/mypkg3.yaml": `
			package: mypkg3
			slices:
				myslice:
					contents:
						/path: {prefer: mypkg2}
		`,
	},
	relerror: `packages "mypkg1" and "mypkg3" cannot both prefer "mypkg2" for /path`,
}, {
	summary: "Cannot have two nodes without 'prefer' even if they provide the same content",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice:
					contents:
						/text: {text: foo}
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice:
					contents:
						/text: {text: foo}
		`,
		"slices/mydir/mypkg3.yaml": `
			package: mypkg3
			slices:
				myslice:
					contents:
						/text: {prefer: mypkg1}
		`,
	},
	relerror: `package "(mypkg1|mypkg2)" and "mypkg3" conflict on /text without prefer relationship`,
}, {
	summary: "Path has a disconnected 'prefer' graph",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice:
					contents:
						/path: {prefer: mypkg3}
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice:
					contents:
						/path: {prefer: mypkg4}
		`,
		"slices/mydir/mypkg3.yaml": `
			package: mypkg3
			slices:
				myslice:
					contents:
						/path:
		`,
		"slices/mydir/mypkg4.yaml": `
			package: mypkg4
			slices:
				myslice:
					contents:
						/path:
		`,
	},
	relerror: `package "[a-z1-9]*" and "[a-z1-9]*" conflict on /path without prefer relationship`,
}, {
	summary: "Path has more than one 'prefer' chain",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice:
					contents:
						/path: {prefer: mypkg3}
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice:
					contents:
						/path: {prefer: mypkg3}
		`,
		"slices/mydir/mypkg3.yaml": `
			package: mypkg3
			slices:
				myslice:
					contents:
						/path:
		`,
	},
	relerror: `packages "mypkg1" and "mypkg2" cannot both prefer "mypkg3" for /path`,
}, {
	summary: "Glob paths can conflict with 'prefer' chain",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice1:
					contents:
						/**:
				myslice2:
					contents:
						/path: {prefer: mypkg2}
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice:
					contents:
						/path:
		`,
	},
	// This test and the following one together ensure that both mypkg2_myslice
	// and mypkg1_myslice2 are checked against the glob.
	relerror: `slices mypkg1_myslice1 and mypkg2_myslice conflict on /\*\* and /path`,
}, {
	summary: "Glob paths can conflict with 'prefer' chain (reverse dependency)",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice1:
					contents:
						/**:
				myslice2:
					contents:
						/path:
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice:
					contents:
						/path: {prefer: mypkg1}
		`,
	},
	// This test and the previous one together ensure that both mypkg2_myslice
	// and mypkg1_myslice2 are checked against the glob.
	relerror: `slices mypkg1_myslice1 and mypkg2_myslice conflict on /\*\* and /path`,
}, {
	summary: "Slices of same package cannot have different 'prefer'",
	input: map[string]string{
		"slices/mydir/mypkg1.yaml": `
			package: mypkg1
			slices:
				myslice1:
					contents:
						/path: {prefer: mypkg2}
				myslice2:
					contents:
						/path: {prefer: mypkg3}
		`,
		"slices/mydir/mypkg2.yaml": `
			package: mypkg2
			slices:
				myslice:
					contents:
						/path:
		`,
		"slices/mydir/mypkg3.yaml": `
			package: mypkg3
			slices:
				myslice:
					contents:
						/path:
		`,
	},
	relerror: `package "mypkg1" has conflicting prefers for /path: mypkg2 != mypkg3`,
}, {
	summary: "Format v2 does not support default",
	input: map[string]string{
		"chisel.yaml": `
			format: v2
			archives:
				ubuntu:
					default: true
					version: 20.04
					components: [main]
					suites: [focal]
					priority: 10
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	relerror: `chisel.yaml: archive "ubuntu" has 'default' field which is deprecated since format v2`,
}, {
	summary: "Format v2 does not support v2-archives",
	input: map[string]string{
		"chisel.yaml": `
			format: v2
			v2-archives:
				ubuntu:
					default: true
					version: 20.04
					components: [main]
					suites: [focal]
					priority: 10
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	relerror: `chisel.yaml: v2-archives is deprecated since format v2`,
}, {
	summary: "Maintenance all dates",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			maintenance:
				standard: 2001-02-03
				expanded: 2004-05-06
				legacy: 2007-08-09
				end-of-life: 2010-11-12
			archives:
				ubuntu:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: false,
				OldRelease: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2001, time.February, 3, 0, 0, 0, 0, time.UTC),
			Expanded:  time.Date(2004, time.May, 6, 0, 0, 0, 0, time.UTC),
			Legacy:    time.Date(2007, time.August, 9, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2010, time.November, 12, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Maintenance: legacy and expanded are optional",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			maintenance:
				standard: 2001-02-03
				end-of-life: 2010-11-12
			archives:
				ubuntu:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: false,
				OldRelease: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2001, time.February, 3, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2010, time.November, 12, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Maintenance: end-of-life is required",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			maintenance:
				standard: 2001-02-03
			archives:
				ubuntu:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	relerror: `chisel.yaml: cannot parse maintenance: "end-of-life" is unset`,
}, {
	summary: "Maintenance: standard is required",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			maintenance:
				end-of-life: 2010-11-12
			archives:
				ubuntu:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	relerror: `chisel.yaml: cannot parse maintenance: "standard" is unset`,
}, {
	summary: "Maintenance: invalid date format",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			maintenance:
				standard: 23 Oct 2010
			archives:
				ubuntu:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	relerror: `chisel.yaml: cannot parse maintenance: expected format for "standard" is YYYY-MM-DD`,
}, {
	summary: "Maintenance: all in standard phase",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			maintenance:
				standard:    2025-01-01
				expanded:    2100-01-01
				legacy:      2100-01-01
				end-of-life: 2100-01-01
			archives:
				ubuntu:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					public-keys: [test-key]
					priority: 3
				esm-apps:
					version: 22.04
					components: [main]
					suites: [jammy-apps-security]
					pro: esm-apps
					priority: 2
					public-keys: [test-key]
				esm-infra:
					version: 22.04
					components: [main]
					suites: [jammy-infra-security]
					pro: esm-infra
					priority: 1
					public-keys: [test-key]
				fips:
					version: 22.04
					components: [main]
					suites: [jammy]
					pro: fips
					priority: 20
					public-keys: [test-key]
				fips-updates:
					version: 22.04
					components: [main]
					suites: [jammy-updates]
					pro: fips-updates
					priority: 21
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   3,
				Maintained: true,
				OldRelease: false,
			},
			"esm-apps": {
				Name:       "esm-apps",
				Pro:        "esm-apps",
				Version:    "22.04",
				Suites:     []string{"jammy-apps-security"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   2,
				Maintained: true,
			},
			"esm-infra": {
				Name:       "esm-infra",
				Pro:        "esm-infra",
				Version:    "22.04",
				Suites:     []string{"jammy-infra-security"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   1,
				Maintained: true,
			},
			"fips": {
				Name:       "fips",
				Pro:        "fips",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   20,
				Maintained: true,
			},
			"fips-updates": {
				Name:       "fips-updates",
				Pro:        "fips-updates",
				Version:    "22.04",
				Suites:     []string{"jammy-updates"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   21,
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			Expanded:  time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
			Legacy:    time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Maintenance: all archives in expanded phase",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			maintenance:
				standard:    2001-01-01
				expanded:    2025-01-01
				legacy:      2100-01-01
				end-of-life: 2100-01-01
			archives:
				ubuntu:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					public-keys: [test-key]
					priority: 3
				esm-apps:
					version: 22.04
					components: [main]
					suites: [jammy-apps-security]
					pro: esm-apps
					priority: 2
					public-keys: [test-key]
				esm-infra:
					version: 22.04
					components: [main]
					suites: [jammy-infra-security]
					pro: esm-infra
					priority: 1
					public-keys: [test-key]
				fips:
					version: 22.04
					components: [main]
					suites: [jammy]
					pro: fips
					priority: 20
					public-keys: [test-key]
				fips-updates:
					version: 22.04
					components: [main]
					suites: [jammy-updates]
					pro: fips-updates
					priority: 21
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   3,
				Maintained: false,
				OldRelease: false,
			},
			"esm-apps": {
				Name:       "esm-apps",
				Pro:        "esm-apps",
				Version:    "22.04",
				Suites:     []string{"jammy-apps-security"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   2,
				Maintained: true,
			},
			"esm-infra": {
				Name:       "esm-infra",
				Pro:        "esm-infra",
				Version:    "22.04",
				Suites:     []string{"jammy-infra-security"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   1,
				Maintained: true,
			},
			"fips": {
				Name:       "fips",
				Pro:        "fips",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   20,
				Maintained: true,
			},
			"fips-updates": {
				Name:       "fips-updates",
				Pro:        "fips-updates",
				Version:    "22.04",
				Suites:     []string{"jammy-updates"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   21,
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC),
			Expanded:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			Legacy:    time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Maintenance: all archives in legacy phase",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			maintenance:
				standard:    2001-01-01
				expanded:    2001-01-01
				legacy:      2025-01-01
				end-of-life: 2100-01-01
			archives:
				ubuntu:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					public-keys: [test-key]
					priority: 3
				esm-apps:
					version: 22.04
					components: [main]
					suites: [jammy-apps-security]
					pro: esm-apps
					priority: 2
					public-keys: [test-key]
				esm-infra:
					version: 22.04
					components: [main]
					suites: [jammy-infra-security]
					pro: esm-infra
					priority: 1
					public-keys: [test-key]
				fips:
					version: 22.04
					components: [main]
					suites: [jammy]
					pro: fips
					priority: 20
					public-keys: [test-key]
				fips-updates:
					version: 22.04
					components: [main]
					suites: [jammy-updates]
					pro: fips-updates
					priority: 21
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   3,
				Maintained: false,
				OldRelease: false,
			},
			"esm-apps": {
				Name:       "esm-apps",
				Pro:        "esm-apps",
				Version:    "22.04",
				Suites:     []string{"jammy-apps-security"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   2,
				Maintained: false,
			},
			"esm-infra": {
				Name:       "esm-infra",
				Pro:        "esm-infra",
				Version:    "22.04",
				Suites:     []string{"jammy-infra-security"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   1,
				Maintained: false,
			},
			"fips": {
				Name:       "fips",
				Pro:        "fips",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   20,
				Maintained: true,
			},
			"fips-updates": {
				Name:       "fips-updates",
				Pro:        "fips-updates",
				Version:    "22.04",
				Suites:     []string{"jammy-updates"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   21,
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC),
			Expanded:  time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC),
			Legacy:    time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Maintenance: all archives in end-of-life phase",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			maintenance:
				standard:    2001-01-01
				expanded:    2001-01-01
				legacy:      2001-01-01
				end-of-life: 2025-01-01
			archives:
				ubuntu:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					public-keys: [test-key]
					priority: 3
				esm-apps:
					version: 22.04
					components: [main]
					suites: [jammy-apps-security]
					pro: esm-apps
					priority: 2
					public-keys: [test-key]
				esm-infra:
					version: 22.04
					components: [main]
					suites: [jammy-infra-security]
					pro: esm-infra
					priority: 1
					public-keys: [test-key]
				fips:
					version: 22.04
					components: [main]
					suites: [jammy]
					pro: fips
					priority: 20
					public-keys: [test-key]
				fips-updates:
					version: 22.04
					components: [main]
					suites: [jammy-updates]
					pro: fips-updates
					priority: 21
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   3,
				Maintained: false,
				OldRelease: true,
			},
			"esm-apps": {
				Name:       "esm-apps",
				Pro:        "esm-apps",
				Version:    "22.04",
				Suites:     []string{"jammy-apps-security"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   2,
				Maintained: false,
			},
			"esm-infra": {
				Name:       "esm-infra",
				Pro:        "esm-infra",
				Version:    "22.04",
				Suites:     []string{"jammy-infra-security"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   1,
				Maintained: false,
			},
			"fips": {
				Name:       "fips",
				Pro:        "fips",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   20,
				Maintained: false,
			},
			"fips-updates": {
				Name:       "fips-updates",
				Pro:        "fips-updates",
				Version:    "22.04",
				Suites:     []string{"jammy-updates"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   21,
				Maintained: false,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC),
			Expanded:  time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC),
			Legacy:    time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Maintenance: pro archives default to end-of-life when expanded or legacy missing",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
			maintenance:
				standard:    2001-01-01
				end-of-life: 2100-01-01
			archives:
				ubuntu:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					public-keys: [test-key]
					priority: 3
				esm-apps:
					version: 22.04
					components: [main]
					suites: [jammy-apps-security]
					pro: esm-apps
					priority: 2
					public-keys: [test-key]
				esm-infra:
					version: 22.04
					components: [main]
					suites: [jammy-infra-security]
					pro: esm-infra
					priority: 1
					public-keys: [test-key]
				fips:
					version: 22.04
					components: [main]
					suites: [jammy]
					pro: fips
					priority: 20
					public-keys: [test-key]
				fips-updates:
					version: 22.04
					components: [main]
					suites: [jammy-updates]
					pro: fips-updates
					priority: 21
					public-keys: [test-key]
			public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   3,
				Maintained: true,
				OldRelease: false,
			},
			"esm-apps": {
				Name:       "esm-apps",
				Pro:        "esm-apps",
				Version:    "22.04",
				Suites:     []string{"jammy-apps-security"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   2,
				Maintained: true,
			},
			"esm-infra": {
				Name:       "esm-infra",
				Pro:        "esm-infra",
				Version:    "22.04",
				Suites:     []string{"jammy-infra-security"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   1,
				Maintained: true,
			},
			"fips": {
				Name:       "fips",
				Pro:        "fips",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   20,
				Maintained: true,
			},
			"fips-updates": {
				Name:       "fips-updates",
				Pro:        "fips-updates",
				Version:    "22.04",
				Suites:     []string{"jammy-updates"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   21,
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "Essentials with arch",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			v3-essential:
				mypkg_myslice4: {arch: [amd64, i386]}
			slices:
				myslice1:
					v3-essential:
						mypkg_myslice2: {arch: amd64}
						mypkg_myslice3: {arch: [amd64, arm64]}
						mypkg_myslice5:
				myslice2:
				myslice3:
				myslice4:
				myslice5:
		`,
	},
	release: &setup.Release{
		Format: "v1",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Maintained: true,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Format: "v1",
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{
					"myslice1": {
						Package: "mypkg",
						Name:    "myslice1",
						Essential: map[setup.SliceKey]setup.EssentialInfo{
							{"mypkg", "myslice2"}: {Arch: []string{"amd64"}},
							{"mypkg", "myslice3"}: {Arch: []string{"amd64", "arm64"}},
							{"mypkg", "myslice4"}: {Arch: []string{"amd64", "i386"}},
							{"mypkg", "myslice5"}: {Arch: nil},
						},
					},
					"myslice2": {
						Package: "mypkg",
						Name:    "myslice2",
						Essential: map[setup.SliceKey]setup.EssentialInfo{
							{"mypkg", "myslice4"}: {Arch: []string{"amd64", "i386"}},
						},
					},
					"myslice3": {
						Package: "mypkg",
						Name:    "myslice3",
						Essential: map[setup.SliceKey]setup.EssentialInfo{
							{"mypkg", "myslice4"}: {Arch: []string{"amd64", "i386"}},
						},
					},
					"myslice4": {
						Package:   "mypkg",
						Name:      "myslice4",
						Essential: nil,
					},
					"myslice5": {
						Package: "mypkg",
						Name:    "myslice5",
						Essential: map[setup.SliceKey]setup.EssentialInfo{
							{"mypkg", "myslice4"}: {Arch: []string{"amd64", "i386"}},
						},
					},
				},
			},
		},
		Maintenance: &setup.Maintenance{
			Standard:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			EndOfLife: time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	},
}, {
	summary: "'essential' and 'v3-essential' cannot intersect",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice1:
					essential:
						- mypkg_myslice2
					v3-essential:
						mypkg_myslice2: {arch: [amd64, i386]}
				myslice2:
		`,
	},
	relerror: `slice mypkg_myslice1 repeats mypkg_myslice2 in essential fields`,
}, {
	summary: "'essential' and 'v3-essential' cannot intersect at pkg level",
	input: map[string]string{
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			essential:
				- mypkg_myslice2
			v3-essential:
				mypkg_myslice2: {arch: [amd64, i386]}
			slices:
				myslice1:
				myslice2:
		`,
	},
	relerror: `package "mypkg" repeats mypkg_myslice2 in essential fields`,
}, {
	summary: "Format v3 expects a map in 'essential' (pkg)",
	input: map[string]string{
		"chisel.yaml": strings.ReplaceAll(testutil.DefaultChiselYaml, "format: v1", "format: v3"),
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			essential:
				- mypkg_myslice2
			slices:
				myslice1:
				myslice2:
		`,
	},
	relerror: `cannot parse package "mypkg" slice definitions: (.|\n)*`,
}, {
	summary: "Format v3 expects a map in 'essential' (slice)",
	input: map[string]string{
		"chisel.yaml": strings.ReplaceAll(testutil.DefaultChiselYaml, "format: v1", "format: v3"),
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice1:
					essential:
						- mypkg_myslice2
				myslice2:
		`,
	},
	relerror: `cannot parse package "mypkg" slice definitions: (.|\n)*`,
}, {
	summary: "In format v3 'v3-essential' is not supported (pkg)",
	input: map[string]string{
		"chisel.yaml": strings.ReplaceAll(testutil.DefaultChiselYaml, "format: v1", "format: v3"),
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			v3-essential:
				mypkg_myslice2:
			slices:
				myslice1:
				myslice2:
		`,
	},
	relerror: `package "mypkg": v3-essential is deprecated since format v3`,
}, {
	summary: "In format v3 'v3-essential' is not supported (slice)",
	input: map[string]string{
		"chisel.yaml": strings.ReplaceAll(testutil.DefaultChiselYaml, "format: v1", "format: v3"),
		"slices/mydir/mypkg.yaml": `
			package: mypkg
			slices:
				myslice1:
					v3-essential:
						mypkg_myslice2:
				myslice2:
		`,
	},
	relerror: `slice mypkg_myslice1: v3-essential is deprecated since format v3`,
}}

func (s *S) TestParseRelease(c *C) {
	// Run tests for "archives" field in "v1" format.
	runParseReleaseTests(c, setupTests)

	// Run tests for "v2-archives" field in "v1" format.
	v2ArchiveTests := make([]setupTest, 0, len(setupTests))
	for _, t := range setupTests {
		m := make(map[string]string)
		for k, v := range t.input {
			if !strings.Contains(v, "v2-archives:") && strings.Contains(v, "format: v1") {
				v = strings.ReplaceAll(v, "archives:", "v2-archives:")
			}
			m[k] = v
		}
		t.input = m
		v2ArchiveTests = append(v2ArchiveTests, t)
	}
	runParseReleaseTests(c, v2ArchiveTests)

	// Run tests for "v2" format.
	v2FormatTests := make([]setupTest, 0, len(setupTests))
	for _, t := range setupTests {
		t.summary += " (v2)"
		m := make(map[string]string)
		skip := false
		for k, v := range t.input {
			if strings.Contains(v, "format: v2") ||
				strings.Contains(v, "format: v3") ||
				strings.Contains(v, "v2-archives:") ||
				strings.Contains(v, "default: true") {
				skip = true
				break
			}
			v = strings.ReplaceAll(v, "format: v1", "format: v2")
			m[k] = v
		}
		if skip {
			// Test was not affected, no need to re-run.
			continue
		}
		t.input = m
		if t.release != nil {
			t.release.Format = "v2"
			for _, pkg := range t.release.Packages {
				pkg.Format = "v2"
			}
		}
		v2FormatTests = append(v2FormatTests, t)
	}
	runParseReleaseTests(c, v2FormatTests)

	// Run tests for "v3" format.
	v3FormatTests := make([]setupTest, 0, len(setupTests))
	for _, t := range setupTests {
		t.summary += " (v3)"
		m := make(map[string]string)
		skip := false
		for k, v := range t.input {
			if strings.Contains(v, "format: v2") ||
				strings.Contains(v, "format: v3") ||
				strings.Contains(v, "v2-archives:") ||
				strings.Contains(v, "default: true") {
				skip = true
				break
			}
			v, skip = oldEssentialToV3(c, testutil.Reindent(v))
			if skip {
				break
			}
			v = strings.ReplaceAll(v, "format: v1", "format: v3")
			m[k] = v
		}
		if skip {
			// Test was not affected, or it is not meaningful, no need to re-run.
			continue
		}
		t.input = m
		if t.release != nil {
			t.release.Format = "v3"
			for _, pkg := range t.release.Packages {
				pkg.Format = "v3"
			}
		}
		v3FormatTests = append(v3FormatTests, t)
	}
	runParseReleaseTests(c, v3FormatTests)
}

func runParseReleaseTests(c *C, tests []setupTest) {
	for _, test := range tests {
		c.Logf("Summary: %s", test.summary)

		if _, ok := test.input["chisel.yaml"]; !ok {
			test.input["chisel.yaml"] = string(testutil.DefaultChiselYaml)
		}
		if test.prefers == nil {
			test.prefers = make(map[string]string)
		}

		dir := c.MkDir()
		for path, data := range test.input {
			fpath := filepath.Join(dir, path)
			err := os.MkdirAll(filepath.Dir(fpath), 0755)
			c.Assert(err, IsNil)
			err = os.WriteFile(fpath, testutil.Reindent(data), 0644)
			c.Assert(err, IsNil)
		}

		release, err := setup.ReadRelease(dir)
		if err != nil || test.relerror != "" {
			if test.relerror != "" {
				c.Assert(err, ErrorMatches, test.relerror)
				continue
			} else {
				c.Assert(err, IsNil)
			}
		}

		c.Assert(release.Path, Equals, dir)
		release.Path = ""

		if test.release != nil {
			c.Assert(release, DeepEquals, test.release)
		}

		if test.selslices != nil {
			selection, err := setup.Select(release, test.selslices, "")
			if test.selerror != "" {
				c.Assert(err, ErrorMatches, test.selerror)
				continue
			} else {
				c.Assert(err, IsNil)
			}
			c.Assert(selection.Release, Equals, release)
			rawPrefers, err := selection.Prefers()
			c.Assert(err, IsNil)
			prefers := make(map[string]string)
			for path, pkg := range rawPrefers {
				prefers[path] = pkg.Name
			}
			c.Assert(prefers, DeepEquals, test.prefers)
			selection.Release = nil
			if test.selection != nil {
				c.Assert(selection, DeepEquals, test.selection)
			}
		}
	}
}

func (s *S) TestPackageMarshalYAML(c *C) {
	for _, test := range setupTests {
		c.Logf("Summary: %s", test.summary)

		if test.relerror == "" || test.release == nil {
			continue
		}

		data, ok := test.input["chisel.yaml"]
		if !ok {
			data = testutil.DefaultChiselYaml
		}

		dir := c.MkDir()
		// Write chisel.yaml.
		fpath := filepath.Join(dir, "chisel.yaml")
		err := os.WriteFile(fpath, testutil.Reindent(data), 0644)
		c.Assert(err, IsNil)
		// Write the packages YAML.
		for _, pkg := range test.release.Packages {
			fpath = filepath.Join(dir, pkg.Path)
			err = os.MkdirAll(filepath.Dir(fpath), 0755)
			c.Assert(err, IsNil)
			pkgData, err := yaml.Marshal(pkg)
			c.Assert(err, IsNil)
			err = os.WriteFile(fpath, testutil.Reindent(string(pkgData)), 0644)
			c.Assert(err, IsNil)
		}

		release, err := setup.ReadRelease(dir)
		c.Assert(err, IsNil)

		release.Path = ""
		c.Assert(release, DeepEquals, test.release)
	}
}

func (s *S) TestPackageYAMLFormat(c *C) {
	var tests = []struct {
		summary  string
		input    map[string]string
		expected map[string]string
	}{{
		summary: "Basic slice",
		input: map[string]string{
			"slices/mypkg.yaml": `
				package: mypkg
				archive: ubuntu
				slices:
					myslice:
						contents:
							/dir/file: {}
			`,
		},
	}, {
		summary: "All types of paths",
		input: map[string]string{
			"slices/mypkg.yaml": `
				package: mypkg
				archive: ubuntu
				slices:
					myslice:
						contents:
							/dir/arch-specific*: {arch: [amd64, arm64, i386]}
							/dir/copy: {copy: /dir/file}
							/dir/empty-file: {text: ""}
							/dir/glob*: {}
							/dir/manifest/**: {generate: manifest}
							/dir/mutable: {text: TODO, mutable: true, arch: riscv64}
							/dir/other-file: {}
							/dir/sub-dir/: {make: true, mode: 0644}
							/dir/symlink: {symlink: /dir/file}
							/dir/until: {until: mutate}
						mutate: |
							# Test multi-line string.
							content.write("/dir/mutable", foo)
			`,
		},
	}, {
		summary: "Global and per-slice essentials",
		input: map[string]string{
			"slices/mypkg.yaml": `
				package: mypkg
				archive: ubuntu
				essential:
					- mypkg_myslice3
				slices:
					myslice1:
						v3-essential:
							mypkg_myslice2: {arch: i386}
						contents:
							/dir/file1: {}
					myslice2:
						contents:
							/dir/file2: {}
					myslice3:
						contents:
							/dir/file3: {}
			`,
		},
		expected: map[string]string{
			"slices/mypkg.yaml": `
				package: mypkg
				archive: ubuntu
				slices:
					myslice1:
						contents:
							/dir/file1: {}
						v3-essential:
							mypkg_myslice2: {arch: i386}
							mypkg_myslice3: {}
					myslice2:
						contents:
							/dir/file2: {}
						v3-essential:
							mypkg_myslice3: {}
					myslice3:
						contents:
							/dir/file3: {}
			`,
		},
	}, {
		summary: "Path with prefer",
		input: map[string]string{
			"slices/mypkg1.yaml": `
				package: mypkg1
				archive: ubuntu
				slices:
					myslice:
						contents:
							/dir/prefer: {prefer: mypkg2}
			`,
			"slices/mypkg2.yaml": `
				package: mypkg2
				archive: ubuntu
				slices:
					myslice:
						contents:
							/dir/prefer: {}
			`,
		},
	}, {
		summary: "Format v3",
		input: map[string]string{
			"chisel.yaml": strings.ReplaceAll(testutil.DefaultChiselYaml, "format: v1", "format: v3"),
			"slices/mypkg.yaml": `
				package: mypkg
				archive: ubuntu
				essential:
					mypkg_three: {arch: i386}
				slices:
					one:
						essential:
							mypkg_two: {arch: [amd64, aarch64]}
					two:
					three:
			`,
		},
		expected: map[string]string{
			"chisel.yaml": strings.ReplaceAll(testutil.DefaultChiselYaml, "format: v1", "format: v3"),
			"slices/mypkg.yaml": `
				package: mypkg
				archive: ubuntu
				slices:
					one:
						essential:
							mypkg_three: {arch: i386}
							mypkg_two: {arch: [amd64, aarch64]}
					three: {}
					two:
						essential:
							mypkg_three: {arch: i386}
			`,
		},
	}}

	for _, test := range tests {
		c.Logf("Summary: %s", test.summary)

		if _, ok := test.input["chisel.yaml"]; !ok {
			test.input["chisel.yaml"] = testutil.DefaultChiselYaml
		}

		dir := c.MkDir()
		for path, data := range test.input {
			fpath := filepath.Join(dir, path)
			err := os.MkdirAll(filepath.Dir(fpath), 0755)
			c.Assert(err, IsNil)
			err = os.WriteFile(fpath, testutil.Reindent(data), 0644)
			c.Assert(err, IsNil)
		}

		release, err := setup.ReadRelease(dir)
		c.Assert(err, IsNil)

		if test.expected == nil {
			test.expected = test.input
		}
		for _, pkg := range release.Packages {
			data, err := yaml.Marshal(pkg)
			c.Assert(err, IsNil)
			expected := string(testutil.Reindent(test.expected[pkg.Path]))
			c.Assert(strings.TrimSpace(string(data)), Equals, strings.TrimSpace(expected))
		}
	}
}

// This is an awkward test because right now the fact Generate is considered
// by SameContent is irrelevant to the implementation, because the code path
// happens to not touch it. More important than this test, there's an entry
// in setupTests that verifies that two packages with slices having
// {generate: manifest} in the same path are considered equal.
var yamlPathGenerateTests = []struct {
	summary      string
	path1, path2 *setup.YAMLPath
	result       bool
}{{
	summary: `Same "generate" value`,
	path1:   &setup.YAMLPath{Generate: setup.GenerateManifest},
	path2:   &setup.YAMLPath{Generate: setup.GenerateManifest},
	result:  true,
}, {
	summary: `Different "generate" value`,
	path1:   &setup.YAMLPath{Generate: setup.GenerateManifest},
	path2:   &setup.YAMLPath{Generate: setup.GenerateNone},
	result:  false,
}}

func (s *S) TestYAMLPathGenerate(c *C) {
	for _, test := range yamlPathGenerateTests {
		c.Logf("Summary: %s", test.summary)
		result := test.path1.SameContent(test.path2)
		c.Assert(result, Equals, test.result)
	}
}

// oldEssentialToV3 converts the essentials in v1 and v2, both 'essential', and
// 'v3-essential' to the shape expected by the v3 format.
// skip is set to true when an accurate translation of the test is not
// possible, for example having duplicates in the list.
func oldEssentialToV3(c *C, input []byte) (out string, skip bool) {
	var raw map[string]any
	err := yaml.Unmarshal(input, &raw)
	c.Assert(err, IsNil)

	if slices, ok := raw["slices"].(map[string]any); ok {
		for _, rawSlice := range slices {
			if slice, ok := rawSlice.(map[string]any); ok {
				newEssential := make(map[string]any)
				if oldEssential, ok := slice["essential"].([]any); ok {
					for _, value := range oldEssential {
						s := value.(string)
						if _, ok := newEssential[s]; ok {
							// Duplicated entries are impossible in v3.
							return "", true
						}
						newEssential[s] = nil
					}
				}
				if oldEssential, ok := slice["v3-essential"].(map[string]any); ok {
					for key, value := range oldEssential {
						if _, ok := newEssential[key]; ok {
							return "", true
						}
						newEssential[key] = value
					}
					delete(slice, "v3-essential")
				}
				slice["essential"] = newEssential
			}
		}
	}

	newEssential := make(map[string]any)
	if oldEssential, ok := raw["essential"].([]any); ok {
		for _, item := range oldEssential {
			s := item.(string)
			if _, ok := newEssential[s]; ok {
				// Duplicated entries are impossible in v3.
				return "", true
			}
			newEssential[s] = nil
		}
	}
	if oldEssential, ok := raw["v3-essential"].(map[string]any); ok {
		for key, value := range oldEssential {
			if _, ok := newEssential[key]; ok {
				// Duplicated entries are impossible in v3.
				return "", true
			}
			newEssential[key] = value
		}
		delete(raw, "v3-essential")
	}
	raw["essential"] = newEssential

	bs, err := yaml.Marshal(raw)
	c.Assert(err, IsNil)
	// Maintenance dates get marshaled as <date>T00:00:00Z by default.
	return strings.ReplaceAll(string(bs), "T00:00:00Z", ""), false
}
