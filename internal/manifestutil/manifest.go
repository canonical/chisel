package manifestutil

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/pkg/jsonwall"
	"github.com/canonical/chisel/pkg/manifest"
)

const DefaultFilename = "manifest.wall"

// FindPaths finds the paths marked with "generate:manifest" and
// returns a map from the manifest path to all the slices that declare it.
func FindPaths(slices []*setup.Slice) map[string][]*setup.Slice {
	manifestSlices := make(map[string][]*setup.Slice)
	for _, slice := range slices {
		for path, info := range slice.Contents {
			if info.Generate == setup.GenerateManifest {
				dir := strings.TrimSuffix(path, "**")
				path = filepath.Join(dir, DefaultFilename)
				manifestSlices[path] = append(manifestSlices[path], slice)
			}
		}
	}
	return manifestSlices
}

type WriteOptions struct {
	PackageInfo []*archive.PackageInfo
	Selection   []*setup.Slice
	Report      *Report
}

func Write(options *WriteOptions, writer io.Writer) error {
	dbw := jsonwall.NewDBWriter(&jsonwall.DBWriterOptions{
		Schema: manifest.Schema,
	})

	err := fastValidate(options)
	if err != nil {
		return err
	}

	err = manifestAddPackages(dbw, options.PackageInfo)
	if err != nil {
		return err
	}

	err = manifestAddSlices(dbw, options.Selection)
	if err != nil {
		return err
	}

	err = manifestAddReport(dbw, options.Report)
	if err != nil {
		return err
	}

	_, err = dbw.WriteTo(writer)
	return err
}

func manifestAddPackages(dbw *jsonwall.DBWriter, infos []*archive.PackageInfo) error {
	for _, info := range infos {
		err := dbw.Add(&manifest.Package{
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
		err := dbw.Add(&manifest.Slice{
			Kind: "slice",
			Name: slice.String(),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func manifestAddReport(dbw *jsonwall.DBWriter, report *Report) error {
	for _, entry := range report.Entries {
		sliceNames := []string{}
		for slice := range entry.Slices {
			err := dbw.Add(&manifest.Content{
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
		err := dbw.Add(&manifest.Path{
			Kind:        "path",
			Path:        entry.Path,
			Mode:        fmt.Sprintf("0%o", unixPerm(entry.Mode)),
			Slices:      sliceNames,
			SHA256:      entry.SHA256,
			FinalSHA256: entry.FinalSHA256,
			Size:        uint64(entry.Size),
			Link:        entry.Link,
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

// fastValidate validates the data to be written into the manifest.
// This is validating internal structures which are supposed to be correct unless there is
// a bug. As such, only assertions that can be done quickly are performed here, instead
// of it being a comprehensive validation of all the structures.
func fastValidate(options *WriteOptions) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("internal error: invalid manifest: %s", err)
		}
	}()
	pkgExist := map[string]bool{}
	for _, pkg := range options.PackageInfo {
		err := validatePackage(pkg)
		if err != nil {
			return err
		}
		pkgExist[pkg.Name] = true
	}
	sliceExist := map[string]bool{}
	for _, slice := range options.Selection {
		if _, ok := pkgExist[slice.Package]; !ok {
			return fmt.Errorf("slice %s refers to missing package %q", slice.String(), slice.Package)
		}
		sliceExist[slice.String()] = true
	}
	for _, entry := range options.Report.Entries {
		err := validateReportEntry(&entry)
		if err != nil {
			return err
		}
		for slice := range entry.Slices {
			if _, ok := sliceExist[slice.String()]; !ok {
				return fmt.Errorf("path %q refers to missing slice %s", entry.Path, slice.String())
			}
		}
	}
	return nil
}

func validateReportEntry(entry *ReportEntry) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("path %q has invalid options: %s", entry.Path, err)
		}
	}()

	switch entry.Mode & fs.ModeType {
	case 0:
		// Regular file.
		if entry.Link != "" {
			return fmt.Errorf("link set for regular file")
		}
	case fs.ModeDir:
		if entry.Link != "" {
			return fmt.Errorf("link set for directory")
		}
		if entry.SHA256 != "" {
			return fmt.Errorf("sha256 set for directory")
		}
		if entry.FinalSHA256 != "" {
			return fmt.Errorf("final_sha256 set for directory")
		}
		if entry.Size != 0 {
			return fmt.Errorf("size set for directory")
		}
	case fs.ModeSymlink:
		if entry.Link == "" {
			return fmt.Errorf("link not set for symlink")
		}
		if entry.SHA256 != "" {
			return fmt.Errorf("sha256 set for symlink")
		}
		if entry.FinalSHA256 != "" {
			return fmt.Errorf("final_sha256 set for symlink")
		}
		if entry.Size != 0 {
			return fmt.Errorf("size set for symlink")
		}
	default:
		return fmt.Errorf("unsupported file type: %s", entry.Path)
	}

	if len(entry.Slices) == 0 {
		return fmt.Errorf("slices is empty")
	}

	return nil
}

func validatePackage(pkg *archive.PackageInfo) (err error) {
	if pkg.Name == "" {
		return fmt.Errorf("package name not set")
	}
	if pkg.Arch == "" {
		return fmt.Errorf("package %q missing arch", pkg.Name)
	}
	if pkg.SHA256 == "" {
		return fmt.Errorf("package %q missing sha256", pkg.Name)
	}
	if pkg.Version == "" {
		return fmt.Errorf("package %q missing version", pkg.Name)
	}
	return nil
}
