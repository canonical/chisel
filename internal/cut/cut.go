package cut

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/klauspost/compress/zstd"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/jsonwall"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/slicer"
)

type RunOptions struct {
	Selection *setup.Selection
	Archives  map[string]archive.Archive
	TargetDir string
}

func Run(options *RunOptions) error {
	pkgArchives, err := selectPkgArchives(options.Archives, options.Selection)
	if err != nil {
		return err
	}

	report, err := slicer.Slice(&slicer.SliceOptions{
		Selection:   options.Selection,
		PkgArchives: pkgArchives,
		TargetDir:   options.TargetDir,
	})
	if err != nil {
		return err
	}

	manifestSlices := locateManifestSlices(options.Selection.Slices)
	if len(manifestSlices) > 0 {
		pkgInfo := []*archive.PackageInfo{}
		for pkg, archive := range pkgArchives {
			info, err := archive.Info(pkg)
			if err != nil {
				return err
			}
			pkgInfo = append(pkgInfo, info)
		}
		manifestWriter, err := generateManifest(&generateManifestOptions{
			ManifestSlices: manifestSlices,
			PackageInfo:    pkgInfo,
			Slices:         options.Selection.Slices,
			Report:         report,
		})
		if err != nil {
			return err
		}
		files := []io.Writer{}
		for generatePath := range manifestSlices {
			relPath := getManifestPath(generatePath)
			logf("Generating manifest at %s...", relPath)
			absPath := filepath.Join(options.TargetDir, relPath)
			if err = os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
				return err
			}
			file, err := os.OpenFile(absPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, dbMode)
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
		_, err = manifestWriter.WriteTo(w)
		if err != nil {
			return err
		}
	}

	return nil
}

// selectPkgArchives selects the appropriate archive for each selected slice
// package. It returns a map of archives indexed by package names.
func selectPkgArchives(archives map[string]archive.Archive, selection *setup.Selection) (map[string]archive.Archive, error) {
	pkgArchives := make(map[string]archive.Archive)
	for _, s := range selection.Slices {
		pkg := selection.Release.Packages[s.Package]
		if _, ok := pkgArchives[pkg.Name]; ok {
			continue
		}
		archive := archives[pkg.Archive]
		if archive == nil {
			return nil, fmt.Errorf("archive %q not defined", pkg.Archive)
		}
		if !archive.Exists(pkg.Name) {
			return nil, fmt.Errorf("slice package %q missing from archive", pkg.Name)
		}
		pkgArchives[pkg.Name] = archive
	}
	return pkgArchives, nil
}

func unixPerm(mode fs.FileMode) (perm uint32) {
	perm = uint32(mode.Perm())
	if mode&fs.ModeSticky != 0 {
		perm |= 01000
	}
	return perm
}

const dbMode fs.FileMode = 0644

type generateManifestOptions struct {
	// Map of slices indexed by paths which contain an entry tagged "generate: manifest".
	ManifestSlices map[string][]*setup.Slice
	PackageInfo    []*archive.PackageInfo
	Slices         []*setup.Slice
	Report         *slicer.Report
}

// generateManifest generates the Chisel manifest(s) at the specified paths. It
// returns the paths inside the rootfs where the manifest(s) are generated.
func generateManifest(opts *generateManifestOptions) (*jsonwall.DBWriter, error) {
	dbw := jsonwall.NewDBWriter(&jsonwall.DBWriterOptions{
		Schema: dbSchema,
	})

	// Add packages to the manifest.
	for _, info := range opts.PackageInfo {
		err := dbw.Add(&dbPackage{
			Kind:    "package",
			Name:    info.Name,
			Version: info.Version,
			Digest:  info.Hash,
			Arch:    info.Arch,
		})
		if err != nil {
			return nil, err
		}
	}
	// Add slices to the manifest.
	for _, s := range opts.Slices {
		err := dbw.Add(&dbSlice{
			Kind: "slice",
			Name: s.String(),
		})
		if err != nil {
			return nil, err
		}
	}
	// Add paths and contents to the manifest.
	for _, entry := range opts.Report.Entries {
		sliceNames := []string{}
		for s := range entry.Slices {
			err := dbw.Add(&dbContent{
				Kind:  "content",
				Slice: s.String(),
				Path:  entry.Path,
			})
			if err != nil {
				return nil, err
			}
			sliceNames = append(sliceNames, s.String())
		}
		sort.Strings(sliceNames)
		err := dbw.Add(&dbPath{
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
			return nil, err
		}
	}
	// Add the manifest path and content entries.
	for path, slices := range opts.ManifestSlices {
		fPath := getManifestPath(path)
		sliceNames := []string{}
		for _, s := range slices {
			err := dbw.Add(&dbContent{
				Kind:  "content",
				Slice: s.String(),
				Path:  fPath,
			})
			if err != nil {
				return nil, err
			}
			sliceNames = append(sliceNames, s.String())
		}
		sort.Strings(sliceNames)
		err := dbw.Add(&dbPath{
			Kind:   "path",
			Path:   fPath,
			Mode:   fmt.Sprintf("0%o", unixPerm(dbMode)),
			Slices: sliceNames,
		})
		if err != nil {
			return nil, err
		}
	}

	return dbw, nil
}
