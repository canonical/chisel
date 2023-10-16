package main

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/jessevdk/go-flags"

	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/strdist"
)

var shortFindHelp = "Find existing slices"
var longFindHelp = `
The find command queries the chisel releases for available slices.

With the --release flag, it queries for slices in a particular branch
of the chisel-releases repository[1] or a particular directory. If left
unspecified, it queries with the release info found in /etc/lsb-release.

[1] https://github.com/canonical/chisel-releases
`

var findDescs = map[string]string{
	"release": "Chisel release branch or directory",
}

type cmdFind struct {
	Release string `long:"release" value-name:"<branch|dir>"`

	Positional struct {
		Query []string `positional-arg-name:"<query>" required:"yes"`
	} `positional-args:"yes"`
}

func init() {
	addCommand("find", shortFindHelp, longFindHelp, func() flags.Commander { return &cmdFind{} }, findDescs, nil)
}

func (cmd *cmdFind) Execute(args []string) error {
	if len(args) > 0 {
		return ErrExtraArgs
	}

	query := strings.TrimSpace(strings.Join(cmd.Positional.Query, " "))
	if query == "" {
		return fmt.Errorf("no search term specified")
	}

	release, releaseLabel, err := getRelease(cmd.Release)
	if err != nil {
		return err
	}

	slices, err := findSlices(release, query)
	if err != nil {
		return err
	}
	if slices == nil {
		fmt.Fprintf(Stdout, "No matching slices for \"%s\"\n", query)
		return nil
	}

	w := tabWriter()
	fmt.Fprintf(w, "Slice\tPackage\tRelease\n")
	for _, s := range slices {
		fmt.Fprintf(w, "%s\t%s\t%s\n", s, s.Package, releaseLabel)
	}
	w.Flush()

	return nil
}

const maxStrDist int64 = 1

// matchSlice returns true if a slice (partially) matches with a query.
func matchSlice(slice *setup.Slice, query string) bool {
	// check if the query is a substring of the pkg_slice slice name
	if strings.Contains(slice.String(), query) {
		return true
	}
	// check if the query string is atmost ``maxStrDist`` Levenshtein [1]
	// distance away from the pkg_slice slice name.
	// [1] https://en.wikipedia.org/wiki/Levenshtein_distance
	dist := strdist.Distance(slice.String(), query, strdist.StandardCost, maxStrDist+1)
	if dist <= maxStrDist {
		return true
	}
	return false
}

// findSlices goes through the release searching for any slices that matches
// the query string. It returns a list of slices who matches the query.
func findSlices(release *setup.Release, query string) (slices []*setup.Slice, err error) {
	if release == nil {
		return nil, fmt.Errorf("cannot find slice: invalid release")
	}
	for _, pkg := range release.Packages {
		for _, slice := range pkg.Slices {
			if slice != nil && matchSlice(slice, query) {
				slices = append(slices, slice)
			}
		}
	}
	sort.Slice(slices, func(i, j int) bool {
		return slices[i].String() < slices[j].String()
	})
	return slices, nil
}

func tabWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(Stdout, 5, 3, 2, ' ', 0)
}
