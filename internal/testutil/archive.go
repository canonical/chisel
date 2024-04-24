package testutil

import (
	"bytes"
	"fmt"
	"io"

	"github.com/canonical/chisel/internal/archive"
)

type TestArchive struct {
	Opts archive.Options
	Pkgs map[string][]byte
}

func (a *TestArchive) Options() *archive.Options {
	return &a.Opts
}

func (a *TestArchive) Fetch(pkg string) (io.ReadCloser, error) {
	if data, ok := a.Pkgs[pkg]; ok {
		return io.NopCloser(bytes.NewBuffer(data)), nil
	}
	return nil, fmt.Errorf("attempted to open %q package", pkg)
}

func (a *TestArchive) Exists(pkg string) bool {
	_, ok := a.Pkgs[pkg]
	return ok
}

func (a *TestArchive) Info(pkg string) (*archive.PackageInfo, error) {
	if !a.Exists(pkg) {
		return nil, fmt.Errorf("cannot find package %q in archive", pkg)
	}
	return &archive.PackageInfo{
		Name:    pkg,
		Version: pkg + "_version",
		Hash:    pkg + "_hash",
		Arch:    pkg + "_arch",
	}, nil
}
