package manifest

import (
	"fmt"
	"os"
	"slices"

	"github.com/klauspost/compress/zstd"

	"github.com/canonical/chisel/internal/jsonwall"
	"github.com/canonical/chisel/internal/setup"
)

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
	Kind      string   `json:"kind"`
	Path      string   `json:"path,omitempty"`
	Mode      string   `json:"mode,omitempty"`
	Slices    []string `json:"slices,omitempty"`
	Hash      string   `json:"sha256,omitempty"`
	FinalHash string   `json:"final_sha256,omitempty"`
	Size      uint64   `json:"size,omitempty"`
	Link      string   `json:"link,omitempty"`
}

type Content struct {
	Kind  string `json:"kind"`
	Slice string `json:"slice,omitempty"`
	Path  string `json:"path,omitempty"`
}

type Manifest struct {
	db *jsonwall.DB
}

// Read loads a Manifest from a file without performing any validation. The file
// is assumed to be both valid jsonwall and a valid Manifest (see [Validate]).
func Read(absPath string) (manifest *Manifest, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot read manifest: %s", err)
		}
	}()

	file, err := os.OpenFile(absPath, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	r, err := zstd.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	db, err := jsonwall.ReadDB(r)
	if err != nil {
		return nil, err
	}
	manifest = &Manifest{db: db}
	return manifest, nil
}

func (manifest *Manifest) IteratePath(pathPrefix string, f func(Path) error) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot read manifest: %s", err)
		}
	}()

	return iteratePrefix(manifest, Path{Kind: "path", Path: pathPrefix}, f)
}

func (manifest *Manifest) IteratePackages(f func(Package) error) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot read manifest: %s", err)
		}
	}()

	return iteratePrefix(manifest, Package{Kind: "package"}, f)
}

func (manifest *Manifest) IterateSlices(pkgName string, f func(Slice) error) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot read manifest: %s", err)
		}
	}()

	return iteratePrefix(manifest, Slice{Kind: "slice", Name: pkgName}, f)
}

// Validate checks that the Manifest is valid. Note that to do that it has to
// load practically the whole manifest into memory and unmarshall all the
// entries.
func Validate(manifest *Manifest) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("invalid manifest: %s", err)
		}
	}()

	pkgExist := map[string]bool{}
	err = iteratePrefix(manifest, Package{Kind: "package"}, func(pkg Package) error {
		pkgExist[pkg.Name] = true
		return nil
	})
	if err != nil {
		return err
	}

	sliceExist := map[string]bool{}
	err = iteratePrefix(manifest, Slice{Kind: "slice"}, func(slice Slice) error {
		sk, err := setup.ParseSliceKey(slice.Name)
		if err != nil {
			return err
		}
		if !pkgExist[sk.Package] {
			return fmt.Errorf(`package %q not found in packages`, sk.Package)
		}
		sliceExist[slice.Name] = true
		return nil
	})
	if err != nil {
		return err
	}

	pathToSlices := map[string][]string{}
	err = iteratePrefix(manifest, Content{Kind: "content"}, func(content Content) error {
		if !sliceExist[content.Slice] {
			return fmt.Errorf(`slice %s not found in slices`, content.Slice)
		}
		pathToSlices[content.Path] = append(pathToSlices[content.Path], content.Slice)
		return nil
	})
	if err != nil {
		return err
	}

	err = iteratePrefix(manifest, Path{Kind: "path"}, func(path Path) error {
		if pathSlices, ok := pathToSlices[path.Path]; !ok {
			return fmt.Errorf(`path %s has no matching entry in contents`, path.Path)
		} else if !slices.Equal(pathSlices, path.Slices) {
			return fmt.Errorf(`path %s and content have diverging slices: %q != %q`, path.Path, path.Slices, pathSlices)
		}
		delete(pathToSlices, path.Path)
		return nil
	})
	if err != nil {
		return err
	}

	if len(pathToSlices) > 0 {
		for path := range pathToSlices {
			return fmt.Errorf(`content path %s has no matching entry in paths`, path)
		}
	}
	return nil
}

type prefixable interface {
	Path | Content | Package | Slice
}

func iteratePrefix[T prefixable](manifest *Manifest, prefix T, f func(T) error) error {
	iter, err := manifest.db.IteratePrefix(prefix)
	if err != nil {
		return err
	}
	for iter.Next() {
		var val T
		err := iter.Get(&val)
		if err != nil {
			return err
		}
		err = f(val)
		if err != nil {
			return err
		}
	}
	return nil
}
