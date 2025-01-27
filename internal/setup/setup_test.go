package setup_test

import (
	"os"
	"path/filepath"
	"strings"

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
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
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
				Name: "mypkg",
				Path: "slices/mydir/mypkg.yaml",
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
				Name: "mypkg",
				Path: "slices/mydir/mypkg.yaml",
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
				Name: "mypkg",
				Path: "slices/mydir/mypkg.yaml",
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
				Name: "mypkg",
				Path: "slices/mydir/mypkg.yaml",
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
				Name: "mypkg",
				Path: "slices/mydir/mypkg.yaml",
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
				Name: "mypkg",
				Path: "slices/mydir/mypkg.yaml",
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
	summary: "Multiple archives with priorities",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
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
		Archives: map[string]*setup.Archive{
			"foo": {
				Name:       "foo",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				Priority:   20,
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
			"bar": {
				Name:       "bar",
				Version:    "22.04",
				Suites:     []string{"jammy-updates"},
				Components: []string{"universe"},
				Priority:   -10,
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
		},
	},
}, {
	summary: "Multiple archives inconsistent use of priorities",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
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
				Name: "mypkg",
				Path: "slices/mydir/mypkg.yaml",
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
			format: v1
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
		Archives: map[string]*setup.Archive{
			"foo": {
				Name:       "foo",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main", "universe"},
				Priority:   20,
				PubKeys:    []*packet.PublicKey{extraTestKey.PubKey},
			},
			"bar": {
				Name:       "bar",
				Version:    "22.04",
				Suites:     []string{"jammy-updates"},
				Components: []string{"universe"},
				Priority:   10,
				PubKeys:    []*packet.PublicKey{testKey.PubKey, extraTestKey.PubKey},
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
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
				Name: "jq",
				Path: "slices/mydir/jq.yaml",
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
				Name: "mypkg",
				Path: "slices/mydir/mypkg.yaml",
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
				Name: "mypkg",
				Path: "slices/mydir/mypkg.yaml",
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
				Name: "myotherpkg",
				Path: "slices/mydir/myotherpkg.yaml",
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
				Name: "mypkg",
				Path: "slices/mydir/mypkg.yaml",
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
				Name: "mypkg",
				Path: "slices/mydir/mypkg.yaml",
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
				Name: "mypkg",
				Path: "slices/mydir/mypkg.yaml",
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
				Name: "mypkg2",
				Path: "slices/mydir/mypkg2.yaml",
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
		Archives: map[string]*setup.Archive{
			"default": {
				Name:       "default",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   1,
			},
			"other-1": {
				Name:       "other-1",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   -2,
			},
			"other-2": {
				Name:       "other-2",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   -3,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
		},
	},
}, {
	summary: "Pro values in archives",
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
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "20.04",
				Suites:     []string{"focal"},
				Components: []string{"main"},
				Priority:   10,
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
			"fips": {
				Name:       "fips",
				Version:    "20.04",
				Suites:     []string{"focal"},
				Components: []string{"main"},
				Pro:        "fips",
				Priority:   20,
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
			"fips-updates": {
				Name:       "fips-updates",
				Version:    "20.04",
				Suites:     []string{"focal-updates"},
				Components: []string{"main"},
				Pro:        "fips-updates",
				Priority:   21,
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
			"esm-apps": {
				Name:       "esm-apps",
				Version:    "20.04",
				Suites:     []string{"focal-apps-security"},
				Components: []string{"main"},
				Pro:        "esm-apps",
				Priority:   16,
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
			"esm-infra": {
				Name:       "esm-infra",
				Version:    "20.04",
				Suites:     []string{"focal-infra-security"},
				Components: []string{"main"},
				Pro:        "esm-infra",
				Priority:   15,
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
		},
	},
}, {
	summary: "Default is ignored",
	input: map[string]string{
		"chisel.yaml": `
			format: v1
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
		Archives: map[string]*setup.Archive{
			"default": {
				Name:       "default",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   10,
			},
			"other": {
				Name:       "other",
				Version:    "22.04",
				Suites:     []string{"jammy"},
				Components: []string{"main"},
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
				Priority:   20,
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
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
		Archives: map[string]*setup.Archive{
			"ubuntu": {
				Name:       "ubuntu",
				Version:    "20.04",
				Suites:     []string{"focal"},
				Components: []string{"main"},
				Priority:   10,
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
			"fips": {
				Name:       "fips",
				Version:    "20.04",
				Suites:     []string{"focal"},
				Components: []string{"main"},
				Pro:        "fips",
				Priority:   20,
				PubKeys:    []*packet.PublicKey{testKey.PubKey},
			},
		},
		Packages: map[string]*setup.Package{
			"mypkg": {
				Name:   "mypkg",
				Path:   "slices/mydir/mypkg.yaml",
				Slices: map[string]*setup.Slice{},
			},
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
}}

var defaultChiselYaml = `
	format: v1
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
`

func (s *S) TestParseRelease(c *C) {
	// Run tests for "archives" field in "v1" format.
	runParseReleaseTests(c, setupTests)

	// Run tests for "v2-archives" field in "v1" format.
	v2ArchiveTests := make([]setupTest, 0, len(setupTests))
	for _, t := range setupTests {
		m := make(map[string]string)
		for k, v := range t.input {
			if !strings.Contains(v, "v2-archives:") {
				v = strings.Replace(v, "archives:", "v2-archives:", -1)
			}
			m[k] = v
		}
		t.input = m
		v2ArchiveTests = append(v2ArchiveTests, t)
	}
	runParseReleaseTests(c, v2ArchiveTests)
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

func (s *S) TestPackageMarshalYAML(c *C) {
	for _, test := range setupTests {
		c.Logf("Summary: %s", test.summary)

		if test.relerror == "" || test.release == nil {
			continue
		}

		data, ok := test.input["chisel.yaml"]
		if !ok {
			data = defaultChiselYaml
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
						essential:
							- mypkg_myslice2
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
						essential:
							- mypkg_myslice3
							- mypkg_myslice2
						contents:
							/dir/file1: {}
					myslice2:
						essential:
							- mypkg_myslice3
						contents:
							/dir/file2: {}
					myslice3:
						contents:
							/dir/file3: {}
			`,
		},
	}}

	for _, test := range tests {
		c.Logf("Summary: %s", test.summary)

		if _, ok := test.input["chisel.yaml"]; !ok {
			test.input["chisel.yaml"] = defaultChiselYaml
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
