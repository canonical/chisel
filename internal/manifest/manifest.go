package manifest

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/klauspost/compress/zstd"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/jsonwall"
	"github.com/canonical/chisel/internal/setup"
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
	err = manifest.IteratePackages(func(pkg *Package) error {
		pkgExist[pkg.Name] = true
		return nil
	})
	if err != nil {
		return err
	}

	sliceExist := map[string]bool{}
	err = manifest.IterateSlices("", func(slice *Slice) error {
		sk, err := setup.ParseSliceKey(slice.Name)
		if err != nil {
			return err
		}
		if !pkgExist[sk.Package] {
			return fmt.Errorf("package %q not found in packages", sk.Package)
		}
		sliceExist[slice.Name] = true
		return nil
	})
	if err != nil {
		return err
	}

	pathToSlices := map[string][]string{}
	err = manifest.IterateContents("", func(content *Content) error {
		if !sliceExist[content.Slice] {
			return fmt.Errorf("slice %s not found in slices", content.Slice)
		}
		if !slices.Contains(pathToSlices[content.Path], content.Slice) {
			pathToSlices[content.Path] = append(pathToSlices[content.Path], content.Slice)
		}
		return nil
	})
	if err != nil {
		return err
	}

	done := map[string]bool{}
	err = manifest.IteratePaths("", func(path *Path) error {
		pathSlices, ok := pathToSlices[path.Path]
		if !ok {
			return fmt.Errorf("path %s has no matching entry in contents", path.Path)
		}
		slices.Sort(pathSlices)
		slices.Sort(path.Slices)
		if !slices.Equal(pathSlices, path.Slices) {
			return fmt.Errorf("path %s and content have diverging slices: %q != %q", path.Path, path.Slices, pathSlices)
		}
		done[path.Path] = true
		return nil
	})
	if err != nil {
		return err
	}

	if len(done) != len(pathToSlices) {
		for path := range pathToSlices {
			return fmt.Errorf("content path %s has no matching entry in paths", path)
		}
	}
	return nil
}

// LocateManifestSlices finds the paths marked with "generate:manifest" and
// returns a map from the manifest path to all the slices that declare it.
func LocateManifestSlices(slices []*setup.Slice, manifestFileName string) map[string][]*setup.Slice {
	manifestSlices := make(map[string][]*setup.Slice)
	for _, slice := range slices {
		for path, info := range slice.Contents {
			if info.Generate == setup.GenerateManifest {
				dir := strings.TrimSuffix(path, "**")
				path = filepath.Join(dir, manifestFileName)
				manifestSlices[path] = append(manifestSlices[path], slice)
			}
		}
	}
	return manifestSlices
}

type GenerateManifestsOptions struct {
	PackageInfo []*archive.PackageInfo
	Selection   []*setup.Slice
	Report      *Report
	TargetDir   string
	Filename    string
	Mode        os.FileMode
}

func GenerateManifests(options *GenerateManifestsOptions) error {
	manifestSlices := LocateManifestSlices(options.Selection, options.Filename)
	if len(manifestSlices) == 0 {
		// Nothing to do.
		return nil
	}
	dbw := jsonwall.NewDBWriter(&jsonwall.DBWriterOptions{
		Schema: Schema,
	})

	err := manifestAddPackages(dbw, options.PackageInfo)
	if err != nil {
		return err
	}

	err = manifestAddSlices(dbw, options.Selection)
	if err != nil {
		return err
	}

	err = manifestAddReport(dbw, options.Report.Entries)
	if err != nil {
		return err
	}

	err = manifestAddManifestPaths(dbw, options.Mode, manifestSlices)
	if err != nil {
		return err
	}

	files := []io.Writer{}
	for relPath := range manifestSlices {
		logf("Generating manifest at %s...", relPath)
		absPath := filepath.Join(options.TargetDir, relPath)
		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			return err
		}
		file, err := os.OpenFile(absPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, options.Mode)
		if err != nil {
			return err
		}
		files = append(files, file)
		defer file.Close()
	}
	w, err := zstd.NewWriter(io.MultiWriter(files...))
	if err != nil {
		return err
	}
	defer w.Close()
	_, err = dbw.WriteTo(w)
	return err
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

func manifestAddPackages(dbw *jsonwall.DBWriter, infos []*archive.PackageInfo) error {
	for _, info := range infos {
		err := dbw.Add(&Package{
			Kind:    "package",
			Name:    info.Name,
			Version: info.Version,
			Digest:  info.SHA256,
			Arch:    info.Arch,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func manifestAddSlices(dbw *jsonwall.DBWriter, slices []*setup.Slice) error {
	for _, slice := range slices {
		err := dbw.Add(&Slice{
			Kind: "slice",
			Name: slice.String(),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func manifestAddReport(dbw *jsonwall.DBWriter, entries map[string]ReportEntry) error {
	for _, entry := range entries {
		sliceNames := []string{}
		for slice := range entry.Slices {
			err := dbw.Add(&Content{
				Kind:  "content",
				Slice: slice.String(),
				Path:  entry.Path,
			})
			if err != nil {
				return err
			}
			sliceNames = append(sliceNames, slice.String())
		}
		sort.Strings(sliceNames)
		err := dbw.Add(&Path{
			Kind:      "path",
			Path:      entry.Path,
			Mode:      fmt.Sprintf("0%o", unixPerm(entry.Mode)),
			Slices:    sliceNames,
			Hash:      entry.Hash,
			FinalHash: entry.FinalHash,
			Size:      uint64(entry.Size),
			Link:      entry.Link,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func manifestAddManifestPaths(dbw *jsonwall.DBWriter, manifestMode os.FileMode, manifestSlices map[string][]*setup.Slice) error {
	for path, slices := range manifestSlices {
		sliceNames := []string{}
		for _, slice := range slices {
			err := dbw.Add(&Content{
				Kind:  "content",
				Slice: slice.String(),
				Path:  path,
			})
			if err != nil {
				return err
			}
			sliceNames = append(sliceNames, slice.String())
		}
		sort.Strings(sliceNames)
		err := dbw.Add(&Path{
			Kind:   "path",
			Path:   path,
			Mode:   fmt.Sprintf("0%o", unixPerm(manifestMode)),
			Slices: sliceNames,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func unixPerm(mode fs.FileMode) (perm uint32) {
	perm = uint32(mode.Perm())
	if mode&fs.ModeSticky != 0 {
		perm |= 01000
	}
	return perm
}
