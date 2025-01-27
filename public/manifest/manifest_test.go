// SPDX-License-Identifier: Apache-2.0

package manifest_test

import (
	"os"
	"path"
	"slices"
	"strings"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/apachetestutil"
	"github.com/canonical/chisel/public/manifest"
)

var readManifestTests = []struct {
	summary string
	input   string
	mfest   *apachetestutil.ManifestContents
	error   string
}{{
	summary: "All types",
	input: `
		{"jsonwall":"1.0","schema":"1.0","count":13}
		{"kind":"content","slice":"pkg1_manifest","path":"/manifest/manifest.wall"}
		{"kind":"content","slice":"pkg1_myslice","path":"/dir/file"}
		{"kind":"content","slice":"pkg1_myslice","path":"/dir/file2"}
		{"kind":"content","slice":"pkg1_myslice","path":"/dir/foo/bar/"}
		{"kind":"content","slice":"pkg1_myslice","path":"/dir/hardlink"}
		{"kind":"content","slice":"pkg1_myslice","path":"/dir/link/file"}
		{"kind":"content","slice":"pkg2_myotherslice","path":"/dir/foo/bar/"}
		{"kind":"package","name":"pkg1","version":"v1","sha256":"hash1","arch":"arch1"}
		{"kind":"package","name":"pkg2","version":"v2","sha256":"hash2","arch":"arch2"}
		{"kind":"path","path":"/dir/file","mode":"0644","slices":["pkg1_myslice"],"sha256":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","final_sha256":"8067926c032c090867013d14fb0eb21ae858344f62ad07086fd32375845c91a6","size":21}
		{"kind":"path","path":"/dir/file2","mode":"0644","slices":["pkg1_myslice"],"sha256":"b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c","size":3,"inode":1}
		{"kind":"path","path":"/dir/foo/bar/","mode":"01777","slices":["pkg2_myotherslice","pkg1_myslice"]}
		{"kind":"path","path":"/dir/hardlink","mode":"0644","slices":["pkg1_myslice"],"sha256":"b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c","size":3,"inode":1}
		{"kind":"path","path":"/dir/link/file","mode":"0644","slices":["pkg1_myslice"],"link":"/dir/file"}
		{"kind":"path","path":"/manifest/manifest.wall","mode":"0644","slices":["pkg1_manifest"]}
		{"kind":"slice","name":"pkg1_manifest"}
		{"kind":"slice","name":"pkg1_myslice"}
		{"kind":"slice","name":"pkg2_myotherslice"}
	`,
	mfest: &apachetestutil.ManifestContents{
		Paths: []*manifest.Path{
			{Kind: "path", Path: "/dir/file", Mode: "0644", Slices: []string{"pkg1_myslice"}, SHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", FinalSHA256: "8067926c032c090867013d14fb0eb21ae858344f62ad07086fd32375845c91a6", Size: 0x15, Link: ""},
			{Kind: "path", Path: "/dir/file2", Mode: "0644", Slices: []string{"pkg1_myslice"}, SHA256: "b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c", Size: 0x03, Link: "", Inode: 0x01},
			{Kind: "path", Path: "/dir/foo/bar/", Mode: "01777", Slices: []string{"pkg2_myotherslice", "pkg1_myslice"}, SHA256: "", FinalSHA256: "", Size: 0x0, Link: ""},
			{Kind: "path", Path: "/dir/hardlink", Mode: "0644", Slices: []string{"pkg1_myslice"}, SHA256: "b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c", Size: 0x03, Link: "", Inode: 0x01},
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
			{Kind: "content", Slice: "pkg1_myslice", Path: "/dir/file2"},
			{Kind: "content", Slice: "pkg1_myslice", Path: "/dir/foo/bar/"},
			{Kind: "content", Slice: "pkg1_myslice", Path: "/dir/hardlink"},
			{Kind: "content", Slice: "pkg1_myslice", Path: "/dir/link/file"},
			{Kind: "content", Slice: "pkg2_myotherslice", Path: "/dir/foo/bar/"},
		},
	},
}, {
	summary: "Unknown schema",
	input: `
		{"jsonwall":"1.0","schema":"2.0","count":1}
		{"kind":"package","name":"pkg1","version":"v1","sha256":"hash1","arch":"arch1"}
	`,
	error: `cannot read manifest: unknown schema version "2.0"`,
}}

func (s *S) TestManifestRead(c *C) {
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
		if test.error != "" {
			c.Assert(err, ErrorMatches, test.error)
			continue
		}
		c.Assert(err, IsNil)
		if test.mfest != nil {
			c.Assert(apachetestutil.DumpManifestContents(c, mfest), DeepEquals, test.mfest)
		}
	}
}
