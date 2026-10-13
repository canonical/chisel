package slicer

import (
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

// PackageRemote represents the remote from which a package is obtained.
type PackageRemote interface {
	// Arch returns the architecture the remote serves packages for, which is
	// the target architecture used to request the package, not the
	// architecture of the fetched package itself (which may be "all", for
	// example).
	Arch() string
	Fetch() (io.ReadSeekCloser, *RemotePackageInfo, error)
}

var _ PackageRemote = (*archiveRemote)(nil)

type archiveRemote struct {
	archive archive.Archive
	name    string
}

func (d *archiveRemote) Arch() string {
	return d.archive.Options().Arch
}

func (d *archiveRemote) Fetch() (io.ReadSeekCloser, *RemotePackageInfo, error) {
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
