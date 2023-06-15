package slicer_test

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/db"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/slicer"
	"github.com/canonical/chisel/internal/testutil"
)

var (
	Reg = testutil.Reg
	Dir = testutil.Dir
	Lnk = testutil.Lnk
)

type testPackage struct {
	info    map[string]string
	content []byte
}

type slicerTest struct {
	summary string
	arch    string
	pkgs    map[string]map[string]testPackage
	release map[string]string
	slices  []setup.SliceKey
	hackopt func(c *C, opts *slicer.RunOptions)
	result  map[string]string
	db      []any
	error   string
}

var packageEntries = map[string][]testutil.TarEntry{
	"copyright-symlink-libssl3": {
		{Header: tar.Header{Name: "./"}},
		{Header: tar.Header{Name: "./usr/"}},
		{Header: tar.Header{Name: "./usr/lib/"}},
		{Header: tar.Header{Name: "./usr/lib/x86_64-linux-gnu/"}},
		{Header: tar.Header{Name: "./usr/lib/x86_64-linux-gnu/libssl.so.3", Mode: 00755}},
		{Header: tar.Header{Name: "./usr/share/"}},
		{Header: tar.Header{Name: "./usr/share/doc/"}},
		{Header: tar.Header{Name: "./usr/share/doc/copyright-symlink-libssl3/"}},
		{Header: tar.Header{Name: "./usr/share/doc/copyright-symlink-libssl3/copyright"}},
	},
	"copyright-symlink-openssl": {
		{Header: tar.Header{Name: "./"}},
		{Header: tar.Header{Name: "./etc/"}},
		{Header: tar.Header{Name: "./etc/ssl/"}},
		{Header: tar.Header{Name: "./etc/ssl/openssl.cnf"}},
		{Header: tar.Header{Name: "./usr/"}},
		{Header: tar.Header{Name: "./usr/bin/"}},
		{Header: tar.Header{Name: "./usr/bin/openssl", Mode: 00755}},
		{Header: tar.Header{Name: "./usr/share/"}},
		{Header: tar.Header{Name: "./usr/share/doc/"}},
		{Header: tar.Header{Name: "./usr/share/doc/copyright-symlink-openssl/"}},
		{Header: tar.Header{Name: "./usr/share/doc/copyright-symlink-openssl/copyright", Linkname: "../libssl3/copyright"}},
	},
}

// filesystem entries of copyright file from base-files package that will be
// automatically injected into every slice
var copyrightEntries = map[string]string{
	"/usr/":                               "dir 0755",
	"/usr/share/":                         "dir 0755",
	"/usr/share/doc/":                     "dir 0755",
	"/usr/share/doc/base-files/":          "dir 0755",
	"/usr/share/doc/base-files/copyright": "file 0644 cdb5461d",
}

