package source

import (
	"fmt"
	"io"
	"slices"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/setup"
)

// Source is a resolved package source, abstracting over archives and stores.
type Source interface {
	Arch() string
	Fetch() (io.ReadSeekCloser, *archive.PackageInfo, error)
}

// archiveSource adapts an archive.Archive to the Source interface for a
// specific package.
type archiveSource struct {
	archive archive.Archive
	name    string
}

func (a *archiveSource) Arch() string {
	return a.archive.Options().Arch
}

func (a *archiveSource) Fetch() (io.ReadSeekCloser, *archive.PackageInfo, error) {
	return a.archive.Fetch(a.name)
}

// storeSource adapts a store to the Source interface for a specific package.
type storeSource struct {
	arch  string
	name  string
	store string
}

func (s *storeSource) Arch() string {
	return s.arch
}

func (s *storeSource) Fetch() (io.ReadSeekCloser, *archive.PackageInfo, error) {
	return nil, nil, fmt.Errorf("cannot fetch package %q from store %q: not implemented", s.name, s.store)
}

// Resolve determines the source for each package in the selection.
// For archive packages it selects the highest priority archive containing the
// package unless a particular archive is pinned within the slice definition
// file. For store packages it records a store source. It returns a map of
// Source indexed by package names.
func Resolve(archives map[string]archive.Archive, selection *setup.Selection) (map[string]Source, error) {
	sortedArchives := make([]*setup.Archive, 0, len(selection.Release.Archives))
	for _, archive := range selection.Release.Archives {
		if archive.Priority < 0 {
			// Ignore negative priority archives unless a package specifically
			// asks for it with the "archive" field.
			continue
		}
		sortedArchives = append(sortedArchives, archive)
	}
	slices.SortFunc(sortedArchives, func(a, b *setup.Archive) int {
		return b.Priority - a.Priority
	})

	sources := make(map[string]Source)
	for _, s := range selection.Slices {
		if _, ok := sources[s.Package]; ok {
			continue
		}
		pkg := selection.Release.Packages[s.Package]
		if pkg.Store != "" {
			sources[pkg.Name] = &storeSource{
				name:  pkg.Name,
				store: pkg.Store,
				// TODO: populate arch, track and risk when implementing
				// fetching from the store.
			}
			continue
		}

		var candidates []*setup.Archive
		if pkg.Archive == "" {
			// If the package has not pinned any archive, choose the highest
			// priority archive in which the package exists.
			candidates = sortedArchives
		} else {
			candidates = []*setup.Archive{selection.Release.Archives[pkg.Archive]}
		}

		var chosen archive.Archive
		for _, archiveInfo := range candidates {
			archive := archives[archiveInfo.Name]
			if archive != nil && archive.Exists(pkg.RealName) {
				chosen = archive
				break
			}
		}
		if chosen == nil {
			return nil, fmt.Errorf("cannot find package %q in archive(s)", pkg.RealName)
		}
		sources[pkg.Name] = &archiveSource{
			archive: chosen,
			name:    pkg.RealName,
		}
	}

	return sources, nil
}
