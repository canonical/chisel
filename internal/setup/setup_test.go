package setup_test

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/openpgp/packet"
	. "gopkg.in/check.v1"

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
			format: chisel-v1
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
			format: chisel-v1
			archives:
				ubuntu:
					version: 22.04
					components: [main, other]
					suites: [jammy, jammy-security]
					v1-public-keys: [test-key]
			v1-public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	release: &setup.Release{
		DefaultArchive: "ubuntu",

		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy", "jammy-security"},
				Components: []string{"main", "other"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Archive: "ubuntu",
				Name:    "mypkg",
				Path:    "slices/mydir/mypkg.yaml",
				Slices:  map[string]*setup.Slice{},
			},
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
		DefaultArchive: "ubuntu",

		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Archive: "ubuntu",
				Name:    "mypkg",
				Path:    "slices/mydir/mypkg.yaml",
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
						Essential: []setup.SliceKey{
							{"mypkg", "myslice1"},
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
		DefaultArchive: "ubuntu",

		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Archive: "ubuntu",
				Name:    "mypkg",
				Path:    "slices/mydir/mypkg.yaml",
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
			Essential: []setup.SliceKey{
				{"mypkg1", "myslice1"},
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
		DefaultArchive: "ubuntu",

		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Archive: "ubuntu",
				Name:    "mypkg",
				Path:    "slices/mydir/mypkg.yaml",
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
		DefaultArchive: "ubuntu",

		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Archive: "ubuntu",
				Name:    "mypkg",
				Path:    "slices/mydir/mypkg.yaml",
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
		DefaultArchive: "ubuntu",

		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Archive: "ubuntu",
				Name:    "mypkg",
				Path:    "slices/mydir/mypkg.yaml",
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
		DefaultArchive: "ubuntu",

		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Archive: "ubuntu",
				Name:    "mypkg",
				Path:    "slices/mydir/mypkg.yaml",
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
	},
}, {
	summary: "Multiple archives",
	input: map[string]string{
		"chisel.yaml": `
			format: chisel-v1
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					default: true
					v1-public-keys: [test-key]
				bar:
					version: 22.04
					components: [universe]
					suites: [jammy-updates]
					v1-public-keys: [test-key]
			v1-public-keys:
				test-key:
					id: ` + testKey.ID + `
					armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
		`,
		"slices/mydir/mypkg.yaml": `
			package: mypkg
		`,
	},
	release: &setup.Release{
		DefaultArchive: "foo",

		Archives: map[string]*setup.Archive{
			"foo": {
				Name:       "foo",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
			"bar": {
				Name:       "bar",
				Version:    "22.04",
				Suites:     []string{"jammy-updates"},
				Components: []string{"universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Archive: "foo",
				Name:    "mypkg",
				Path:    "slices/mydir/mypkg.yaml",
				Slices:  map[string]*setup.Slice{},
			},
		},
	},
}, {
	summary: "Extra fields in YAML are ignored (necessary for forward compatibility)",
	input: map[string]string{
		"chisel.yaml": `
			format: chisel-v1
			archives:
				ubuntu:
					version: 22.04
					components: [main, other]
					suites: [jammy, jammy-security]
					v1-public-keys: [test-key]
					madeUpKey1: whatever
			madeUpKey2: whatever
			v1-public-keys:
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
		DefaultArchive: "ubuntu",

		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy", "jammy-security"},
				Components: []string{"main", "other"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Archive: "ubuntu",
				Name:    "mypkg",
				Path:    "slices/mydir/mypkg.yaml",
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
	},
}, {
	summary: "Archives with public keys",
	input: map[string]string{
		"chisel.yaml": `
			format: chisel-v1
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					v1-public-keys: [extra-key]
					default: true
				bar:
					version: 22.04
					components: [universe]
					suites: [jammy-updates]
					v1-public-keys: [test-key, extra-key]
			v1-public-keys:
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
		DefaultArchive: "foo",

		Archives: map[string]*setup.Archive{
			"foo": {
				Name:       "foo",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{extraTestKey.PubKey},
			},
			"bar": {
				Name:       "bar",
				Version:    "22.04",
				Suites:     []string{"jammy-updates"},
				Components: []string{"universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey, extraTestKey.PubKey},
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Archive: "foo",
				Name:    "mypkg",
				Path:    "slices/mydir/mypkg.yaml",
				Slices:  map[string]*setup.Slice{},
			},
		},
	},
}, {
	summary: "Archive without public keys",
	input: map[string]string{
		"chisel.yaml": `
			format: chisel-v1
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					default: true
		`,
	},
	relerror: `chisel.yaml: archive "foo" missing v1-public-keys field`,
}, {
	summary: "Unknown public key",
	input: map[string]string{
		"chisel.yaml": `
			format: chisel-v1
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					v1-public-keys: [extra-key]
					default: true
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
			format: chisel-v1
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					v1-public-keys: [extra-key]
					default: true
			v1-public-keys:
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
			format: chisel-v1
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					suites: [jammy]
					v1-public-keys: [extra-key]
					default: true
			v1-public-keys:
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
		DefaultArchive: "ubuntu",

		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
		},
		Packages: map[string]*setup.Package{
			"jq": {
				Archive: "ubuntu",
				Name:    "jq",
				Path:    "slices/mydir/jq.yaml",
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
		DefaultArchive: "ubuntu",
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Archive: "ubuntu",
				Name:    "mypkg",
				Path:    "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{
					"slice1": {
						Package: "mypkg",
						Name:    "slice1",
						Essential: []setup.SliceKey{
							{"mypkg", "slice2"},
						},
					},
					"slice2": {
						Package: "mypkg",
						Name:    "slice2",
					},
					"slice3": {
						Package: "mypkg",
						Name:    "slice3",
						Essential: []setup.SliceKey{
							{"mypkg", "slice2"},
							{"mypkg", "slice1"},
							{"mypkg", "slice4"},
						},
					},
					"slice4": {
						Package: "mypkg",
						Name:    "slice4",
						Essential: []setup.SliceKey{
							{"mypkg", "slice2"},
						},
					},
				},
			},
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
		DefaultArchive: "ubuntu",

		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Archive: "ubuntu",
				Name:    "mypkg",
				Path:    "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{
					"slice1": {
						Package: "mypkg",
						Name:    "slice1",
						Essential: []setup.SliceKey{
							{"myotherpkg", "slice2"},
							{"mypkg", "slice2"},
							{"myotherpkg", "slice1"},
						},
					},
					"slice2": {
						Package: "mypkg",
						Name:    "slice2",
						Essential: []setup.SliceKey{
							{"myotherpkg", "slice2"},
						},
					},
				},
			},
			"myotherpkg": {
				Archive: "ubuntu",
				Name:    "myotherpkg",
				Path:    "slices/mydir/myotherpkg.yaml",
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
	relerror: `slice mypkg_slice1 defined with redundant essential slice: mypkg_slice2`,
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
	relerror: `slice mypkg_slice1 defined with redundant essential slice: mypkg_slice2`,
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
	relerror: `package mypkg defined with redundant essential slice: mypkg_slice1`,
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
	// TODO this should be an error because the content does not match.
}}

var defaultChiselYaml = `
	format: chisel-v1
	archives:
		ubuntu:
			version: 22.04
			components: [main, universe]
			v1-public-keys: [test-key]
	v1-public-keys:
		test-key:
			id: ` + testKey.ID + `
			armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
`

func (s *S) TestParseRelease(c *C) {
	// Run tests for format chisel-v1.
	runParseReleaseTests(c, setupTests)

	// Run tests for format v1.
	v1SetupTests := make([]setupTest, len(setupTests))
	for i, t := range setupTests {
		t.relerror = strings.Replace(t.relerror, "chisel-v1", "v1", -1)
		t.relerror = strings.Replace(t.relerror, "v1-public-keys", "public-keys", -1)
		m := map[string]string{}
		for k, v := range t.input {
			v = strings.Replace(v, "chisel-v1", "v1", -1)
			v = strings.Replace(v, "v1-public-keys", "public-keys", -1)
			m[k] = v
		}
		t.input = m
		v1SetupTests[i] = t
	}
	runParseReleaseTests(c, v1SetupTests)
}

func runParseReleaseTests(c *C, tests []setupTest) {
	for _, test := range tests {
		c.Logf("Summary: %s", test.summary)

		if _, ok := test.input["chisel.yaml"]; !ok {
			test.input["chisel.yaml"] = string(defaultChiselYaml)
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
			selection, err := setup.Select(release, test.selslices)
			if test.selerror != "" {
				c.Assert(err, ErrorMatches, test.selerror)
				continue
			} else {
				c.Assert(err, IsNil)
			}
			c.Assert(selection.Release, Equals, release)
			selection.Release = nil
			if test.selection != nil {
				c.Assert(selection, DeepEquals, test.selection)
			}
		}
	}
}

var sliceKeyTests = []struct {
	input    string
	expected setup.SliceKey
	err      string
}{{
	input:    "foo_bar",
	expected: setup.SliceKey{Package: "foo", Slice: "bar"},
}, {
	input:    "fo_bar",
	expected: setup.SliceKey{Package: "fo", Slice: "bar"},
}, {
	input:    "1234_bar",
	expected: setup.SliceKey{Package: "1234", Slice: "bar"},
}, {
	input:    "foo1.1-2-3_bar",
	expected: setup.SliceKey{Package: "foo1.1-2-3", Slice: "bar"},
}, {
	input:    "foo-pkg_dashed-slice-name",
	expected: setup.SliceKey{Package: "foo-pkg", Slice: "dashed-slice-name"},
}, {
	input:    "foo+_bar",
	expected: setup.SliceKey{Package: "foo+", Slice: "bar"},
}, {
	input:    "foo_slice123",
	expected: setup.SliceKey{Package: "foo", Slice: "slice123"},
}, {
	input:    "g++_bins",
	expected: setup.SliceKey{Package: "g++", Slice: "bins"},
}, {
	input:    "a+_bar",
	expected: setup.SliceKey{Package: "a+", Slice: "bar"},
}, {
	input:    "a._bar",
	expected: setup.SliceKey{Package: "a.", Slice: "bar"},
}, {
	input: "foo_ba",
	err:   `invalid slice reference: "foo_ba"`,
}, {
	input: "f_bar",
	err:   `invalid slice reference: "f_bar"`,
}, {
	input: "1234_789",
	err:   `invalid slice reference: "1234_789"`,
}, {
	input: "foo_bar.x.y",
	err:   `invalid slice reference: "foo_bar.x.y"`,
}, {
	input: "foo-_-bar",
	err:   `invalid slice reference: "foo-_-bar"`,
}, {
	input: "foo_bar-",
	err:   `invalid slice reference: "foo_bar-"`,
}, {
	input: "foo-_bar",
	err:   `invalid slice reference: "foo-_bar"`,
}, {
	input: "-foo_bar",
	err:   `invalid slice reference: "-foo_bar"`,
}, {
	input: "foo_bar_baz",
	err:   `invalid slice reference: "foo_bar_baz"`,
}, {
	input: "a-_bar",
	err:   `invalid slice reference: "a-_bar"`,
}, {
	input: "+++_bar",
	err:   `invalid slice reference: "\+\+\+_bar"`,
}, {
	input: "..._bar",
	err:   `invalid slice reference: "\.\.\._bar"`,
}, {
	input: "white space_no-whitespace",
	err:   `invalid slice reference: "white space_no-whitespace"`,
}}

func (s *S) TestParseSliceKey(c *C) {
	for _, test := range sliceKeyTests {
		key, err := setup.ParseSliceKey(test.input)
		if test.err != "" {
			c.Assert(err, ErrorMatches, test.err)
			continue
		}
		c.Assert(err, IsNil)
		c.Assert(key, DeepEquals, test.expected)
	}
}
