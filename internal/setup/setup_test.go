package setup_test

import (
	"os"
	"path/filepath"

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
	relerror: `chisel.yaml: expected format "chisel-v1", got "foobar"`,
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
	for _, test := range setupTests {
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
