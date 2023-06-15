package db

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"strconv"
)

func decodeDigest(str string) (*[sha256.Size]byte, error) {
	if str == "" {
		return nil, nil
	}
	if len(str) != 2*sha256.Size {
		return nil, fmt.Errorf("length %d != %d", len(str), 2*sha256.Size)
	}
	sl, err := hex.DecodeString(str)
	if err != nil {
		return nil, err
	}
	var digest [sha256.Size]byte
	copy(digest[:], sl)
	return &digest, nil
}

// The *alias types exist to avoid recursive marshaling, see
// https://choly.ca/post/go-json-marshalling/.

// Package represents a package that was sliced
type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	// package digests in chisel are used only in hex encoding so there's
	// currently no need to store them as byte arrays
	SHA256 string `json:"sha256"`
	Arch   string `json:"arch"`
}

type packageAlias Package

type jsonPackage struct {
	Kind string `json:"kind"`
	packageAlias
}

func (t Package) MarshalJSON() ([]byte, error) {
	j := jsonPackage{"package", packageAlias(t)}
	return json.Marshal(j)
}

func (t *Package) UnmarshalJSON(data []byte) error {
	j := jsonPackage{}
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	if j.Kind != "package" {
		return fmt.Errorf(`invalid kind %q: must be "package"`, j.Kind)
	}
	*t = Package(j.packageAlias)
	return nil
}

// Slice represents a slice of a package that was installed
type Slice struct {
	Name string `json:"name"`
}

type sliceAlias Slice

type jsonSlice struct {
	Kind string `json:"kind"`
	sliceAlias
}

func (t Slice) MarshalJSON() ([]byte, error) {
	j := jsonSlice{"slice", sliceAlias(t)}
	return json.Marshal(j)
}

func (t *Slice) UnmarshalJSON(data []byte) error {
	j := jsonSlice{}
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	if j.Kind != "slice" {
		return fmt.Errorf(`invalid kind %q: must be "slice"`, j.Kind)
	}
	*t = Slice(j.sliceAlias)
	return nil
}

// Path represents a path that was sliced from a package.
//
// The filesystem object type can be determined from Path's fields:
//
//	a) If the SHA256 attribute is non-nil, this Path refers to a regular file,
//	   its Path attribute must not end with /, and its Link attribute must be
//	   empty.
//	c) Otherwise, if the Link attribute is not empty, this Path refers to a
//	   symbolic link.
//	c) Otherwise, this Path refers to a directory, and its Path attribute must
//	   end with /.
//
// If the Path refers to a regular file and its FinalSHA256 attribute is
// non-nil, the SHA256 attribute must have a different value, and this Path
// represents a regular file whose content in the package had digest equal to
// the SHA256 attribute, but its content has changed during the installation
// and the final digest of the content is equal to the FinalSHA256 attribute.
type Path struct {
	Path        string
	Mode        fs.FileMode
	Slices      []string
	SHA256      *[sha256.Size]byte
	FinalSHA256 *[sha256.Size]byte
	Size        int64
	Link        string
}

type jsonPath struct {
	Kind        string   `json:"kind"`
	Path        string   `json:"path"`
	Mode        string   `json:"mode"`
	Slices      []string `json:"slices"`
	SHA256      string   `json:"sha256,omitempty"`
	FinalSHA256 string   `json:"final_sha256,omitempty"`
	Size        *int64   `json:"size,omitempty"`
	Link        string   `json:"link,omitempty"`
}

func (t Path) MarshalJSON() ([]byte, error) {
	j := jsonPath{
		Kind:   "path",
		Path:   t.Path,
		Mode:   fmt.Sprintf("%#o", t.Mode),
		Slices: t.Slices,
		Link:   t.Link,
	}
	if t.Slices == nil {
		j.Slices = []string{}
	}
	if t.SHA256 != nil {
		j.SHA256 = fmt.Sprintf("%x", *t.SHA256)
		if t.FinalSHA256 != nil {
			j.FinalSHA256 = fmt.Sprintf("%x", *t.FinalSHA256)
		}
		j.Size = &t.Size
	}
	return json.Marshal(j)
}

func (t *Path) UnmarshalJSON(data []byte) error {
	j := jsonPath{}
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	if j.Kind != "path" {
		return fmt.Errorf(`invalid kind %q: must be "path"`, j.Kind)
	}
	mode, err := strconv.ParseUint(j.Mode, 8, 32)
	if err != nil {
		return fmt.Errorf("invalid mode %#v: %w", j.Mode, err)
	}
	digest, err := decodeDigest(j.SHA256)
	if err != nil {
		return fmt.Errorf("invalid sha256 %#v: %w", j.SHA256, err)
	}
	finalDigest, err := decodeDigest(j.FinalSHA256)
	if err != nil {
		return fmt.Errorf("invalid final_sha256 %#v: %w", j.SHA256, err)
	}
	t.Path = j.Path
	t.Mode = fs.FileMode(mode)
	t.Slices = j.Slices
	if t.Slices != nil && len(t.Slices) == 0 {
		t.Slices = nil
	}
	t.SHA256 = digest
	t.FinalSHA256 = finalDigest
	t.Size = 0
	if j.Size != nil {
		t.Size = *j.Size
	}
	t.Link = j.Link
	return nil
}

// Content represents an ownership of a path by a slice. There can be more than
// one slice owning a path.
type Content struct {
	Slice string `json:"slice"`
	Path  string `json:"path"`
}

type contentAlias Content

type jsonContent struct {
	Kind string `json:"kind"`
	contentAlias
}

func (t Content) MarshalJSON() ([]byte, error) {
	j := jsonContent{"content", contentAlias(t)}
	return json.Marshal(j)
}

func (t *Content) UnmarshalJSON(data []byte) error {
	j := jsonContent{}
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	if j.Kind != "content" {
		return fmt.Errorf(`invalid kind %q: must be "content"`, j.Kind)
	}
	*t = Content(j.contentAlias)
	return nil
}
