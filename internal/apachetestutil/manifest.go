// SPDX-License-Identifier: Apache-2.0

package apachetestutil

import (
	"gopkg.in/check.v1"

	"github.com/canonical/chisel/public/manifest"
)

type ManifestContents struct {
	Paths    []*manifest.Path
	Packages []*manifest.Package
	Slices   []*manifest.Slice
	Contents []*manifest.Content
}

func DumpManifestContents(c *check.C, mfest *manifest.Manifest) *ManifestContents {
	var slices []*manifest.Slice
	err := mfest.IterateSlices("", func(slice *manifest.Slice) error {
		slices = append(slices, slice)
		return nil
	})
	c.Assert(err, check.IsNil)

	var pkgs []*manifest.Package
	err = mfest.IteratePackages(func(pkg *manifest.Package) error {
		pkgs = append(pkgs, pkg)
		return nil
	})
	c.Assert(err, check.IsNil)

	var paths []*manifest.Path
	err = mfest.IteratePaths("", func(path *manifest.Path) error {
		paths = append(paths, path)
		return nil
	})
	c.Assert(err, check.IsNil)

	var contents []*manifest.Content
	err = mfest.IterateContents("", func(content *manifest.Content) error {
		contents = append(contents, content)
		return nil
	})
	c.Assert(err, check.IsNil)

	mc := ManifestContents{
		Paths:    paths,
		Packages: pkgs,
		Slices:   slices,
		Contents: contents,
	}
	return &mc
}
