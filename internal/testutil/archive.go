package testutil

import (
	"bytes"
	"fmt"
	"io"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/cache"
)

type TestArchive struct {
	Opts     archive.Options
	Packages map[string]*TestPackage
}

type TestPackage struct {
	Name    string
	Version string
	// Hashes holds the digests of the package, keyed by digest kind. When
	// unset, a sha256 digest of Hash is used.
	Hashes   map[cache.DigestKind]string
	Hash     string
	Arch     string
	Data     []byte
	Archives []string
}

func (p *TestPackage) digests() map[cache.DigestKind]string {
	if p.Hashes == nil {
		return map[cache.DigestKind]string{cache.SHA256: p.Hash}
	}
	return p.Hashes
}

func (a *TestArchive) Options() *archive.Options {
	return &a.Opts
}

func (a *TestArchive) Fetch(pkgName string) (io.ReadSeekCloser, *archive.PackageInfo, error) {
	pkg, ok := a.Packages[pkgName]
	if !ok {
		return nil, nil, fmt.Errorf("cannot find package %q in archive", pkgName)
	}
	info := &archive.PackageInfo{
		Name:    pkg.Name,
		Version: pkg.Version,
		Digests: pkg.digests(),
		Arch:    pkg.Arch,
	}
	return ReadSeekNopCloser(bytes.NewReader(pkg.Data)), info, nil
}

func (a *TestArchive) Exists(pkg string) bool {
	_, ok := a.Packages[pkg]
	return ok
}

func (a *TestArchive) Info(pkgName string) (*archive.PackageInfo, error) {
	pkg, ok := a.Packages[pkgName]
	if !ok {
		return nil, fmt.Errorf("cannot find package %q in archive", pkgName)
	}
	return &archive.PackageInfo{
		Name:    pkg.Name,
		Version: pkg.Version,
		Digests: pkg.digests(),
		Arch:    pkg.Arch,
	}, nil
}
