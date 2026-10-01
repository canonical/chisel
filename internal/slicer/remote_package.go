package slicer

import (
	"fmt"
	"io"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/cache"
)

// RemotePackageInfo describes a package as reported by its provider.
type RemotePackageInfo struct {
	Name       string
	Version    string
	Revision   int
	Arch       string
	Store      string
	DigestKind cache.DigestKind
	Digest     string
}

func (p *RemotePackageInfo) PkgName() string                 { return p.Name }
func (p *RemotePackageInfo) PkgVersion() string              { return p.Version }
func (p *RemotePackageInfo) PkgRevision() int                { return p.Revision }
func (p *RemotePackageInfo) PkgArch() string                 { return p.Arch }
func (p *RemotePackageInfo) PkgStore() string                { return p.Store }
func (p *RemotePackageInfo) PkgDigestKind() cache.DigestKind { return p.DigestKind }
func (p *RemotePackageInfo) PkgDigest() string               { return p.Digest }

type RemotePackage interface {
	Fetch() (io.ReadSeekCloser, *RemotePackageInfo, error)
}

var (
	_ RemotePackage = (*debPackage)(nil)
	_ RemotePackage = (*binPackage)(nil)
)

type debPackage struct {
	archive archive.Archive
	name    string
}

func (d *debPackage) Fetch() (io.ReadSeekCloser, *RemotePackageInfo, error) {
	reader, info, err := d.archive.Fetch(d.name)
	if err != nil {
		return nil, nil, err
	}
	return reader, &RemotePackageInfo{
		Name:       info.Name,
		Version:    info.Version,
		Arch:       info.Arch,
		DigestKind: cache.SHA256,
		Digest:     info.SHA256,
	}, nil
}

type binPackage struct {
	name     string
	realName string
	store    string
}

func (b *binPackage) Fetch() (io.ReadSeekCloser, *RemotePackageInfo, error) {
	return nil, nil, fmt.Errorf("cannot fetch package %q from store %q: not implemented", b.name, b.store)
}