var slicerTests = []slicerTest{{
	summary: "Basic slicing",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/usr/bin/hello:
						/usr/bin/hallo: {copy: /usr/bin/hello}
						/bin/hallo:     {symlink: ../usr/bin/hallo}
						/etc/passwd:    {text: data1}
						/etc/dir/sub/:  {make: true, mode: 01777}
		`,
	},
	result: map[string]string{
		"/usr/":          "dir 0755",
		"/usr/bin/":      "dir 0755",
		"/usr/bin/hello": "file 0775 eaf29575",
		"/usr/bin/hallo": "file 0775 eaf29575",
		"/bin/":          "dir 0755",
		"/bin/hallo":     "symlink ../usr/bin/hallo",
		"/etc/":          "dir 0755",
		"/etc/dir/":      "dir 0755",
		"/etc/dir/sub/":  "dir 01777",
		"/etc/passwd":    "file 0644 5b41362b",
	},
	db: []any{
		db.Package{
			Name:    "base-files",
			Version: "1.0",
		},
		db.Slice{
			Name: "base-files_myslice",
		},
		db.Path{
			Path:   "/bin/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/bin/hallo",
			Mode:   0644,
			Slices: []string{"base-files_myslice"},
			Link:   "../usr/bin/hallo",
		},
		db.Path{
			Path:   "/etc/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/etc/dir/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/etc/dir/sub/",
			Mode:   0777,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/etc/passwd",
			Mode:   0644,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0x5b, 0x41, 0x36, 0x2b, 0xc8, 0x2b, 0x7f, 0x3d,
				0x56, 0xed, 0xc5, 0xa3, 0x06, 0xdb, 0x22, 0x10,
				0x57, 0x07, 0xd0, 0x1f, 0xf4, 0x81, 0x9e, 0x26,
				0xfa, 0xef, 0x97, 0x24, 0xa2, 0xd4, 0x06, 0xc9,
			},
			Size: 5,
		},
		db.Path{
			Path:   "/usr/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/usr/bin/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/usr/bin/hallo",
			Mode:   0775,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0xea, 0xf2, 0x95, 0x75, 0x43, 0x07, 0x7e, 0x93,
				0x01, 0x5b, 0x0b, 0x2e, 0x06, 0xeb, 0xe3, 0x20,
				0xed, 0x56, 0x8e, 0xf8, 0x53, 0x24, 0x5c, 0x7e,
				0xbe, 0xdf, 0x33, 0xc4, 0xe4, 0x5c, 0xf4, 0x0d,
			},
			Size: 29,
		},
		db.Path{
			Path:   "/usr/bin/hello",
			Mode:   0775,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0xea, 0xf2, 0x95, 0x75, 0x43, 0x07, 0x7e, 0x93,
				0x01, 0x5b, 0x0b, 0x2e, 0x06, 0xeb, 0xe3, 0x20,
				0xed, 0x56, 0x8e, 0xf8, 0x53, 0x24, 0x5c, 0x7e,
				0xbe, 0xdf, 0x33, 0xc4, 0xe4, 0x5c, 0xf4, 0x0d,
			},
			Size: 29,
		},
		db.Path{
			Path:   "/usr/share/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/copyright",
			Mode:   0644,
			Slices: []string(nil),
			SHA256: &[...]byte{
				0xcd, 0xb5, 0x46, 0x1d, 0x85, 0x15, 0x00, 0x2d,
				0x0f, 0xe3, 0xba, 0xbb, 0x76, 0x4e, 0xec, 0x38,
				0x77, 0x45, 0x8b, 0x20, 0xf4, 0xe4, 0xbb, 0x16,
				0x21, 0x9f, 0x62, 0xea, 0x95, 0x3a, 0xfe, 0xea,
			},
			Size: 1228,
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/bin/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/bin/hallo",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/etc/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/etc/dir/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/etc/dir/sub/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/etc/passwd",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/bin/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/bin/hallo",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/bin/hello",
		},
	},
}, {
	summary: "Glob extraction",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/**/he*o:
		`,
	},
	result: map[string]string{
		"/usr/":          "dir 0755",
		"/usr/bin/":      "dir 0755",
		"/usr/bin/hello": "file 0775 eaf29575",
	},
	db: []any{
		db.Package{
			Name:    "base-files",
			Version: "1.0",
		},
		db.Slice{
			Name: "base-files_myslice",
		},
		db.Path{
			Path:   "/usr/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/usr/bin/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/usr/bin/hello",
			Mode:   0775,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0xea, 0xf2, 0x95, 0x75, 0x43, 0x07, 0x7e, 0x93,
				0x01, 0x5b, 0x0b, 0x2e, 0x06, 0xeb, 0xe3, 0x20,
				0xed, 0x56, 0x8e, 0xf8, 0x53, 0x24, 0x5c, 0x7e,
				0xbe, 0xdf, 0x33, 0xc4, 0xe4, 0x5c, 0xf4, 0x0d,
			},
			Size: 29,
		},
		db.Path{
			Path:   "/usr/share/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/copyright",
			Mode:   0644,
			Slices: []string(nil),
			SHA256: &[...]byte{
				0xcd, 0xb5, 0x46, 0x1d, 0x85, 0x15, 0x00, 0x2d,
				0x0f, 0xe3, 0xba, 0xbb, 0x76, 0x4e, 0xec, 0x38,
				0x77, 0x45, 0x8b, 0x20, 0xf4, 0xe4, 0xbb, 0x16,
				0x21, 0x9f, 0x62, 0xea, 0x95, 0x3a, 0xfe, 0xea,
			},
			Size: 1228,
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/bin/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/bin/hello",
		},
	},
}, {
	summary: "Create new file under extracted directory",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						# Note the missing /tmp/ here.
						/tmp/new: {text: data1}
		`,
	},
	result: map[string]string{
		"/tmp/":    "dir 01777", // This is the magic.
		"/tmp/new": "file 0644 5b41362b",
	},
	db: []any{
		db.Package{
			Name:    "base-files",
			Version: "1.0",
		},
		db.Slice{
			Name: "base-files_myslice",
		},
		db.Path{
			Path:   "/tmp/",
			Mode:   0777,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/tmp/new",
			Mode:   0644,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0x5b, 0x41, 0x36, 0x2b, 0xc8, 0x2b, 0x7f, 0x3d,
				0x56, 0xed, 0xc5, 0xa3, 0x06, 0xdb, 0x22, 0x10,
				0x57, 0x07, 0xd0, 0x1f, 0xf4, 0x81, 0x9e, 0x26,
				0xfa, 0xef, 0x97, 0x24, 0xa2, 0xd4, 0x06, 0xc9,
			},
			Size: 5,
		},
		db.Path{
			Path:   "/usr/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/copyright",
			Mode:   0644,
			Slices: []string(nil),
			SHA256: &[...]byte{
				0xcd, 0xb5, 0x46, 0x1d, 0x85, 0x15, 0x00, 0x2d,
				0x0f, 0xe3, 0xba, 0xbb, 0x76, 0x4e, 0xec, 0x38,
				0x77, 0x45, 0x8b, 0x20, 0xf4, 0xe4, 0xbb, 0x16,
				0x21, 0x9f, 0x62, 0xea, 0x95, 0x3a, 0xfe, 0xea,
			},
			Size: 1228,
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/tmp/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/tmp/new",
		},
	},
}, {
	summary: "Create new nested file under extracted directory",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						# Note the missing /tmp/ here.
						/tmp/new/sub: {text: data1}
		`,
	},
	result: map[string]string{
		"/tmp/":        "dir 01777", // This is the magic.
		"/tmp/new/":    "dir 0755",
		"/tmp/new/sub": "file 0644 5b41362b",
	},
	db: []any{
		db.Package{
			Name:    "base-files",
			Version: "1.0",
		},
		db.Slice{
			Name: "base-files_myslice",
		},
		db.Path{
			Path:   "/tmp/",
			Mode:   0777,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/tmp/new/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/tmp/new/sub",
			Mode:   0644,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0x5b, 0x41, 0x36, 0x2b, 0xc8, 0x2b, 0x7f, 0x3d,
				0x56, 0xed, 0xc5, 0xa3, 0x06, 0xdb, 0x22, 0x10,
				0x57, 0x07, 0xd0, 0x1f, 0xf4, 0x81, 0x9e, 0x26,
				0xfa, 0xef, 0x97, 0x24, 0xa2, 0xd4, 0x06, 0xc9,
			},
			Size: 5,
		},
		db.Path{
			Path:   "/usr/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/copyright",
			Mode:   0644,
			Slices: []string(nil),
			SHA256: &[...]byte{
				0xcd, 0xb5, 0x46, 0x1d, 0x85, 0x15, 0x00, 0x2d,
				0x0f, 0xe3, 0xba, 0xbb, 0x76, 0x4e, 0xec, 0x38,
				0x77, 0x45, 0x8b, 0x20, 0xf4, 0xe4, 0xbb, 0x16,
				0x21, 0x9f, 0x62, 0xea, 0x95, 0x3a, 0xfe, 0xea,
			},
			Size: 1228,
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/tmp/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/tmp/new/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/tmp/new/sub",
		},
	},
}, {
	summary: "Create new directory under extracted directory",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						# Note the missing /tmp/ here.
						/tmp/new/: {make: true}
		`,
	},
	result: map[string]string{
		"/tmp/":     "dir 01777", // This is the magic.
		"/tmp/new/": "dir 0755",
	},
	db: []any{
		db.Package{
			Name:    "base-files",
			Version: "1.0",
		},
		db.Slice{
			Name: "base-files_myslice",
		},
		db.Path{
			Path:   "/tmp/",
			Mode:   0777,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/tmp/new/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/usr/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/copyright",
			Mode:   0644,
			Slices: []string(nil),
			SHA256: &[...]byte{
				0xcd, 0xb5, 0x46, 0x1d, 0x85, 0x15, 0x00, 0x2d,
				0x0f, 0xe3, 0xba, 0xbb, 0x76, 0x4e, 0xec, 0x38,
				0x77, 0x45, 0x8b, 0x20, 0xf4, 0xe4, 0xbb, 0x16,
				0x21, 0x9f, 0x62, 0xea, 0x95, 0x3a, 0xfe, 0xea,
			},
			Size: 1228,
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/tmp/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/tmp/new/",
		},
	},
}, {
	summary: "Conditional architecture",
	arch:    "amd64",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/tmp/file1: {text: data1, arch: amd64}
						/tmp/file2: {text: data1, arch: i386}
						/tmp/file3: {text: data1, arch: [i386, amd64]}
						/usr/bin/hello1: {copy: /usr/bin/hello, arch: amd64}
						/usr/bin/hello2: {copy: /usr/bin/hello, arch: i386}
						/usr/bin/hello3: {copy: /usr/bin/hello, arch: [i386, amd64]}
		`,
	},
	result: map[string]string{
		"/tmp/":           "dir 01777",
		"/tmp/file1":      "file 0644 5b41362b",
		"/tmp/file3":      "file 0644 5b41362b",
		"/usr/":           "dir 0755",
		"/usr/bin/":       "dir 0755",
		"/usr/bin/hello1": "file 0775 eaf29575",
		"/usr/bin/hello3": "file 0775 eaf29575",
	},
	db: []any{
		db.Package{
			Name:    "base-files",
			Version: "1.0",
		},
		db.Slice{
			Name: "base-files_myslice",
		},
		db.Path{
			Path:   "/tmp/",
			Mode:   0777,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/tmp/file1",
			Mode:   0644,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0x5b, 0x41, 0x36, 0x2b, 0xc8, 0x2b, 0x7f, 0x3d,
				0x56, 0xed, 0xc5, 0xa3, 0x06, 0xdb, 0x22, 0x10,
				0x57, 0x07, 0xd0, 0x1f, 0xf4, 0x81, 0x9e, 0x26,
				0xfa, 0xef, 0x97, 0x24, 0xa2, 0xd4, 0x06, 0xc9,
			},
			Size: 5,
		},
		db.Path{
			Path:   "/tmp/file3",
			Mode:   0644,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0x5b, 0x41, 0x36, 0x2b, 0xc8, 0x2b, 0x7f, 0x3d,
				0x56, 0xed, 0xc5, 0xa3, 0x06, 0xdb, 0x22, 0x10,
				0x57, 0x07, 0xd0, 0x1f, 0xf4, 0x81, 0x9e, 0x26,
				0xfa, 0xef, 0x97, 0x24, 0xa2, 0xd4, 0x06, 0xc9,
			},
			Size: 5,
		},
		db.Path{
			Path:   "/usr/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/usr/bin/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/usr/bin/hello1",
			Mode:   0775,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0xea, 0xf2, 0x95, 0x75, 0x43, 0x07, 0x7e, 0x93,
				0x01, 0x5b, 0x0b, 0x2e, 0x06, 0xeb, 0xe3, 0x20,
				0xed, 0x56, 0x8e, 0xf8, 0x53, 0x24, 0x5c, 0x7e,
				0xbe, 0xdf, 0x33, 0xc4, 0xe4, 0x5c, 0xf4, 0x0d,
			},
			Size: 29,
		},
		db.Path{
			Path:   "/usr/bin/hello3",
			Mode:   0775,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0xea, 0xf2, 0x95, 0x75, 0x43, 0x07, 0x7e, 0x93,
				0x01, 0x5b, 0x0b, 0x2e, 0x06, 0xeb, 0xe3, 0x20,
				0xed, 0x56, 0x8e, 0xf8, 0x53, 0x24, 0x5c, 0x7e,
				0xbe, 0xdf, 0x33, 0xc4, 0xe4, 0x5c, 0xf4, 0x0d,
			},
			Size: 29,
		},
		db.Path{
			Path:   "/usr/share/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/copyright",
			Mode:   0644,
			Slices: []string(nil),
			SHA256: &[...]byte{
				0xcd, 0xb5, 0x46, 0x1d, 0x85, 0x15, 0x00, 0x2d,
				0x0f, 0xe3, 0xba, 0xbb, 0x76, 0x4e, 0xec, 0x38,
				0x77, 0x45, 0x8b, 0x20, 0xf4, 0xe4, 0xbb, 0x16,
				0x21, 0x9f, 0x62, 0xea, 0x95, 0x3a, 0xfe, 0xea,
			},
			Size: 1228,
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/tmp/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/tmp/file1",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/tmp/file3",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/bin/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/bin/hello1",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/bin/hello3",
		},
	},
}, {
	summary: "Script: write a file",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/tmp/file1: {text: data1, mutable: true}
					mutate: |
						content.write("/tmp/file1", "data2")
		`,
	},
	result: map[string]string{
		"/tmp/":      "dir 01777",
		"/tmp/file1": "file 0644 d98cf53e",
	},
	db: []any{
		db.Package{
			Name:    "base-files",
			Version: "1.0",
		},
		db.Slice{
			Name: "base-files_myslice",
		},
		db.Path{
			Path:   "/tmp/",
			Mode:   0777,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/tmp/file1",
			Mode:   0644,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0x5b, 0x41, 0x36, 0x2b, 0xc8, 0x2b, 0x7f, 0x3d,
				0x56, 0xed, 0xc5, 0xa3, 0x06, 0xdb, 0x22, 0x10,
				0x57, 0x07, 0xd0, 0x1f, 0xf4, 0x81, 0x9e, 0x26,
				0xfa, 0xef, 0x97, 0x24, 0xa2, 0xd4, 0x06, 0xc9,
			},
			Size: 5,
		},
		db.Path{
			Path:   "/usr/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/copyright",
			Mode:   0644,
			Slices: []string(nil),
			SHA256: &[...]byte{
				0xcd, 0xb5, 0x46, 0x1d, 0x85, 0x15, 0x00, 0x2d,
				0x0f, 0xe3, 0xba, 0xbb, 0x76, 0x4e, 0xec, 0x38,
				0x77, 0x45, 0x8b, 0x20, 0xf4, 0xe4, 0xbb, 0x16,
				0x21, 0x9f, 0x62, 0xea, 0x95, 0x3a, 0xfe, 0xea,
			},
			Size: 1228,
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/tmp/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/tmp/file1",
		},
	},
}, {
	summary: "Script: read a file",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/tmp/file1: {text: data1}
						/foo/file2: {text: data2, mutable: true}
					mutate: |
						data = content.read("/tmp/file1")
						content.write("/foo/file2", data)
		`,
	},
	result: map[string]string{
		"/tmp/":      "dir 01777",
		"/tmp/file1": "file 0644 5b41362b",
		"/foo/":      "dir 0755",
		"/foo/file2": "file 0644 5b41362b",
	},
	db: []any{
		db.Package{
			Name:    "base-files",
			Version: "1.0",
		},
		db.Slice{
			Name: "base-files_myslice",
		},
		db.Path{
			Path:   "/foo/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/foo/file2",
			Mode:   0644,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0xd9, 0x8c, 0xf5, 0x3e, 0x0c, 0x8b, 0x77, 0xc1,
				0x4a, 0x96, 0x35, 0x8d, 0x5b, 0x69, 0x58, 0x42,
				0x25, 0xb4, 0xbb, 0x90, 0x26, 0x42, 0x3c, 0xbc,
				0x2f, 0x7b, 0x01, 0x61, 0x89, 0x4c, 0x40, 0x2c,
			},
			Size: 5,
		},
		db.Path{
			Path:   "/tmp/",
			Mode:   0777,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/tmp/file1",
			Mode:   0644,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0x5b, 0x41, 0x36, 0x2b, 0xc8, 0x2b, 0x7f, 0x3d,
				0x56, 0xed, 0xc5, 0xa3, 0x06, 0xdb, 0x22, 0x10,
				0x57, 0x07, 0xd0, 0x1f, 0xf4, 0x81, 0x9e, 0x26,
				0xfa, 0xef, 0x97, 0x24, 0xa2, 0xd4, 0x06, 0xc9,
			},
			Size: 5,
		},
		db.Path{
			Path:   "/usr/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/copyright",
			Mode:   0644,
			Slices: []string(nil),
			SHA256: &[...]byte{
				0xcd, 0xb5, 0x46, 0x1d, 0x85, 0x15, 0x00, 0x2d,
				0x0f, 0xe3, 0xba, 0xbb, 0x76, 0x4e, 0xec, 0x38,
				0x77, 0x45, 0x8b, 0x20, 0xf4, 0xe4, 0xbb, 0x16,
				0x21, 0x9f, 0x62, 0xea, 0x95, 0x3a, 0xfe, 0xea,
			},
			Size: 1228,
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/foo/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/foo/file2",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/tmp/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/tmp/file1",
		},
	},
}, {
	summary: "Script: use 'until' to remove file after mutate",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/tmp/file1: {text: data1, until: mutate}
						/foo/file2: {text: data2, mutable: true}
					mutate: |
						data = content.read("/tmp/file1")
						content.write("/foo/file2", data)
		`,
	},
	result: map[string]string{
		"/tmp/":      "dir 01777",
		"/foo/":      "dir 0755",
		"/foo/file2": "file 0644 5b41362b",
	},
	db: []any{
		db.Package{
			Name:    "base-files",
			Version: "1.0",
		},
		db.Slice{
			Name: "base-files_myslice",
		},
		db.Path{
			Path:   "/foo/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/foo/file2",
			Mode:   0644,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0xd9, 0x8c, 0xf5, 0x3e, 0x0c, 0x8b, 0x77, 0xc1,
				0x4a, 0x96, 0x35, 0x8d, 0x5b, 0x69, 0x58, 0x42,
				0x25, 0xb4, 0xbb, 0x90, 0x26, 0x42, 0x3c, 0xbc,
				0x2f, 0x7b, 0x01, 0x61, 0x89, 0x4c, 0x40, 0x2c,
			},
			Size: 5,
		},
		db.Path{
			Path:   "/tmp/",
			Mode:   0777,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/usr/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/copyright",
			Mode:   0644,
			Slices: []string(nil),
			SHA256: &[...]byte{
				0xcd, 0xb5, 0x46, 0x1d, 0x85, 0x15, 0x00, 0x2d,
				0x0f, 0xe3, 0xba, 0xbb, 0x76, 0x4e, 0xec, 0x38,
				0x77, 0x45, 0x8b, 0x20, 0xf4, 0xe4, 0xbb, 0x16,
				0x21, 0x9f, 0x62, 0xea, 0x95, 0x3a, 0xfe, 0xea,
			},
			Size: 1228,
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/foo/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/foo/file2",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/tmp/",
		},
	},
}, {
	summary: "Script: use 'until' to remove wildcard after mutate",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/usr/bin**:  {until: mutate}
						/etc/passwd: {until: mutate, text: data1}
		`,
	},
	result: map[string]string{
		"/usr/": "dir 0755",
		"/etc/": "dir 0755",
	},
	db: []any{
		db.Package{
			Name:    "base-files",
			Version: "1.0",
		},
		db.Slice{
			Name: "base-files_myslice",
		},
		db.Path{
			Path:   "/etc/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/usr/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/usr/share/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/copyright",
			Mode:   0644,
			Slices: []string(nil),
			SHA256: &[...]byte{
				0xcd, 0xb5, 0x46, 0x1d, 0x85, 0x15, 0x00, 0x2d,
				0x0f, 0xe3, 0xba, 0xbb, 0x76, 0x4e, 0xec, 0x38,
				0x77, 0x45, 0x8b, 0x20, 0xf4, 0xe4, 0xbb, 0x16,
				0x21, 0x9f, 0x62, 0xea, 0x95, 0x3a, 0xfe, 0xea,
			},
			Size: 1228,
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/etc/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/",
		},
	},
}, {
	summary: "Script: 'until' does not remove non-empty directories",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/usr/bin/: {until: mutate}
						/usr/bin/hallo: {copy: /usr/bin/hello}
		`,
	},
	result: map[string]string{
		"/usr/":          "dir 0755",
		"/usr/bin/":      "dir 0755",
		"/usr/bin/hallo": "file 0775 eaf29575",
	},
	db: []any{
		db.Package{
			Name:    "base-files",
			Version: "1.0",
		},
		db.Slice{
			Name: "base-files_myslice",
		},
		db.Path{
			Path:   "/usr/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/usr/bin/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/usr/bin/hallo",
			Mode:   0775,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0xea, 0xf2, 0x95, 0x75, 0x43, 0x07, 0x7e, 0x93,
				0x01, 0x5b, 0x0b, 0x2e, 0x06, 0xeb, 0xe3, 0x20,
				0xed, 0x56, 0x8e, 0xf8, 0x53, 0x24, 0x5c, 0x7e,
				0xbe, 0xdf, 0x33, 0xc4, 0xe4, 0x5c, 0xf4, 0x0d,
			},
			Size: 29,
		},
		db.Path{
			Path:   "/usr/share/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/copyright",
			Mode:   0644,
			Slices: []string(nil),
			SHA256: &[...]byte{
				0xcd, 0xb5, 0x46, 0x1d, 0x85, 0x15, 0x00, 0x2d,
				0x0f, 0xe3, 0xba, 0xbb, 0x76, 0x4e, 0xec, 0x38,
				0x77, 0x45, 0x8b, 0x20, 0xf4, 0xe4, 0xbb, 0x16,
				0x21, 0x9f, 0x62, 0xea, 0x95, 0x3a, 0xfe, 0xea,
			},
			Size: 1228,
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/bin/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/bin/hallo",
		},
	},
}, {
	summary: "Script: cannot write non-mutable files",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/tmp/file1: {text: data1}
					mutate: |
						content.write("/tmp/file1", "data2")
		`,
	},
	error: `slice base-files_myslice: cannot write file which is not mutable: /tmp/file1`,
}, {
	summary: "Script: cannot read unlisted content",
	slices:  []setup.SliceKey{{"base-files", "myslice2"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice1:
					contents:
						/tmp/file1: {text: data1}
				myslice2:
					mutate: |
						content.read("/tmp/file1")
		`,
	},
	error: `slice base-files_myslice2: cannot read file which is not selected: /tmp/file1`,
}, {
	summary: "Script: can read globbed content",
	slices:  []setup.SliceKey{{"base-files", "myslice1"}, {"base-files", "myslice2"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice1:
					contents:
						/usr/bin/*:
				myslice2:
					mutate: |
						content.read("/usr/bin/hello")
		`,
	},
	db: []any{
		db.Package{
			Name:    "base-files",
			Version: "1.0",
		},
		db.Slice{
			Name: "base-files_myslice1",
		},
		db.Slice{
			Name: "base-files_myslice2",
		},
		db.Path{
			Path:   "/usr/",
			Mode:   0755,
			Slices: []string{"base-files_myslice1"},
		},
		db.Path{
			Path:   "/usr/bin/",
			Mode:   0755,
			Slices: []string{"base-files_myslice1"},
		},
		db.Path{
			Path:   "/usr/bin/hello",
			Mode:   0775,
			Slices: []string{"base-files_myslice1"},
			SHA256: &[...]byte{
				0xea, 0xf2, 0x95, 0x75, 0x43, 0x07, 0x7e, 0x93,
				0x01, 0x5b, 0x0b, 0x2e, 0x06, 0xeb, 0xe3, 0x20,
				0xed, 0x56, 0x8e, 0xf8, 0x53, 0x24, 0x5c, 0x7e,
				0xbe, 0xdf, 0x33, 0xc4, 0xe4, 0x5c, 0xf4, 0x0d,
			},
			Size: 29,
		},
		db.Path{
			Path:   "/usr/share/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/copyright",
			Mode:   0644,
			Slices: []string(nil),
			SHA256: &[...]byte{
				0xcd, 0xb5, 0x46, 0x1d, 0x85, 0x15, 0x00, 0x2d,
				0x0f, 0xe3, 0xba, 0xbb, 0x76, 0x4e, 0xec, 0x38,
				0x77, 0x45, 0x8b, 0x20, 0xf4, 0xe4, 0xbb, 0x16,
				0x21, 0x9f, 0x62, 0xea, 0x95, 0x3a, 0xfe, 0xea,
			},
			Size: 1228,
		},
		db.Content{
			Slice: "base-files_myslice1",
			Path:  "/usr/",
		},
		db.Content{
			Slice: "base-files_myslice1",
			Path:  "/usr/bin/",
		},
		db.Content{
			Slice: "base-files_myslice1",
			Path:  "/usr/bin/hello",
		},
	},
}, {
	summary: "Relative content root directory must not error",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/tmp/file1: {text: data1}
					mutate: |
						content.read("/tmp/file1")
		`,
	},
	hackopt: func(c *C, opts *slicer.RunOptions) {
		dir, err := os.Getwd()
		c.Assert(err, IsNil)
		opts.TargetDir, err = filepath.Rel(dir, opts.TargetDir)
		c.Assert(err, IsNil)
	},
	db: []any{
		db.Package{
			Name:    "base-files",
			Version: "1.0",
		},
		db.Slice{
			Name: "base-files_myslice",
		},
		db.Path{
			Path:   "/tmp/",
			Mode:   0777,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/tmp/file1",
			Mode:   0644,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0x5b, 0x41, 0x36, 0x2b, 0xc8, 0x2b, 0x7f, 0x3d,
				0x56, 0xed, 0xc5, 0xa3, 0x06, 0xdb, 0x22, 0x10,
				0x57, 0x07, 0xd0, 0x1f, 0xf4, 0x81, 0x9e, 0x26,
				0xfa, 0xef, 0x97, 0x24, 0xa2, 0xd4, 0x06, 0xc9,
			},
			Size: 5,
		},
		db.Path{
			Path:   "/usr/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/copyright",
			Mode:   0644,
			Slices: []string(nil),
			SHA256: &[...]byte{
				0xcd, 0xb5, 0x46, 0x1d, 0x85, 0x15, 0x00, 0x2d,
				0x0f, 0xe3, 0xba, 0xbb, 0x76, 0x4e, 0xec, 0x38,
				0x77, 0x45, 0x8b, 0x20, 0xf4, 0xe4, 0xbb, 0x16,
				0x21, 0x9f, 0x62, 0xea, 0x95, 0x3a, 0xfe, 0xea,
			},
			Size: 1228,
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/tmp/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/tmp/file1",
		},
	},
}, {
	summary: "Can list parent directories of normal paths",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/a/b/c: {text: foo}
						/x/y/: {make: true}
					mutate: |
						content.list("/")
						content.list("/a")
						content.list("/a/b")
						content.list("/x")
						content.list("/x/y")
		`,
	},
	db: []any{
		db.Package{
			Name:    "base-files",
			Version: "1.0",
		},
		db.Slice{
			Name: "base-files_myslice",
		},
		db.Path{
			Path:   "/a/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/a/b/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/a/b/c",
			Mode:   0644,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0x2c, 0x26, 0xb4, 0x6b, 0x68, 0xff, 0xc6, 0x8f,
				0xf9, 0x9b, 0x45, 0x3c, 0x1d, 0x30, 0x41, 0x34,
				0x13, 0x42, 0x2d, 0x70, 0x64, 0x83, 0xbf, 0xa0,
				0xf9, 0x8a, 0x5e, 0x88, 0x62, 0x66, 0xe7, 0xae,
			},
			Size: 3,
		},
		db.Path{
			Path:   "/usr/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/copyright",
			Mode:   0644,
			Slices: []string(nil),
			SHA256: &[...]byte{
				0xcd, 0xb5, 0x46, 0x1d, 0x85, 0x15, 0x00, 0x2d,
				0x0f, 0xe3, 0xba, 0xbb, 0x76, 0x4e, 0xec, 0x38,
				0x77, 0x45, 0x8b, 0x20, 0xf4, 0xe4, 0xbb, 0x16,
				0x21, 0x9f, 0x62, 0xea, 0x95, 0x3a, 0xfe, 0xea,
			},
			Size: 1228,
		},
		db.Path{
			Path:   "/x/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/x/y/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/a/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/a/b/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/a/b/c",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/x/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/x/y/",
		},
	},
}, {
	summary: "Cannot list unselected directory",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/a/b/c: {text: foo}
					mutate: |
						content.list("/a/d")
		`,
	},
	error: `slice base-files_myslice: cannot list directory which is not selected: /a/d/`,
}, {
	summary: "Cannot list file path as a directory",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/a/b/c: {text: foo}
					mutate: |
						content.list("/a/b/c")
		`,
	},
	error: `slice base-files_myslice: content is not a directory: /a/b/c`,
}, {
	summary: "Can list parent directories of globs",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/**/bin/h?llo:
					mutate: |
						content.list("/usr/bin")
		`,
	},
	db: []any{
		db.Package{
			Name:    "base-files",
			Version: "1.0",
		},
		db.Slice{
			Name: "base-files_myslice",
		},
		db.Path{
			Path:   "/usr/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/usr/bin/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/usr/bin/hello",
			Mode:   0775,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0xea, 0xf2, 0x95, 0x75, 0x43, 0x07, 0x7e, 0x93,
				0x01, 0x5b, 0x0b, 0x2e, 0x06, 0xeb, 0xe3, 0x20,
				0xed, 0x56, 0x8e, 0xf8, 0x53, 0x24, 0x5c, 0x7e,
				0xbe, 0xdf, 0x33, 0xc4, 0xe4, 0x5c, 0xf4, 0x0d,
			},
			Size: 29,
		},
		db.Path{
			Path:   "/usr/share/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/copyright",
			Mode:   0644,
			Slices: []string(nil),
			SHA256: &[...]byte{
				0xcd, 0xb5, 0x46, 0x1d, 0x85, 0x15, 0x00, 0x2d,
				0x0f, 0xe3, 0xba, 0xbb, 0x76, 0x4e, 0xec, 0x38,
				0x77, 0x45, 0x8b, 0x20, 0xf4, 0xe4, 0xbb, 0x16,
				0x21, 0x9f, 0x62, 0xea, 0x95, 0x3a, 0xfe, 0xea,
			},
			Size: 1228,
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/bin/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/bin/hello",
		},
	},
}, {
	summary: "Cannot list directories not matched by glob",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/**/bin/h?llo:
					mutate: |
						content.list("/etc")
		`,
	},
	error: `slice base-files_myslice: cannot list directory which is not selected: /etc/`,
}, {
	summary: "Duplicate copyright symlink is ignored",
	slices:  []setup.SliceKey{{"copyright-symlink-openssl", "bins"}},
	release: map[string]string{
		"slices/mydir/copyright-symlink-libssl3.yaml": `
			package: copyright-symlink-libssl3
			slices:
				libs:
					contents:
						/usr/lib/x86_64-linux-gnu/libssl.so.3:
		`,
		"slices/mydir/copyright-symlink-openssl.yaml": `
			package: copyright-symlink-openssl
			slices:
				bins:
					essential:
						- copyright-symlink-libssl3_libs
						- copyright-symlink-openssl_config
					contents:
						/usr/bin/openssl:
				config:
					contents:
						/etc/ssl/openssl.cnf:
		`,
	},
	db: []any{
		db.Package{
			Name:    "copyright-symlink-libssl3",
			Version: "1.0",
		},
		db.Package{
			Name:    "copyright-symlink-openssl",
			Version: "1.0",
		},
		db.Slice{
			Name: "copyright-symlink-libssl3_libs",
		},
		db.Slice{
			Name: "copyright-symlink-openssl_bins",
		},
		db.Slice{
			Name: "copyright-symlink-openssl_config",
		},
		db.Path{
			Path:   "/etc/",
			Mode:   0755,
			Slices: []string{"copyright-symlink-openssl_config"},
		},
		db.Path{
			Path:   "/etc/ssl/",
			Mode:   0755,
			Slices: []string{"copyright-symlink-openssl_config"},
		},
		db.Path{
			Path:   "/etc/ssl/openssl.cnf",
			Mode:   0644,
			Slices: []string{"copyright-symlink-openssl_config"},
			SHA256: &[...]byte{
				0xe3, 0xb0, 0xc4, 0x42, 0x98, 0xfc, 0x1c, 0x14,
				0x9a, 0xfb, 0xf4, 0xc8, 0x99, 0x6f, 0xb9, 0x24,
				0x27, 0xae, 0x41, 0xe4, 0x64, 0x9b, 0x93, 0x4c,
				0xa4, 0x95, 0x99, 0x1b, 0x78, 0x52, 0xb8, 0x55,
			},
		},
		db.Path{
			Path:   "/usr/",
			Mode:   0755,
			Slices: []string{"copyright-symlink-libssl3_libs", "copyright-symlink-openssl_bins"},
		},
		db.Path{
			Path:   "/usr/bin/",
			Mode:   0755,
			Slices: []string{"copyright-symlink-openssl_bins"},
		},
		db.Path{
			Path:   "/usr/bin/openssl",
			Mode:   0755,
			Slices: []string{"copyright-symlink-openssl_bins"},
			SHA256: &[...]byte{
				0xe3, 0xb0, 0xc4, 0x42, 0x98, 0xfc, 0x1c, 0x14,
				0x9a, 0xfb, 0xf4, 0xc8, 0x99, 0x6f, 0xb9, 0x24,
				0x27, 0xae, 0x41, 0xe4, 0x64, 0x9b, 0x93, 0x4c,
				0xa4, 0x95, 0x99, 0x1b, 0x78, 0x52, 0xb8, 0x55,
			},
		},
		db.Path{
			Path:   "/usr/lib/",
			Mode:   0755,
			Slices: []string{"copyright-symlink-libssl3_libs"},
		},
		db.Path{
			Path:   "/usr/lib/x86_64-linux-gnu/",
			Mode:   0755,
			Slices: []string{"copyright-symlink-libssl3_libs"},
		},
		db.Path{
			Path:   "/usr/lib/x86_64-linux-gnu/libssl.so.3",
			Mode:   0755,
			Slices: []string{"copyright-symlink-libssl3_libs"},
			SHA256: &[...]byte{
				0xe3, 0xb0, 0xc4, 0x42, 0x98, 0xfc, 0x1c, 0x14,
				0x9a, 0xfb, 0xf4, 0xc8, 0x99, 0x6f, 0xb9, 0x24,
				0x27, 0xae, 0x41, 0xe4, 0x64, 0x9b, 0x93, 0x4c,
				0xa4, 0x95, 0x99, 0x1b, 0x78, 0x52, 0xb8, 0x55,
			},
		},
		db.Path{
			Path:   "/usr/share/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/copyright-symlink-libssl3/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/copyright-symlink-libssl3/copyright",
			Mode:   0644,
			Slices: []string(nil),
			SHA256: &[...]byte{
				0xe3, 0xb0, 0xc4, 0x42, 0x98, 0xfc, 0x1c, 0x14,
				0x9a, 0xfb, 0xf4, 0xc8, 0x99, 0x6f, 0xb9, 0x24,
				0x27, 0xae, 0x41, 0xe4, 0x64, 0x9b, 0x93, 0x4c,
				0xa4, 0x95, 0x99, 0x1b, 0x78, 0x52, 0xb8, 0x55,
			},
		},
		db.Path{
			Path:   "/usr/share/doc/copyright-symlink-openssl/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/copyright-symlink-openssl/copyright",
			Mode:   0777,
			Slices: []string(nil),
			Link:   "../libssl3/copyright",
		},
		db.Content{
			Slice: "copyright-symlink-libssl3_libs",
			Path:  "/usr/",
		},
		db.Content{
			Slice: "copyright-symlink-libssl3_libs",
			Path:  "/usr/lib/",
		},
		db.Content{
			Slice: "copyright-symlink-libssl3_libs",
			Path:  "/usr/lib/x86_64-linux-gnu/",
		},
		db.Content{
			Slice: "copyright-symlink-libssl3_libs",
			Path:  "/usr/lib/x86_64-linux-gnu/libssl.so.3",
		},
		db.Content{
			Slice: "copyright-symlink-openssl_bins",
			Path:  "/usr/",
		},
		db.Content{
			Slice: "copyright-symlink-openssl_bins",
			Path:  "/usr/bin/",
		},
		db.Content{
			Slice: "copyright-symlink-openssl_bins",
			Path:  "/usr/bin/openssl",
		},
		db.Content{
			Slice: "copyright-symlink-openssl_config",
			Path:  "/etc/",
		},
		db.Content{
			Slice: "copyright-symlink-openssl_config",
			Path:  "/etc/ssl/",
		},
		db.Content{
			Slice: "copyright-symlink-openssl_config",
			Path:  "/etc/ssl/openssl.cnf",
		},
	},
}, {
	summary: "Can list unclean directory paths",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/a/b/c: {text: foo}
						/x/y/: {make: true}
					mutate: |
						content.list("/////")
						content.list("/a/")
						content.list("/a/b/../b/")
						content.list("/x///")
						content.list("/x/./././y")
		`,
	},
	db: []any{
		db.Package{
			Name:    "base-files",
			Version: "1.0",
		},
		db.Slice{
			Name: "base-files_myslice",
		},
		db.Path{
			Path:   "/a/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/a/b/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/a/b/c",
			Mode:   0644,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0x2c, 0x26, 0xb4, 0x6b, 0x68, 0xff, 0xc6, 0x8f,
				0xf9, 0x9b, 0x45, 0x3c, 0x1d, 0x30, 0x41, 0x34,
				0x13, 0x42, 0x2d, 0x70, 0x64, 0x83, 0xbf, 0xa0,
				0xf9, 0x8a, 0x5e, 0x88, 0x62, 0x66, 0xe7, 0xae,
			},
			Size: 3,
		},
		db.Path{
			Path:   "/usr/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/copyright",
			Mode:   0644,
			Slices: []string(nil),
			SHA256: &[...]byte{
				0xcd, 0xb5, 0x46, 0x1d, 0x85, 0x15, 0x00, 0x2d,
				0x0f, 0xe3, 0xba, 0xbb, 0x76, 0x4e, 0xec, 0x38,
				0x77, 0x45, 0x8b, 0x20, 0xf4, 0xe4, 0xbb, 0x16,
				0x21, 0x9f, 0x62, 0xea, 0x95, 0x3a, 0xfe, 0xea,
			},
			Size: 1228,
		},
		db.Path{
			Path:   "/x/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/x/y/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/a/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/a/b/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/a/b/c",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/x/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/x/y/",
		},
	},
}, {
	summary: "Cannot read directories",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"slices/mydir/base-files.yaml": `
			package: base-files
			slices:
				myslice:
					contents:
						/x/y/: {make: true}
					mutate: |
						content.read("/x/y")
		`,
	},
	error: `slice base-files_myslice: content is not a file: /x/y`,
}, {
	summary: "Non-default archive",
	slices:  []setup.SliceKey{{"base-files", "myslice"}},
	release: map[string]string{
		"chisel.yaml": `
			format: chisel-v1
			archives:
				foo:
					version: 22.04
					components: [main, universe]
					default: true
				bar:
					version: 22.04
					components: [main]
		`,
		"slices/mydir/base-files.yaml": `
			package: base-files
			archive: bar
			slices:
				myslice:
					contents:
						/usr/bin/hello:
		`,
	},
	result: map[string]string{
		"/usr/":          "dir 0755",
		"/usr/bin/":      "dir 0755",
		"/usr/bin/hello": "file 0775 eaf29575",
	},
	db: []any{
		db.Package{
			Name:    "base-files",
			Version: "1.0",
		},
		db.Slice{
			Name: "base-files_myslice",
		},
		db.Path{
			Path:   "/usr/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/usr/bin/",
			Mode:   0755,
			Slices: []string{"base-files_myslice"},
		},
		db.Path{
			Path:   "/usr/bin/hello",
			Mode:   0775,
			Slices: []string{"base-files_myslice"},
			SHA256: &[...]byte{
				0xea, 0xf2, 0x95, 0x75, 0x43, 0x07, 0x7e, 0x93,
				0x01, 0x5b, 0x0b, 0x2e, 0x06, 0xeb, 0xe3, 0x20,
				0xed, 0x56, 0x8e, 0xf8, 0x53, 0x24, 0x5c, 0x7e,
				0xbe, 0xdf, 0x33, 0xc4, 0xe4, 0x5c, 0xf4, 0x0d,
			},
			Size: 29,
		},
		db.Path{
			Path:   "/usr/share/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/base-files/copyright",
			Mode:   0644,
			Slices: []string(nil),
			SHA256: &[...]byte{
				0xcd, 0xb5, 0x46, 0x1d, 0x85, 0x15, 0x00, 0x2d,
				0x0f, 0xe3, 0xba, 0xbb, 0x76, 0x4e, 0xec, 0x38,
				0x77, 0x45, 0x8b, 0x20, 0xf4, 0xe4, 0xbb, 0x16,
				0x21, 0x9f, 0x62, 0xea, 0x95, 0x3a, 0xfe, 0xea,
			},
			Size: 1228,
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/bin/",
		},
		db.Content{
			Slice: "base-files_myslice",
			Path:  "/usr/bin/hello",
		},
	},
}, {
	summary: "Custom archives with custom packages",
	pkgs: map[string]map[string]testPackage{
		"leptons": {
			"electron": testPackage{
				content: testutil.MustMakeDeb([]testutil.TarEntry{
					Dir(0755, "./"),
					Dir(0755, "./mass/"),
					Reg(0644, "./mass/electron", "9.1093837015E−31 kg\n"),
					Dir(0755, "./usr/"),
					Dir(0755, "./usr/share/"),
					Dir(0755, "./usr/share/doc/"),
					Dir(0755, "./usr/share/doc/electron/"),
					Reg(0644, "./usr/share/doc/electron/copyright", ""),
				}),
			},
		},
		"hadrons": {
			"proton": testPackage{
				content: testutil.MustMakeDeb([]testutil.TarEntry{
					Dir(0755, "./"),
					Dir(0755, "./mass/"),
					Reg(0644, "./mass/proton", "1.67262192369E−27 kg\n"),
				}),
			},
		},
	},
	release: map[string]string{
		"chisel.yaml": `
			format: chisel-v1
			archives:
				leptons:
					version: 1
					suites: [main]
					components: [main, universe]
					default: true
				hadrons:
					version: 1
					suites: [main]
					components: [main]
		`,
		"slices/mydir/electron.yaml": `
			package: electron
			slices:
				mass:
					contents:
						/mass/electron:
		`,
		"slices/mydir/proton.yaml": `
			package: proton
			archive: hadrons
			slices:
				mass:
					contents:
						/mass/proton:
		`,
	},
	slices: []setup.SliceKey{
		{"electron", "mass"},
		{"proton", "mass"},
	},
	result: map[string]string{
		"/mass/":                            "dir 0755",
		"/mass/electron":                    "file 0644 a1258e30",
		"/mass/proton":                      "file 0644 a2390d10",
		"/usr/":                             "dir 0755",
		"/usr/share/":                       "dir 0755",
		"/usr/share/doc/":                   "dir 0755",
		"/usr/share/doc/electron/":          "dir 0755",
		"/usr/share/doc/electron/copyright": "file 0644 empty",
	},
	db: []any{
		db.Package{
			Name:    "electron",
			Version: "1.0",
		},
		db.Package{
			Name:    "proton",
			Version: "1.0",
		},
		db.Slice{
			Name: "electron_mass",
		},
		db.Slice{
			Name: "proton_mass",
		},
		db.Path{
			Path:   "/mass/",
			Mode:   0755,
			Slices: []string{"electron_mass", "proton_mass"},
		},
		db.Path{
			Path:   "/mass/electron",
			Mode:   0644,
			Slices: []string{"electron_mass"},
			SHA256: &[...]byte{
				0xa1, 0x25, 0x8e, 0x30, 0x83, 0xf9, 0xa2, 0xd9,
				0x94, 0xc5, 0x0d, 0xea, 0x22, 0xf3, 0xaf, 0x9e,
				0xaa, 0x32, 0x39, 0x6b, 0x37, 0xba, 0xb6, 0x15,
				0x2b, 0x00, 0xd5, 0x04, 0x9e, 0x76, 0x75, 0x16,
			},
			Size: 22,
		},
		db.Path{
			Path:   "/mass/proton",
			Mode:   0644,
			Slices: []string{"proton_mass"},
			SHA256: &[...]byte{
				0xa2, 0x39, 0x0d, 0x10, 0x58, 0x70, 0xf8, 0xe0,
				0xd8, 0xa6, 0x04, 0x60, 0xf8, 0x5d, 0x3f, 0x49,
				0x3a, 0x11, 0xe3, 0xcc, 0xec, 0xff, 0xef, 0x06,
				0x51, 0xbb, 0x60, 0xfb, 0xc9, 0x36, 0xc7, 0x66,
			},
			Size: 23,
		},
		db.Path{
			Path:   "/usr/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/electron/",
			Mode:   0755,
			Slices: []string(nil),
		},
		db.Path{
			Path:   "/usr/share/doc/electron/copyright",
			Mode:   0644,
			Slices: []string(nil),
			SHA256: &[...]byte{
				0xe3, 0xb0, 0xc4, 0x42, 0x98, 0xfc, 0x1c, 0x14,
				0x9a, 0xfb, 0xf4, 0xc8, 0x99, 0x6f, 0xb9, 0x24,
				0x27, 0xae, 0x41, 0xe4, 0x64, 0x9b, 0x93, 0x4c,
				0xa4, 0x95, 0x99, 0x1b, 0x78, 0x52, 0xb8, 0x55,
			},
		},
		db.Content{
			Slice: "electron_mass",
			Path:  "/mass/",
		},
		db.Content{
			Slice: "electron_mass",
			Path:  "/mass/electron",
		},
		db.Content{
			Slice: "proton_mass",
			Path:  "/mass/",
		},
		db.Content{
			Slice: "proton_mass",
			Path:  "/mass/proton",
		},
	},
}}

const defaultChiselYaml = `
	format: chisel-v1
	archives:
		ubuntu:
			version: 22.04
			components: [main, universe]
`

type testPackageInfo map[string]string

var _ archive.PackageInfo = (testPackageInfo)(nil)

func (info testPackageInfo) Name() string    { return info["Package"] }
func (info testPackageInfo) Version() string { return info["Version"] }
func (info testPackageInfo) Arch() string    { return info["Architecture"] }
func (info testPackageInfo) SHA256() string  { return info["SHA256"] }

func (s testPackageInfo) Get(key string) (value string) {
	if s != nil {
		value = s[key]
	}
	return
}

type testArchive struct {
	options archive.Options
	pkgs    map[string]testPackage
}

func (a *testArchive) Options() *archive.Options {
	return &a.options
}

func (a *testArchive) Fetch(pkg string) (io.ReadCloser, error) {
	if data, ok := a.pkgs[pkg]; ok {
		return io.NopCloser(bytes.NewBuffer(data.content)), nil
	}
	return nil, fmt.Errorf("attempted to open %q package", pkg)
}

func (a *testArchive) Exists(pkg string) bool {
	_, ok := a.pkgs[pkg]
	return ok
}

func (a *testArchive) Info(pkg string) archive.PackageInfo {
	var info map[string]string
	if pkgData, ok := a.pkgs[pkg]; ok {
		if info = pkgData.info; info == nil {
			info = map[string]string{
				"Version": "1.0",
			}
		}
	}
	return testPackageInfo(info)
}

func (s *S) TestRun(c *C) {
	for _, test := range slicerTests {
		c.Logf("Summary: %s", test.summary)

		if _, ok := test.release["chisel.yaml"]; !ok {
			test.release["chisel.yaml"] = string(defaultChiselYaml)
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

		pkgs := map[string]testPackage{
			"base-files": testPackage{content: testutil.PackageData["base-files"]},
		}
		for name, entries := range packageEntries {
			deb, err := testutil.MakeDeb(entries)
			c.Assert(err, IsNil)
			pkgs[name] = testPackage{content: deb}
		}
		archives := map[string]archive.Archive{}
		for name, setupArchive := range release.Archives {
			var archivePkgs map[string]testPackage
			if test.pkgs != nil {
				archivePkgs = test.pkgs[name]
			}
			if archivePkgs == nil {
				archivePkgs = pkgs
			}
			archive := &testArchive{
				options: archive.Options{
					Label:      setupArchive.Name,
					Version:    setupArchive.Version,
					Suites:     setupArchive.Suites,
					Components: setupArchive.Components,
					Arch:       test.arch,
				},
				pkgs: archivePkgs,
			}
			archives[name] = archive
		}

		var obtainedDB = &fakeDB{}
		var expectedDB = &fakeDB{}

		targetDir := c.MkDir()
		options := slicer.RunOptions{
			Selection: selection,
			Archives:  archives,
			TargetDir: targetDir,
			AddToDB:   obtainedDB.add,
		}
		if test.hackopt != nil {
			test.hackopt(c, &options)
		}
		err = slicer.Run(&options)
		if test.error == "" {
			c.Assert(err, IsNil)
		} else {
			c.Assert(err, ErrorMatches, test.error)
			continue
		}

		if test.result != nil {
			result := make(map[string]string, len(copyrightEntries)+len(test.result))
			if test.pkgs == nil {
				// This was added in order to not specify copyright entries for each
				// existing test. These tests use only the base-files embedded
				// package. Custom packages may not include copyright entries
				// though. So if a test defines any custom packages, it must include
				// copyright entries explicitly in the results.
				for k, v := range copyrightEntries {
					result[k] = v
				}
			}
			for k, v := range test.result {
				result[k] = v
			}
			c.Assert(testutil.TreeDump(targetDir), DeepEquals, result)
		}

		//c.Log(obtainedDB.dump())
		for _, v := range test.db {
			err := expectedDB.add(v)
			c.Assert(err, IsNil)
		}
		c.Assert(obtainedDB.values(), DeepEquals, expectedDB.values())
	}
}
