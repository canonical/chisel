package testutil

import (
	"bytes"
	"fmt"
	"io"

	"github.com/canonical/chisel/internal/archive"
)

type TestArchive struct {
	Opts archive.Options
	Pkgs map[string]TestPackage
}

type TestPackage struct {
	Name    string
	Version string
	Hash    string
	Arch    string
	Data    []byte
}

func (a *TestArchive) Options() *archive.Options {
	return &a.Opts
}

func (a *TestArchive) Fetch(pkgName string) (io.ReadCloser, error) {
	if pkg, ok := a.Pkgs[pkgName]; ok {
		return io.NopCloser(bytes.NewBuffer(pkg.Data)), nil
	}
	return nil, fmt.Errorf("cannot find package %q in archive", pkgName)
}

func (a *TestArchive) Exists(pkg string) bool {
	_, ok := a.Pkgs[pkg]
	return ok
}

func (a *TestArchive) Info(pkgName string) (*archive.PackageInfo, error) {
	pkg, ok := a.Pkgs[pkgName]
	if !ok {
		return nil, fmt.Errorf("cannot find package %q in archive", pkgName)
	}
	return &archive.PackageInfo{
		Name:    pkg.Name,
		Version: pkg.Version,
		Hash:    pkg.Hash,
		Arch:    pkg.Arch,
	}, nil
}
