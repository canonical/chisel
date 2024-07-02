package manifest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/klauspost/compress/zstd"

	"github.com/canonical/chisel/internal/jsonwall"
	"github.com/canonical/chisel/internal/setup"
)

const Filename = "manifest.wall"
const Schema = "1.0"
const Mode fs.FileMode = 0644

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

// LocateManifestSlices finds the paths marked with "generate:manifest" and
// returns a map from the manifest path to all the slices that declare it.
func LocateManifestSlices(slices []*setup.Slice) (map[string][]*setup.Slice, error) {
	manifestSlices := make(map[string][]*setup.Slice)
	for _, s := range slices {
		for path, info := range s.Contents {
			if info.Generate == setup.GenerateManifest {
				dir, err := setup.GetGeneratePath(path)
				if err != nil {
					return nil, fmt.Errorf("internal error: %s", err)
				}
				path = filepath.Join(dir, Filename)
				manifestSlices[path] = append(manifestSlices[path], s)
			}
		}
	}
	return manifestSlices, nil
}

type Manifest struct {
	Paths    []Path
	Contents []Content
	Packages []Package
	Slices   []Slice
}

func Read(rootDir string, relPath string) (manifest *Manifest, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot read manifest: %s", err)
		}
	}()

	absPath := filepath.Join(rootDir, relPath)
	file, err := os.OpenFile(absPath, os.O_RDONLY, Mode)
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

	manifest = &Manifest{}
	iter, err := jsonwallDB.Iterate(map[string]string{"kind": "path"})
	if err != nil {
		return nil, err
	}
	for iter.Next() {
		var path Path
		err := iter.Get(&path)
		if err != nil {
			return nil, err
		}
		manifest.Paths = append(manifest.Paths, path)
	}
	iter, err = jsonwallDB.Iterate(map[string]string{"kind": "content"})
	if err != nil {
		return nil, err
	}
	for iter.Next() {
		var content Content
		err := iter.Get(&content)
		if err != nil {
			return nil, err
		}
		manifest.Contents = append(manifest.Contents, content)
	}
	iter, err = jsonwallDB.Iterate(map[string]string{"kind": "package"})
	if err != nil {
		return nil, err
	}
	for iter.Next() {
		var pkg Package
		err := iter.Get(&pkg)
		if err != nil {
			return nil, err
		}
		manifest.Packages = append(manifest.Packages, pkg)
	}
	iter, err = jsonwallDB.Iterate(map[string]string{"kind": "slice"})
	if err != nil {
		return nil, err
	}
	for iter.Next() {
		var slice Slice
		err := iter.Get(&slice)
		if err != nil {
			return nil, err
		}
		manifest.Slices = append(manifest.Slices, slice)
	}

	err = validate(manifest)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

func validate(manifest *Manifest) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf(`invalid manifest: %s`, err)
		}
	}()

	pkgExist := map[string]bool{}
	for _, pkg := range manifest.Packages {
		if pkg.Kind != "package" {
			return fmt.Errorf(`in packages expected kind "package", got %q`, pkg.Kind)
		}
		pkgExist[pkg.Name] = true
	}
	sliceExist := map[string]bool{}
	for _, slice := range manifest.Slices {
		if slice.Kind != "slice" {
			return fmt.Errorf(`in slices expected kind "slice", got %q`, slice.Kind)
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
	for _, content := range manifest.Contents {
		if content.Kind != "content" {
			return fmt.Errorf(`in contents expected kind "content", got "%s"`, content.Kind)
		}
		if !sliceExist[content.Slice] {
			return fmt.Errorf(`slice %s not found in slices`, content.Slice)
		}
		pathToSlices[content.Path] = append(pathToSlices[content.Path], content.Slice)
	}
	for _, path := range manifest.Paths {
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
