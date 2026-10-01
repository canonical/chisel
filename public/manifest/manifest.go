// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"io"

	"github.com/canonical/chisel/public/jsonwall"
)

const Schema = "1.0"

type Package struct {
	Kind    string `json:"kind"`
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	Digest  string `json:"sha256,omitempty"`
	Arch    string `json:"arch,omitempty"`
}

type Slice struct {
	Kind string `json:"kind"`
	Name string `json:"name,omitempty"`
}

type Path struct {
	Kind        string   `json:"kind"`
	Path        string   `json:"path,omitempty"`
	Mode        string   `json:"mode,omitempty"`
	Slices      []string `json:"slices,omitempty"`
	SHA256      string   `json:"sha256,omitempty"`
	FinalSHA256 string   `json:"final_sha256,omitempty"`
	Size        uint64   `json:"size,omitempty"`
	Link        string   `json:"link,omitempty"`
	Inode       uint64   `json:"inode,omitempty"`
}

type Content struct {
	Kind  string `json:"kind"`
	Slice string `json:"slice,omitempty"`
	Path  string `json:"path,omitempty"`
}

type Manifest struct {
	db *jsonwall.DB
}

// Read loads a Manifest without performing any validation. The data is assumed
// to be both valid jsonwall and a valid Manifest (see Validate).
func Read(reader io.Reader) (manifest *Manifest, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot read manifest: %s", err)
		}
	}()

	db, err := jsonwall.ReadDB(reader)
	if err != nil {
		return nil, err
	}
	mfestSchema := db.Schema()
	if mfestSchema != Schema {
		return nil, fmt.Errorf("unknown schema version %q", mfestSchema)
	}

	manifest = &Manifest{db: db}
	return manifest, nil
}

func (manifest *Manifest) IteratePaths(pathPrefix string, onMatch func(*Path) error) (err error) {
	return iteratePrefix(manifest, &Path{Kind: "path", Path: pathPrefix}, onMatch)
}

func (manifest *Manifest) IteratePackages(onMatch func(*Package) error) (err error) {
	return iteratePrefix(manifest, &Package{Kind: "package"}, onMatch)
}

func (manifest *Manifest) IterateSlices(pkgName string, onMatch func(*Slice) error) (err error) {
	return iteratePrefix(manifest, &Slice{Kind: "slice", Name: pkgName}, onMatch)
}

func (manifest *Manifest) IterateContents(slice string, onMatch func(*Content) error) (err error) {
	return iteratePrefix(manifest, &Content{Kind: "content", Slice: slice}, onMatch)
}

type prefixable interface {
	Path | Content | Package | Slice
}

func iteratePrefix[T prefixable](manifest *Manifest, prefix *T, onMatch func(*T) error) error {
	iter, err := manifest.db.IteratePrefix(prefix)
	if err != nil {
		return err
	}
	for iter.Next() {
		var val T
		err := iter.Get(&val)
		if err != nil {
			return fmt.Errorf("cannot read manifest: %s", err)
		}
		err = onMatch(&val)
		if err != nil {
			return err
		}
	}
	return nil
}
