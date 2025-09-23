package main

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/jessevdk/go-flags"
	"gopkg.in/yaml.v3"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/cache"
	"github.com/canonical/chisel/internal/deb"
	"github.com/canonical/chisel/internal/setup"
)

var shortCheckReleaseArchivesHelp = "Check the release's archives"

var longCheckReleaseArchivesHelp = `
The check-release-archives command downloads all the packages for a given
release to check that there are no issues which are not handled in the slice
definition files (SDFs).

Types of issues:
- "path-conflict". When multiple packages provide different content for the
same path. For example, for parent directories which are not listed explicitly
in the SDFs, Chisel will try to preserve permissions by using the mode from the
package's tarball. If several packages have different permissions for the same
directory, that could lead to a conflict.
`

var checkReleaseArchivesDescs = map[string]string{
	"release": "Chisel release name or directory (e.g. ubuntu-22.04)",
	"arch":    "Package architecture",
}

type cmdDebugCheckReleaseArchives struct {
	Release string `long:"release" value-name:"<branch|dir>"`
	Arch    string `long:"arch" value-name:"<arch>"`
}

func init() {
	addDebugCommand("check-release-archives", shortCheckReleaseArchivesHelp, longCheckReleaseArchivesHelp, func() flags.Commander { return &cmdDebugCheckReleaseArchives{} }, checkReleaseArchivesDescs, nil)
}

var archiveOpen = archive.Open

type pathObservation struct {
	Archive  string   `yaml:"archive"`
	Packages []string `yaml:"packages,flow"`
	Kind     string   `yaml:"kind"`
	Mode     yamlMode `yaml:"mode,omitempty"`
	Link     string   `yaml:"link,omitempty"`
}

func (cmd *cmdDebugCheckReleaseArchives) Execute(args []string) error {
	release, err := obtainRelease(cmd.Release)
	if err != nil {
		return err
	}

	archives := make(map[string]archive.Archive)
	for archiveName, archiveInfo := range release.Archives {
		openArchive, err := archiveOpen(&archive.Options{
			Label:      archiveName,
			Version:    archiveInfo.Version,
			Arch:       cmd.Arch,
			Suites:     archiveInfo.Suites,
			Components: archiveInfo.Components,
			Pro:        archiveInfo.Pro,
			CacheDir:   cache.DefaultDir("chisel"),
			PubKeys:    archiveInfo.PubKeys,
			Maintained: archiveInfo.Maintained,
			OldRelease: archiveInfo.OldRelease,
		})
		if err == archive.ErrCredentialsNotFound {
			logf("Archive %q ignored: credentials not found\n", archiveName)
			continue
		} else if err != nil {
			return err
		}
		archives[archiveName] = openArchive
	}

	pathObs, err := computePathObservations(release, archives)
	if err != nil {
		return err
	}

	var issues []any
	type pathConflict struct {
		Issue        string            `yaml:"issue"`
		Path         string            `yaml:"path"`
		Observations []pathObservation `yaml:"observations"`
	}
	var sortedPaths []string
	for path := range pathObs {
		sortedPaths = append(sortedPaths, path)
	}
	slices.Sort(sortedPaths)
	for _, path := range sortedPaths {
		observations := pathObs[path]
		if hasPathConflict(observations) {
			issues = append(issues, pathConflict{
				// At this time, there is only one possible type of conflict,
				// we do not need to check.
				Issue:        "path-conflict",
				Path:         path,
				Observations: observations,
			})
		}
	}

	if len(issues) > 0 {
		err := yaml.NewEncoder(Stdout).Encode(issues)
		if err != nil {
			return fmt.Errorf("internal error: cannot marshal issue list: %s", err)
		}
		return errors.New("issues found in the release archives")
	}

	return nil
}

func computePathObservations(release *setup.Release, archives map[string]archive.Archive) (map[string][]pathObservation, error) {
	var orderedPkgs []string
	for packageName := range release.Packages {
		orderedPkgs = append(orderedPkgs, packageName)
	}
	slices.Sort(orderedPkgs)
	var orderedArchives []string
	for archiveName := range archives {
		orderedArchives = append(orderedArchives, archiveName)
	}
	slices.Sort(orderedArchives)

	pathObs := map[string][]pathObservation{}
	for _, archiveName := range orderedArchives {
		archive := archives[archiveName]
		logf("Processing archive %s...", archiveName)
		for _, pkgName := range orderedPkgs {
			if !archive.Exists(pkgName) {
				continue
			}
			pkgReader, _, err := archive.Fetch(pkgName)
			if err != nil {
				return nil, err
			}
			dataReader, err := deb.DataReader(pkgReader)
			if err != nil {
				return nil, err
			}
			tarReader := tar.NewReader(dataReader)
			for {
				tarHeader, err := tarReader.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					return nil, err
				}

				path, ok := sanitizeTarPath(tarHeader.Name)
				if !ok {
					continue
				}
				if tarHeader.FileInfo().Mode().IsRegular() {
					continue
				}

				// Make paths uniform: while directories always end in '/',
				// symlinks don't.
				path = strings.TrimSuffix(path, "/")

				// We look for a previous observation that extracts the same
				// content in terms of mode, link, etc. and we add the package
				// to it. If there is none, we create a new one.
				observations := pathObs[path]
				index := slices.IndexFunc(observations, func(o pathObservation) bool {
					return o.Archive == archiveName &&
						tarHeader.Linkname == o.Link &&
						tarHeader.Mode == int64(o.Mode)
				})
				if index != -1 {
					observations[index].Packages = append(observations[index].Packages, pkgName)
				} else {
					kind := "symlink"
					if tarHeader.FileInfo().IsDir() {
						kind = "dir"
					}
					var mode yamlMode
					if kind == "dir" {
						mode = yamlMode(tarHeader.Mode)
					}
					pathObs[path] = append(pathObs[path], pathObservation{
						Kind:     kind,
						Mode:     mode,
						Link:     tarHeader.Linkname,
						Archive:  archiveName,
						Packages: []string{pkgName},
					})
				}
			}
		}
	}
	return pathObs, nil
}

func hasPathConflict(observations []pathObservation) bool {
	if len(observations) == 0 {
		return false
	}

	base := observations[0]
	for _, observation := range observations[1:] {
		if observation.Kind != base.Kind ||
			observation.Mode != base.Mode ||
			observation.Link != base.Link {
			return true
		}
	}
	return false
}

// sanitizeTarPath removes the leading "./" from the source path in the tarball,
// and verifies that the path is not empty.
func sanitizeTarPath(path string) (string, bool) {
	if len(path) < 3 || path[0] != '.' || path[1] != '/' {
		return "", false
	}
	return path[1:], true
}

type yamlMode int64

func (ym yamlMode) MarshalYAML() (interface{}, error) {
	// Workaround for marshalling integers in octal format.
	// Ref: https://github.com/go-yaml/yaml/issues/420.
	node := &yaml.Node{}
	err := node.Encode(uint(ym))
	if err != nil {
		return nil, err
	}
	node.Value = fmt.Sprintf("0%o", ym)
	return node, nil
}

var _ yaml.Marshaler = yamlMode(0)
