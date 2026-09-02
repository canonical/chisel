// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/canonical/chisel/public/jsonwall"
)

const Schema = "1.0"

// Package describes a package installed in the target filesystem.
type Package struct {
	Kind    string
	Name    string
	Version string
	Digest  string
	// DigestKind is the algorithm used to compute Digest, and is empty when
	// no digest is recorded.
	DigestKind string
	Arch       string
}

// packageJSON is the JSON encoding of a Package, with the digest recorded
// under the field named after its kind. At most one of the digest fields may
// be set.
type packageJSON struct {
	Kind    string `json:"kind"`
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
	SHA384  string `json:"sha384,omitempty"`
	SHA512  string `json:"sha512,omitempty"`
	Arch    string `json:"arch,omitempty"`
}

func (p *Package) MarshalJSON() ([]byte, error) {
	pj := packageJSON{
		Kind:    p.Kind,
		Name:    p.Name,
		Version: p.Version,
		Arch:    p.Arch,
	}
	switch p.DigestKind {
	case "":
		// No digest recorded.
	case "sha256":
		pj.SHA256 = p.Digest
	case "sha384":
		pj.SHA384 = p.Digest
	case "sha512":
		pj.SHA512 = p.Digest
	default:
		return nil, fmt.Errorf("cannot marshal package %q: unsupported digest kind %q", p.Name, p.DigestKind)
	}
	return json.Marshal(pj)
}

func (p *Package) UnmarshalJSON(data []byte) error {
	var pj packageJSON
	err := json.Unmarshal(data, &pj)
	if err != nil {
		return err
	}
	digest, kind, err := pj.digest()
	if err != nil {
		return err
	}
	*p = Package{
		Kind:       pj.Kind,
		Name:       pj.Name,
		Version:    pj.Version,
		Digest:     digest,
		DigestKind: kind,
		Arch:       pj.Arch,
	}
	return nil
}

// digest returns the package digest and its kind, as recorded in the wire
// representation. At most one digest field may be set.
func (pj *packageJSON) digest() (digest, kind string, err error) {
	set := 0
	for _, entry := range []struct {
		kind   string
		digest string
	}{
		{"sha256", pj.SHA256},
		{"sha384", pj.SHA384},
		{"sha512", pj.SHA512},
	} {
		if entry.digest != "" {
			set++
			digest, kind = entry.digest, entry.kind
		}
	}
	if set > 1 {
		return "", "", fmt.Errorf("package %q has multiple digests recorded", pj.Name)
	}
	return digest, kind, nil
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
