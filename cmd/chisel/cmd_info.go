package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jessevdk/go-flags"
	"gopkg.in/yaml.v3"

	"github.com/canonical/chisel/internal/setup"
)

var shortInfoHelp = "Show information about package slices"
var longInfoHelp = `
The info command shows detailed information about package slices.

It accepts a whitespace-separated list of strings. The list can be
composed of package names, slice names, or a combination of both. The
default output format is YAML. When multiple arguments are provided,
the output is a list of YAML documents separated by a "---" line.

Slice definitions are shown verbatim according to their definition in
the selected release. For example, globs are not expanded.
`

var infoDescs = map[string]string{
	"release": "Chisel release name or directory (e.g. ubuntu-22.04)",
}

type infoCmd struct {
	Release string `long:"release" value-name:"<branch|dir>"`

	Positional struct {
		Queries []string `positional-arg-name:"<pkg|slice>" required:"yes"`
	} `positional-args:"yes"`
}

func init() {
	addCommand("info", shortInfoHelp, longInfoHelp, func() flags.Commander { return &infoCmd{} }, infoDescs, nil)
}

func (cmd *infoCmd) Execute(args []string) error {
	if len(args) > 0 {
		return ErrExtraArgs
	}

	release, err := obtainRelease(cmd.Release)
	if err != nil {
		return err
	}

	packages, notFound := selectPackageSlices(release, cmd.Positional.Queries)

	for i, pkg := range packages {
		data, err := yaml.Marshal(pkg)
		if err != nil {
			return err
		}
		if i > 0 {
			fmt.Fprintln(Stdout, "---")
		}
		fmt.Fprint(Stdout, string(data))
	}

	if len(notFound) > 0 {
		for i := range notFound {
			notFound[i] = strconv.Quote(notFound[i])
		}
		return fmt.Errorf("no slice definitions found for: " + strings.Join(notFound, ", "))
	}

	return nil
}

// selectPackageSlices takes in a release and a list of query strings
// of package names and/or slice names, and returns a list of packages
// containing the found slices. It also returns a list of query
// strings that were not found.
func selectPackageSlices(release *setup.Release, queries []string) (packages []*setup.Package, notFound []string) {
	var pkgOrder []string
	pkgSlices := make(map[string][]string)
	allPkgSlices := make(map[string]bool)

	sliceExists := func(key setup.SliceKey) bool {
		pkg, ok := release.Packages[key.Package]
		if !ok {
			return false
		}
		_, ok = pkg.Slices[key.Slice]
		return ok
	}
	for _, query := range queries {
		var pkg, slice string
		if strings.Contains(query, "_") {
			key, err := setup.ParseSliceKey(query)
			if err != nil || !sliceExists(key) {
				notFound = append(notFound, query)
				continue
			}
			pkg, slice = key.Package, key.Slice
		} else {
			if _, ok := release.Packages[query]; !ok {
				notFound = append(notFound, query)
				continue
			}
			pkg = query
		}
		if len(pkgSlices[pkg]) == 0 && !allPkgSlices[pkg] {
			pkgOrder = append(pkgOrder, pkg)
		}
		if slice == "" {
			allPkgSlices[pkg] = true
		} else {
			pkgSlices[pkg] = append(pkgSlices[pkg], slice)
		}
	}

	for _, pkgName := range pkgOrder {
		var pkg *setup.Package
		if allPkgSlices[pkgName] {
			pkg = release.Packages[pkgName]
		} else {
			releasePkg := release.Packages[pkgName]
			pkg = &setup.Package{
				Name:    releasePkg.Name,
				Archive: releasePkg.Archive,
				Slices:  make(map[string]*setup.Slice),
			}
			for _, sliceName := range pkgSlices[pkgName] {
				pkg.Slices[sliceName] = releasePkg.Slices[sliceName]
			}
		}
		packages = append(packages, pkg)
	}
	return packages, notFound
}
