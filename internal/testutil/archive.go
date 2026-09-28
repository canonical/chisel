package testutil

import (
	"bytes"
	"fmt"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/deb"
	"github.com/canonical/chisel/internal/tarball"
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

func (a *TestArchive) Fetch(pkgName string) (tarball.PkgReader, *archive.PackageInfo, error) {
	pkg, ok := a.Packages[pkgName]
	if !ok {
		return nil, nil, fmt.Errorf("cannot find package %q in archive", pkgName)
	}
	info := &archive.PackageInfo{
		Name:    pkg.Name,
		Version: pkg.Version,
		SHA256:  pkg.Hash,
		Arch:    pkg.Arch,
	}
	return deb.OpenPkg(ReadSeekNopCloser(bytes.NewReader(pkg.Data))), info, nil
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
		SHA256:  pkg.Hash,
		Arch:    pkg.Arch,
	}, nil
}
