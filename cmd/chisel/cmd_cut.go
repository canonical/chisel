package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/klauspost/compress/zstd"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/cache"
	"github.com/canonical/chisel/internal/jsonwall"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/slicer"
)

var shortCutHelp = "Cut a tree with selected slices"
var longCutHelp = `
The cut command uses the provided selection of package slices
to create a new filesystem tree in the root location.
`

var cutDescs = map[string]string{
	"release": "Chisel release directory",
	"root":    "Root for generated content",
	"arch":    "Package architecture",
}

type cmdCut struct {
	Release string `long:"release" value-name:"<dir>"`
	RootDir string `long:"root" value-name:"<dir>" required:"yes"`
	Arch    string `long:"arch" value-name:"<arch>"`

	Positional struct {
		SliceRefs []string `positional-arg-name:"<slice names>" required:"yes"`
	} `positional-args:"yes"`
}

func init() {
	addCommand("cut", shortCutHelp, longCutHelp, func() flags.Commander { return &cmdCut{} }, cutDescs, nil)
}

func (cmd *cmdCut) Execute(args []string) error {
	if len(args) > 0 {
		return ErrExtraArgs
	}

	sliceKeys := make([]setup.SliceKey, len(cmd.Positional.SliceRefs))
	for i, sliceRef := range cmd.Positional.SliceRefs {
		sliceKey, err := setup.ParseSliceKey(sliceRef)
		if err != nil {
			return err
		}
		sliceKeys[i] = sliceKey
	}

	var release *setup.Release
	var err error
	if strings.Contains(cmd.Release, "/") {
		release, err = setup.ReadRelease(cmd.Release)
	} else {
		var label, version string
		if cmd.Release == "" {
			label, version, err = readReleaseInfo()
		} else {
			label, version, err = parseReleaseInfo(cmd.Release)
		}
		if err != nil {
			return err
		}
		release, err = setup.FetchRelease(&setup.FetchOptions{
			Label:   label,
			Version: version,
		})
	}
	if err != nil {
		return err
	}

	selection, err := setup.Select(release, sliceKeys)
	if err != nil {
		return err
	}

	archives, err := openArchives(release, cmd.Arch)
	if err != nil {
		return err
	}
	pkgArchives, err := selectPkgArchives(archives, selection)
	if err != nil {
		return err
	}

	report, err := slicer.Run(&slicer.RunOptions{
		Selection:   selection,
		PkgArchives: pkgArchives,
		TargetDir:   cmd.RootDir,
	})
	if err != nil {
		return err
	}

	manifestSlices := locateManifestSlices(selection.Slices)
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
			Slices:         selection.Slices,
			Report:         report,
		})
		if err != nil {
			return err
		}
		manifestPaths := []string{}
		for path := range manifestSlices {
			manifestPath := filepath.Join(cmd.RootDir, getManifestPath(path))
			manifestPaths = append(manifestPaths, manifestPath)
		}
		err = writeManifests(manifestWriter, manifestPaths)
		if err != nil {
			return err
		}
	}

	return nil
}

// openArchives opens the archives listed in the release for a particular
// architecture. It returns a map of archives indexed by archive name.
func openArchives(release *setup.Release, arch string) (map[string]archive.Archive, error) {
	archives := make(map[string]archive.Archive)
	for archiveName, archiveInfo := range release.Archives {
		archive, err := openArchive(&archive.Options{
			Label:      archiveName,
			Version:    archiveInfo.Version,
			Arch:       arch,
			Suites:     archiveInfo.Suites,
			Components: archiveInfo.Components,
			CacheDir:   cache.DefaultDir("chisel"),
			PubKeys:    archiveInfo.PubKeys,
		})
		if err != nil {
			return nil, err
		}
		archives[archiveName] = archive
	}
	return archives, nil
}

// selectePkgArchives selects the appropriate archive for each selected slice
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

// locateManifestSlices finds the paths marked with "generate:manifest" and
// returns a map from said path to all the slices that declare it.
func locateManifestSlices(slices []*setup.Slice) map[string][]*setup.Slice {
	manifestSlices := make(map[string][]*setup.Slice)
	for _, s := range slices {
		for path, info := range s.Contents {
			if info.Generate == setup.GenerateManifest {
				if manifestSlices[path] == nil {
					manifestSlices[path] = []*setup.Slice{}
				}
				manifestSlices[path] = append(manifestSlices[path], s)
			}
		}
	}
	return manifestSlices
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
			Kind:   "path",
			Path:   entry.Path,
			Mode:   fmt.Sprintf("0%o", unixPerm(entry.Mode)),
			Slices: sliceNames,
			Hash:   entry.Hash,
			Size:   uint64(entry.Size),
			Link:   entry.Link,
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

// writeManifests writes all added entries and generates the manifest file(s).
func writeManifests(writer *jsonwall.DBWriter, paths []string) (err error) {
	files := []io.Writer{}
	for _, path := range paths {
		if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		logf("Generating manifest at %s...", path)
		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, dbMode)
		if err != nil {
			return err
		}
		files = append(files, file)
		defer file.Close()
	}

	// Using a MultiWriter allows to compress the data only once and write the
	// compressed data to each path.
	w, err := zstd.NewWriter(io.MultiWriter(files...))
	if err != nil {
		return err
	}
	defer w.Close()

	_, err = writer.WriteTo(w)
	return err
}

func unixPerm(mode fs.FileMode) (perm uint32) {
	perm = uint32(mode.Perm())
	if mode&fs.ModeSticky != 0 {
		perm |= 01000
	}
	return perm
}

// TODO These need testing, and maybe moving into a common file.

var releaseExp = regexp.MustCompile(`^([a-z](?:-?[a-z0-9]){2,})-([0-9]+(?:\.?[0-9])+)$`)

func parseReleaseInfo(release string) (label, version string, err error) {
	match := releaseExp.FindStringSubmatch(release)
	if match == nil {
		return "", "", fmt.Errorf("invalid release reference: %q", release)
	}
	return match[1], match[2], nil
}

func readReleaseInfo() (label, version string, err error) {
	data, err := os.ReadFile("/etc/lsb-release")
	if err == nil {
		const labelPrefix = "DISTRIB_ID="
		const versionPrefix = "DISTRIB_RELEASE="
		for _, line := range strings.Split(string(data), "\n") {
			switch {
			case strings.HasPrefix(line, labelPrefix):
				label = strings.ToLower(line[len(labelPrefix):])
			case strings.HasPrefix(line, versionPrefix):
				version = line[len(versionPrefix):]
			}
			if label != "" && version != "" {
				return label, version, nil
			}
		}
	}
	return "", "", fmt.Errorf("cannot infer release via /etc/lsb-release, see the --release option")
}
