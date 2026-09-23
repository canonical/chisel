// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/canonical/chisel/public/jsonwall"
)

const Schema = "1.0"

type Package struct {
	Kind    string
	Name    string
	Version string
	// Digest holds the sha256 digest when present, and the sha512 digest
	// otherwise. It is empty when neither is recorded.
	Digest string
	// Digests holds the digests of the package, keyed by digest kind
	// (e.g. "sha256").
	Digests map[string]string
	Arch    string
}

type packageJSON struct {
	Kind    string `json:"kind"`
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
	SHA512  string `json:"sha512,omitempty"`
	SHA384  string `json:"sha384,omitempty"`
	Arch    string `json:"arch,omitempty"`
}

type PackageOptions struct {
	Name    string
	Version string
	Arch    string
	Digests map[string]string
}

func NewPackage(opts *PackageOptions) (*Package, error) {
	o, err := getValidOptions(opts)
	if err != nil {
		return nil, err
	}
	digest := o.Digests["sha256"]
	if digest == "" {
		digest = o.Digests["sha512"]
	}
	return &Package{
		Kind:    "package",
		Name:    o.Name,
		Version: o.Version,
		Digest:  digest,
		Digests: o.Digests,
		Arch:    o.Arch,
	}, nil
}

func getValidOptions(options *PackageOptions) (*PackageOptions, error) {
	optsCopy := *options
	o := &optsCopy
	digests := make(map[string]string, len(options.Digests))
	for kind, digest := range options.Digests {
		switch kind {
		case "sha256", "sha512", "sha384":
			digests[kind] = digest
		default:
			return nil, fmt.Errorf("cannot create package %q: unsupported digest kind %q", options.Name, kind)
		}
	}
	o.Digests = digests
	return o, nil
}

func (p *Package) MarshalJSON() ([]byte, error) {
	pj := packageJSON{
		Kind:    p.Kind,
		Name:    p.Name,
		Version: p.Version,
		Arch:    p.Arch,
	}
	for kind, digest := range p.Digests {
		switch kind {
		case "sha256":
			pj.SHA256 = digest
		case "sha512":
			pj.SHA512 = digest
		case "sha384":
			pj.SHA384 = digest
		default:
			return nil, fmt.Errorf("cannot marshal package %q: unsupported digest kind %q", p.Name, kind)
		}
	}
	return json.Marshal(pj)
}

func (p *Package) UnmarshalJSON(data []byte) error {
	var pj packageJSON
	err := json.Unmarshal(data, &pj)
	if err != nil {
		return err
	}
	digests := make(map[string]string)
	for kind, digest := range map[string]string{
		"sha256": pj.SHA256,
		"sha512": pj.SHA512,
		"sha384": pj.SHA384,
	} {
		if digest != "" {
			digests[kind] = digest
		}
	}
	pkg, err := NewPackage(&PackageOptions{
		Name:    pj.Name,
		Version: pj.Version,
		Arch:    pj.Arch,
		Digests: digests,
	})
	if err != nil {
		return err
	}
	*p = *pkg
	return nil
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
