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
	Name    string `json:"name"`
	Version string `json:"version"`
	Digest  string `json:"sha256"`
	Arch    string `json:"arch"`
}

type Slice struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type Path struct {
	Kind      string   `json:"kind"`
	Path      string   `json:"path"`
	Mode      string   `json:"mode"`
	Slices    []string `json:"slices"`
	Hash      string   `json:"sha256,omitempty"`
	FinalHash string   `json:"final_sha256,omitempty"`
	Size      uint64   `json:"size,omitempty"`
	Link      string   `json:"link,omitempty"`
}

type Content struct {
	Kind  string `json:"kind"`
	Slice string `json:"slice"`
	Path  string `json:"path"`
}

type Manifest struct {
	db *jsonwall.DB
}

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
	jsonwallDB, err := jsonwall.ReadDB(r)
	if err != nil {
		return nil, err
	}

	manifest = &Manifest{db: jsonwallDB}
	err = validate(manifest)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

func (manifest *Manifest) IteratePath(prefix string, f func(Path) error) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot read manifest: %s", err)
		}
	}()

	iter, err := manifest.db.IteratePrefix(map[string]string{"kind": "path", "path": prefix})
	if err != nil {
		return err
	}
	for iter.Next() {
		var path Path
		err := iter.Get(&path)
		if err != nil {
			return err
		}
		err = f(path)
		if err != nil {
			return err
		}
	}
	return nil
}

func (manifest *Manifest) IteratePkgs(f func(Package) error) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot read manifest: %s", err)
		}
	}()

	iter, err := manifest.db.Iterate(map[string]string{"kind": "package"})
	if err != nil {
		return err
	}
	for iter.Next() {
		var pkg Package
		err := iter.Get(&pkg)
		if err != nil {
			return err
		}
		err = f(pkg)
		if err != nil {
			return err
		}
	}
	return nil
}

func (manifest *Manifest) IterateSlices(pkgName string, f func(Slice) error) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot read manifest: %s", err)
		}
	}()

	iter, err := manifest.db.IteratePrefix(map[string]string{"kind": "slice", "name": pkgName})
	if err != nil {
		return err
	}
	for iter.Next() {
		var slice Slice
		err := iter.Get(&slice)
		if err != nil {
			return err
		}
		err = f(slice)
		if err != nil {
			return err
		}
	}
	return nil
}

func validate(manifest *Manifest) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf(`invalid manifest: %s`, err)
		}
	}()

	pkgExist := map[string]bool{}
	iter, err := manifest.db.Iterate(map[string]string{"kind": "package"})
	if err != nil {
		return err
	}
	for iter.Next() {
		var pkg Package
		err := iter.Get(&pkg)
		if err != nil {
			return err
		}
		if pkg.Kind != "package" {
			return fmt.Errorf(`in packages expected kind "package", got %q`, pkg.Kind)
		}
		pkgExist[pkg.Name] = true
	}

	sliceExist := map[string]bool{}
	iter, err = manifest.db.Iterate(map[string]string{"kind": "slice"})
	if err != nil {
		return err
	}
	for iter.Next() {
		var slice Slice
		err := iter.Get(&slice)
		if err != nil {
			return err
		}
		sk, err := setup.ParseSliceKey(slice.Name)
		if err != nil {
			return err
		}
		if !pkgExist[sk.Package] {
			return fmt.Errorf(`package %q not found in packages`, sk.Package)
		}
		sliceExist[slice.Name] = true
	}

	pathToSlices := map[string][]string{}
	iter, err = manifest.db.Iterate(map[string]string{"kind": "content"})
	if err != nil {
		return err
	}
	for iter.Next() {
		var content Content
		err := iter.Get(&content)
		if err != nil {
			return err
		}
		if content.Kind != "content" {
			return fmt.Errorf(`in contents expected kind "content", got "%s"`, content.Kind)
		}
		if !sliceExist[content.Slice] {
			return fmt.Errorf(`slice %s not found in slices`, content.Slice)
		}
		pathToSlices[content.Path] = append(pathToSlices[content.Path], content.Slice)
	}

	iter, err = manifest.db.Iterate(map[string]string{"kind": "path"})
	if err != nil {
		return err
	}
	for iter.Next() {
		var path Path
		err := iter.Get(&path)
		if err != nil {
			return err
		}
		if path.Kind != "path" {
			return fmt.Errorf(`in paths expected kind "path", got "%s"`, path.Kind)
		}
		if pathSlices, ok := pathToSlices[path.Path]; !ok {
			return fmt.Errorf(`path %s has no matching entry in contents`, path.Path)
		} else if !slices.Equal(pathSlices, path.Slices) {
			return fmt.Errorf(`path %s and content have diverging slices: %q != %q`, path.Path, path.Slices, pathSlices)
		}
		delete(pathToSlices, path.Path)
	}

	if len(pathToSlices) > 0 {
		for path := range pathToSlices {
			return fmt.Errorf(`content path %s has no matching entry in paths`, path)
		}
	}
	return nil
}
