package slicer

import (
	"fmt"
	"io"
	"slices"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/setup"
)

// Fetcher fetches a package from the location selected for it in the
// release.
type Fetcher interface {
	Arch() string
	Fetch() (io.ReadSeekCloser, *archive.PackageInfo, error)
}

var (
	_ Fetcher = (*debFetcher)(nil)
	_ Fetcher = (*binFetcher)(nil)
)

// debFetcher fetches packages from an archive.
type debFetcher struct {
	archive archive.Archive
	name    string
}

func (d *debFetcher) Arch() string {
	return d.archive.Options().Arch
}

func (d *debFetcher) Fetch() (io.ReadSeekCloser, *archive.PackageInfo, error) {
	return d.archive.Fetch(d.name)
}

// binFetcher fetches bin packages from a store.
type binFetcher struct {
	arch  string
	name  string
	store string
}

func (b *binFetcher) Arch() string {
	return b.arch
}

func (b *binFetcher) Fetch() (io.ReadSeekCloser, *archive.PackageInfo, error) {
	return nil, nil, fmt.Errorf("cannot fetch package %q from store %q: not implemented", b.name, b.store)
}

// resolveFetchers determines the fetcher for each package in the selection.
// For packages from an archive it selects the highest priority archive
// containing the package unless a particular archive is pinned within the
// slice definition file. For packages from a store it selects the store
// named in the slice definition file. It returns a map of Fetcher indexed
// by package names.
func resolveFetchers(archives map[string]archive.Archive, selection *setup.Selection) (map[string]Fetcher, error) {
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

	fetchers := make(map[string]Fetcher)
	for _, s := range selection.Slices {
		if _, ok := fetchers[s.Package]; ok {
			continue
		}
		pkg := selection.Release.Packages[s.Package]
		if pkg.Store != "" {
			fetchers[pkg.Name] = &binFetcher{
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
		fetchers[pkg.Name] = &debFetcher{
			archive: chosen,
			name:    pkg.RealName,
		}
	}

	return fetchers, nil
}
