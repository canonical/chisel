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
)

var shortCheckReleaseArchivesHelp = "Check the release's archives"

var longCheckReleaseArchivesHelp = `
The check-release-archives command downloads all the packages for a given
release to check that there are no issues which are not handled in the slice
definition files (SDFs).

Types of issues:
- "parent-directory-conflict". For parent directories which are not listed
explicitly in the SDFs, Chisel will try to preserve permissions by using the
mode from the package's tarball. If several packages have different permissions
for the same directory, that can lead to a conflict.
`

var checkReleaseArchivesDescs = map[string]string{
	"release": "Chisel release name or directory (e.g. ubuntu-22.04)",
	"arch":    "Package architecture",
}

type cmdDebugCheckReleaseArchives struct {
	Release string `long:"release" value-name:"<branch|dir>"`
	Arch    string `long:"arch" value-name:"<arch>"`
}

var archiveOpen = archive.Open

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
		})
		if err == archive.ErrCredentialsNotFound {
			logf("Archive %q ignored: credentials not found\n", archiveName)
			continue
		} else if err != nil {
			return err
		}
		archives[archiveName] = openArchive
	}

	type pkgArchive struct {
		Pkg     string `yaml:"package"`
		Archive string `yaml:"archive"`
	}

	type pathContent struct {
		Mode        yamlMode     `yaml:"mode"`
		Link        string       `yaml:"link"`
		PkgArchives []pkgArchive `yaml:"packages,flow"`
	}

	var orderedPkgs []string
	for packageName := range release.Packages {
		orderedPkgs = append(orderedPkgs, packageName)
	}
	slices.Sort(orderedPkgs)

	pathContents := map[string][]pathContent{}
	for archiveName, archive := range archives {
		logf("Processing archive %s...", archiveName)
		for _, pkgName := range orderedPkgs {
			if !archive.Exists(pkgName) {
				continue
			}
			pkgReader, _, err := archive.Fetch(pkgName)
			if err != nil {
				return err
			}
			dataReader, err := deb.DataReader(pkgReader)
			if err != nil {
				return err
			}
			tarReader := tar.NewReader(dataReader)
			for {
				tarHeader, err := tarReader.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}

				path, ok := sanitizeTarPath(tarHeader.Name)
				if !ok {
					continue
				}

				// Make paths uniform: while directories always end in '/',
				// symlinks don't.
				path = strings.TrimSuffix(path, "/")

				contents := pathContents[path]
				found := false
				// We look for a previous group that has the same entry in
				// terms of mode, link, etc. and we add the package to the
				// group. If there is none, we create a new one.
				for i, content := range contents {
					link := tarHeader.Linkname != "" || content.Link != ""
					if (link && tarHeader.Linkname == content.Link) ||
						(!link && tarHeader.Mode == int64(content.Mode)) {
						content.PkgArchives = append(content.PkgArchives, pkgArchive{pkgName, archiveName})
						contents[i] = content
						found = true
						break
					}
				}
				if !found {
					pathContents[path] = append(pathContents[path], pathContent{
						Mode:        yamlMode(tarHeader.Mode),
						Link:        tarHeader.Linkname,
						PkgArchives: []pkgArchive{{pkgName, archiveName}},
					})
				}
			}
		}
	}

	var issues []any
	type contentConflict struct {
		Issue    string        `yaml:"issue"`
		Path     string        `yaml:"path"`
		Contents []pathContent `yaml:"extracted-from"`
	}
	var sortedPaths []string
	for path := range pathContents {
		sortedPaths = append(sortedPaths, path)
	}
	slices.Sort(sortedPaths)
	for _, dir := range sortedPaths {
		contents := pathContents[dir]
		if len(contents) > 1 {
			issues = append(issues, contentConflict{
				Issue:    "content-conflict",
				Path:     dir,
				Contents: contents,
			})
		}
	}

	if len(issues) > 0 {
		yb, err := yaml.Marshal(issues)
		if err != nil {
			return fmt.Errorf("internal error: cannot marshal issue list: %s", err)
		}
		fmt.Fprintf(Stdout, "%s", string(yb))
		return errors.New("issues found in the release archives")
	}

	return nil
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

func init() {
	addDebugCommand("check-release-archives", shortCheckReleaseArchivesHelp, longCheckReleaseArchivesHelp, func() flags.Commander { return &cmdDebugCheckReleaseArchives{} }, checkReleaseArchivesDescs, nil)
}
