package testutil

import (
	"bytes"
	"fmt"
	"io"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/cache"
	"github.com/canonical/chisel/internal/pkgutil"
)

type TestArchive struct {
	Opts     archive.Options
	Packages map[string]*TestPackage
}

type TestPackage struct {
	Name     string
	Version  string
	Hash     string
	Arch     string
	Data     []byte
	Archives []string
}

func (a *TestArchive) Options() *archive.Options {
	return &a.Opts
}

func (a *TestArchive) Fetch(pkgName string) (io.ReadSeekCloser, *pkgutil.Info, error) {
	pkg, ok := a.Packages[pkgName]
	if !ok {
		return nil, nil, fmt.Errorf("cannot find package %q in archive", pkgName)
	}
	return ReadSeekNopCloser(bytes.NewReader(pkg.Data)), pkg.info(), nil
}

func (a *TestArchive) Exists(pkg string) bool {
	_, ok := a.Packages[pkg]
	return ok
}

func (a *TestArchive) Info(pkgName string) (*pkgutil.Info, error) {
	pkg, ok := a.Packages[pkgName]
	if !ok {
		return nil, fmt.Errorf("cannot find package %q in archive", pkgName)
	}
	return pkg.info(), nil
}

// info returns the package information as a package source would report it.
func (p *TestPackage) info() *pkgutil.Info {
	return &pkgutil.Info{
		Name:       p.Name,
		Version:    p.Version,
		Arch:       p.Arch,
		DigestKind: cache.SHA256,
		Digest:     p.Hash,
	}
}
