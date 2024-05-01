package cut_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/cut"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/testutil"
)

var (
	testKey = testutil.PGPKeys["key1"]
)

type cutTest struct {
	summary    string
	release    map[string]string
	slices     []setup.SliceKey
	pkgs       map[string][]byte
	filesystem map[string]string
	db         string
	dbPaths    []string
	err        string
}

var cutTests = []cutTest{{
	summary: "Basic cut",
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/file:
						/dir/file-copy:  {copy: /dir/file}
						/other-dir/file: {symlink: ../dir/file}
						/dir/text-file:  {text: data1}
						/dir/foo/bar/:   {make: true, mode: 01777}
				manifest:
					contents:
						/manifest/**: {generate: manifest}
		`,
	},
	slices: []setup.SliceKey{
		{"test-package", "myslice"},
		{"test-package", "manifest"},
	},
	filesystem: map[string]string{
		"/manifest/":          "dir 0755",
		"/manifest/chisel.db": "file 0644 99a1ff16",
		"/dir/":               "dir 0755",
		"/dir/file":           "file 0644 cc55e2ec",
		"/dir/file-copy":      "file 0644 cc55e2ec",
		"/dir/foo/":           "dir 0755",
		"/dir/foo/bar/":       "dir 01777",
		"/dir/text-file":      "file 0644 5b41362b",
		"/other-dir/":         "dir 0755",
		"/other-dir/file":     "symlink ../dir/file",
	},
	dbPaths: []string{"/manifest/chisel.db"},
	db: `
{"jsonwall":"1.0","schema":"1.0","count":16}
{"kind":"content","slice":"test-package_manifest","path":"/manifest/chisel.db"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/file"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/file-copy"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/foo/bar/"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/text-file"}
{"kind":"content","slice":"test-package_myslice","path":"/other-dir/file"}
{"kind":"package","name":"test-package","version":"test-package_version","sha256":"test-package_hash","arch":"test-package_arch"}
{"kind":"path","path":"/dir/file","mode":"0644","slices":["test-package_myslice"],"sha256":"cc55e2ecf36e40171ded57167c38e1025c99dc8f8bcdd6422368385a977ae1fe","size":14}
{"kind":"path","path":"/dir/file-copy","mode":"0644","slices":["test-package_myslice"],"sha256":"cc55e2ecf36e40171ded57167c38e1025c99dc8f8bcdd6422368385a977ae1fe","size":14}
{"kind":"path","path":"/dir/foo/bar/","mode":"01777","slices":["test-package_myslice"]}
{"kind":"path","path":"/dir/text-file","mode":"0644","slices":["test-package_myslice"],"sha256":"5b41362bc82b7f3d56edc5a306db22105707d01ff4819e26faef9724a2d406c9","size":5}
{"kind":"path","path":"/manifest/chisel.db","mode":"0644","slices":["test-package_manifest"]}
{"kind":"path","path":"/other-dir/file","mode":"0644","slices":["test-package_myslice"],"link":"../dir/file"}
{"kind":"slice","name":"test-package_manifest"}
{"kind":"slice","name":"test-package_myslice"}
`,
}, {
	summary: "All types of paths",
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/file:
						/d?r/s*al/*/:
						/parent/**:
						/dir/file-copy:   {copy: /dir/file}
						/dir/file-copy-2: {copy: /dir/file, mode: 0755}
						/dir/link/file:   {symlink: /dir/file}
						/dir/link/file-2: {symlink: ../file, mode: 0777}
						/dir/text/file:   {text: data1}
						/dir/text/file-2: {text: data2, mode: 0755}
						/dir/text/file-3: {text: data3, mutable: true}
						/dir/text/file-4: {text: data4, until: mutate}
						/dir/text/file-5: {text: "", mode: 0755}
						/dir/text/file-6: {symlink: ./file-3}
						/dir/text/file-7: {text: data7, arch: s390x}
						/dir/all-text:    {text: "", mutable: true}
						/dir/foo/bar/:    {make: true, mode: 01777}
					mutate: |
						content.write("/dir/text/file-3", "foo")
						dir = "/dir/text/"
						data = [ content.read(dir + f) for f in content.list(dir) ]
						content.write("/dir/all-text", "".join(data))
				manifest:
					contents:
						/manifest/**: {generate: manifest}
		`,
	},
	slices: []setup.SliceKey{
		{"test-package", "myslice"},
		{"test-package", "manifest"},
	},
	filesystem: map[string]string{
		"/manifest/":               "dir 0755",
		"/manifest/chisel.db":      "file 0644 a4c4c1d2",
		"/dir/":                    "dir 0755",
		"/dir/all-text":            "file 0644 8067926c",
		"/dir/file":                "file 0644 cc55e2ec",
		"/dir/file-copy":           "file 0644 cc55e2ec",
		"/dir/file-copy-2":         "file 0644 cc55e2ec",
		"/dir/foo/":                "dir 0755",
		"/dir/foo/bar/":            "dir 01777",
		"/dir/link/":               "dir 0755",
		"/dir/link/file":           "symlink /dir/file",
		"/dir/link/file-2":         "symlink ../file",
		"/dir/several/":            "dir 0755",
		"/dir/several/levels/":     "dir 0755",
		"/dir/text/":               "dir 0755",
		"/dir/text/file":           "file 0644 5b41362b",
		"/dir/text/file-2":         "file 0755 d98cf53e",
		"/dir/text/file-3":         "file 0644 2c26b46b",
		"/dir/text/file-5":         "file 0755 empty",
		"/dir/text/file-6":         "symlink ./file-3",
		"/parent/":                 "dir 01777",
		"/parent/permissions/":     "dir 0764",
		"/parent/permissions/file": "file 0755 722c14b3",
	},
	dbPaths: []string{"/manifest/chisel.db"},
	db: `
{"jsonwall":"1.0","schema":"1.0","count":38}
{"kind":"content","slice":"test-package_manifest","path":"/manifest/chisel.db"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/all-text"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/file"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/file-copy"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/file-copy-2"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/foo/bar/"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/link/file"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/link/file-2"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/several/levels/"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/text/file"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/text/file-2"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/text/file-3"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/text/file-5"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/text/file-6"}
{"kind":"content","slice":"test-package_myslice","path":"/parent/"}
{"kind":"content","slice":"test-package_myslice","path":"/parent/permissions/"}
{"kind":"content","slice":"test-package_myslice","path":"/parent/permissions/file"}
{"kind":"package","name":"test-package","version":"test-package_version","sha256":"test-package_hash","arch":"test-package_arch"}
{"kind":"path","path":"/dir/all-text","mode":"0644","slices":["test-package_myslice"],"sha256":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","final_sha256":"8067926c032c090867013d14fb0eb21ae858344f62ad07086fd32375845c91a6","size":21}
{"kind":"path","path":"/dir/file","mode":"0644","slices":["test-package_myslice"],"sha256":"cc55e2ecf36e40171ded57167c38e1025c99dc8f8bcdd6422368385a977ae1fe","size":14}
{"kind":"path","path":"/dir/file-copy","mode":"0644","slices":["test-package_myslice"],"sha256":"cc55e2ecf36e40171ded57167c38e1025c99dc8f8bcdd6422368385a977ae1fe","size":14}
{"kind":"path","path":"/dir/file-copy-2","mode":"0644","slices":["test-package_myslice"],"sha256":"cc55e2ecf36e40171ded57167c38e1025c99dc8f8bcdd6422368385a977ae1fe","size":14}
{"kind":"path","path":"/dir/foo/bar/","mode":"01777","slices":["test-package_myslice"]}
{"kind":"path","path":"/dir/link/file","mode":"0644","slices":["test-package_myslice"],"link":"/dir/file"}
{"kind":"path","path":"/dir/link/file-2","mode":"0777","slices":["test-package_myslice"],"link":"../file"}
{"kind":"path","path":"/dir/several/levels/","mode":"0755","slices":["test-package_myslice"]}
{"kind":"path","path":"/dir/text/file","mode":"0644","slices":["test-package_myslice"],"sha256":"5b41362bc82b7f3d56edc5a306db22105707d01ff4819e26faef9724a2d406c9","size":5}
{"kind":"path","path":"/dir/text/file-2","mode":"0755","slices":["test-package_myslice"],"sha256":"d98cf53e0c8b77c14a96358d5b69584225b4bb9026423cbc2f7b0161894c402c","size":5}
{"kind":"path","path":"/dir/text/file-3","mode":"0644","slices":["test-package_myslice"],"sha256":"f60f2d65da046fcaaf8a10bd96b5630104b629e111aff46ce89792e1caa11b18","final_sha256":"2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae","size":3}
{"kind":"path","path":"/dir/text/file-5","mode":"0755","slices":["test-package_myslice"],"sha256":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}
{"kind":"path","path":"/dir/text/file-6","mode":"0644","slices":["test-package_myslice"],"link":"./file-3"}
{"kind":"path","path":"/manifest/chisel.db","mode":"0644","slices":["test-package_manifest"]}
{"kind":"path","path":"/parent/","mode":"01777","slices":["test-package_myslice"]}
{"kind":"path","path":"/parent/permissions/","mode":"0764","slices":["test-package_myslice"]}
{"kind":"path","path":"/parent/permissions/file","mode":"0755","slices":["test-package_myslice"],"sha256":"722c14b3fe33f2a36e4e02c0034951d2a6820ad11e0bd633ffa90d09754640cc","size":5}
{"kind":"slice","name":"test-package_manifest"}
{"kind":"slice","name":"test-package_myslice"}
`,
}, {
	summary: "Multiple DBs",
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				manifest-1:
					contents:
						/dir/file:
						/manifest-1/**:   {generate: manifest}
				manifest-2:
					contents:
						/dir/other-file:
						/manifest-2/**: {generate: manifest}
		`,
	},
	slices: []setup.SliceKey{
		{"test-package", "manifest-1"},
		{"test-package", "manifest-2"},
	},
	filesystem: map[string]string{
		"/dir/":                 "dir 0755",
		"/dir/file":             "file 0644 cc55e2ec",
		"/dir/other-file":       "file 0644 63d5dd49",
		"/manifest-1/":          "dir 0755",
		"/manifest-1/chisel.db": "file 0644 67047a13",
		"/manifest-2/":          "dir 0755",
		"/manifest-2/chisel.db": "file 0644 67047a13",
	},
	dbPaths: []string{"/manifest-1/chisel.db", "/manifest-2/chisel.db"},
	db: `
{"jsonwall":"1.0","schema":"1.0","count":12}
{"kind":"content","slice":"test-package_manifest-1","path":"/dir/file"}
{"kind":"content","slice":"test-package_manifest-1","path":"/manifest-1/chisel.db"}
{"kind":"content","slice":"test-package_manifest-2","path":"/dir/other-file"}
{"kind":"content","slice":"test-package_manifest-2","path":"/manifest-2/chisel.db"}
{"kind":"package","name":"test-package","version":"test-package_version","sha256":"test-package_hash","arch":"test-package_arch"}
{"kind":"path","path":"/dir/file","mode":"0644","slices":["test-package_manifest-1"],"sha256":"cc55e2ecf36e40171ded57167c38e1025c99dc8f8bcdd6422368385a977ae1fe","size":14}
{"kind":"path","path":"/dir/other-file","mode":"0644","slices":["test-package_manifest-2"],"sha256":"63d5dd494bf949a0d10fed7a6a419cfd9609caff766e9af65170ff350ae0fa57","size":7}
{"kind":"path","path":"/manifest-1/chisel.db","mode":"0644","slices":["test-package_manifest-1"]}
{"kind":"path","path":"/manifest-2/chisel.db","mode":"0644","slices":["test-package_manifest-2"]}
{"kind":"slice","name":"test-package_manifest-1"}
{"kind":"slice","name":"test-package_manifest-2"}
`,
}, {
	summary: "No DB if corresponding slice(s) are not selected",
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/file:
				manifest:
					contents:
						/manifest/**: {generate: manifest}
		`,
	},
	slices: []setup.SliceKey{
		{"test-package", "myslice"},
	},
	filesystem: map[string]string{
		"/dir/":     "dir 0755",
		"/dir/file": "file 0644 cc55e2ec",
	},
}, {
	summary: "Copyright is extracted implicitly but not recorded in the db",
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				myslice:
					contents:
						/dir/file:
				manifest:
					contents:
						/manifest/**: {generate: manifest}
		`,
	},
	pkgs: map[string][]byte{
		"test-package": testutil.MustMakeDeb(
			append(testutil.TestPackageEntries,
				testutil.Dir(0755, "./usr/"),
				testutil.Dir(0755, "./usr/share/"),
				testutil.Dir(0755, "./usr/share/doc/"),
				testutil.Dir(0755, "./usr/share/doc/test-package/"),
				testutil.Reg(0644, "./usr/share/doc/test-package/copyright", "copyright"),
			),
		),
	},
	slices: []setup.SliceKey{
		{"test-package", "myslice"},
		{"test-package", "manifest"},
	},
	filesystem: map[string]string{
		"/manifest/":                            "dir 0755",
		"/manifest/chisel.db":                   "file 0644 ac8e6a97",
		"/dir/":                                 "dir 0755",
		"/dir/file":                             "file 0644 cc55e2ec",
		"/usr/":                                 "dir 0755",
		"/usr/share/":                           "dir 0755",
		"/usr/share/doc/":                       "dir 0755",
		"/usr/share/doc/test-package/":          "dir 0755",
		"/usr/share/doc/test-package/copyright": "file 0644 c2fca2aa",
	},
	db: `
{"jsonwall":"1.0","schema":"1.0","count":8}
{"kind":"content","slice":"test-package_manifest","path":"/manifest/chisel.db"}
{"kind":"content","slice":"test-package_myslice","path":"/dir/file"}
{"kind":"package","name":"test-package","version":"test-package_version","sha256":"test-package_hash","arch":"test-package_arch"}
{"kind":"path","path":"/dir/file","mode":"0644","slices":["test-package_myslice"],"sha256":"cc55e2ecf36e40171ded57167c38e1025c99dc8f8bcdd6422368385a977ae1fe","size":14}
{"kind":"path","path":"/manifest/chisel.db","mode":"0644","slices":["test-package_manifest"]}
{"kind":"slice","name":"test-package_manifest"}
{"kind":"slice","name":"test-package_myslice"}
`,
	dbPaths: []string{"/manifest/chisel.db"},
}, {
	summary: "Implicit parent permissions for manifest directory",
	release: map[string]string{
		"slices/mydir/test-package.yaml": `
			package: test-package
			slices:
				manifest:
					contents:
						/manifest/**: {generate: manifest}
		`,
	},
	pkgs: map[string][]byte{
		"test-package": testutil.MustMakeDeb(
			append(testutil.TestPackageEntries, testutil.Dir(0764, "./manifest/")),
		),
	},
	slices: []setup.SliceKey{
		{"test-package", "manifest"},
	},
	filesystem: map[string]string{
		// Parent directory permissions are preserved.
		"/manifest/":          "dir 0764",
		"/manifest/chisel.db": "file 0644 71961c3d",
	},
	dbPaths: []string{"/manifest/chisel.db"},
	db: `
{"jsonwall":"1.0","schema":"1.0","count":5}
{"kind":"content","slice":"test-package_manifest","path":"/manifest/chisel.db"}
{"kind":"package","name":"test-package","version":"test-package_version","sha256":"test-package_hash","arch":"test-package_arch"}
{"kind":"path","path":"/manifest/chisel.db","mode":"0644","slices":["test-package_manifest"]}
{"kind":"slice","name":"test-package_manifest"}
`,
}}

var defaultChiselYaml = `
	format: v1
	archives:
		ubuntu:
			version: 22.04
			components: [main, universe]
			public-keys: [test-key]
	public-keys:
		test-key:
			id: ` + testKey.ID + `
			armor: |` + "\n" + testutil.PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t") + `
`

func (s *S) TestCut(c *C) {
	for _, test := range cutTests {
		c.Logf("Summary: %s", test.summary)

		if _, ok := test.release["chisel.yaml"]; !ok {
			test.release["chisel.yaml"] = string(defaultChiselYaml)
		}

		if test.pkgs == nil {
			test.pkgs = map[string][]byte{
				"test-package": testutil.PackageData["test-package"],
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

		selection, err := setup.Select(release, test.slices)
		c.Assert(err, IsNil)

		archives := map[string]archive.Archive{}
		for name, setupArchive := range release.Archives {
			archive := &testutil.TestArchive{
				Opts: archive.Options{
					Label:      setupArchive.Name,
					Version:    setupArchive.Version,
					Suites:     setupArchive.Suites,
					Components: setupArchive.Components,
				},
				Pkgs: test.pkgs,
			}
			archives[name] = archive
		}

		targetDir := c.MkDir()
		err = cut.Run(&cut.RunOptions{
			Selection: selection,
			Archives:  archives,
			TargetDir: targetDir,
		})
		if test.err != "" {
			c.Assert(err, ErrorMatches, test.err)
			continue
		}
		c.Assert(err, IsNil)

		if test.filesystem != nil {
			c.Assert(testutil.TreeDump(targetDir), DeepEquals, test.filesystem)
		}

		test.db = strings.TrimLeft(test.db, "\n")
		if test.dbPaths != nil {
			for _, path := range test.dbPaths {
				db, err := readZSTDFile(filepath.Join(targetDir, path))
				c.Assert(err, IsNil)
				c.Assert(db, DeepEquals, test.db)
			}
		}
	}
}

func readZSTDFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	reader, err := zstd.NewReader(file)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	bytes, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
