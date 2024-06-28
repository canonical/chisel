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
The find command queries the slice definitions for matching slices.
Globs (* and ?) are allowed in the query.

By default it fetches the slices for the same Ubuntu version as the
current host, unless the --release flag is used.
`

var findDescs = map[string]string{
	"release": "Chisel release name or directory (e.g. ubuntu-22.04)",
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

	release, err := obtainRelease(cmd.Release)
	if err != nil {
		return err
	}

	slices, err := findSlices(release, cmd.Positional.Query)
	if err != nil {
		return err
	}
	if len(slices) == 0 {
		fmt.Fprintf(Stderr, "No matching slices for \"%s\"\n", strings.Join(cmd.Positional.Query, " "))
		return nil
	}

	w := tabWriter()
	fmt.Fprintf(w, "Slice\tSummary\n")
	for _, s := range slices {
		fmt.Fprintf(w, "%s\t%s\n", s, "-")
	}
	w.Flush()

	return nil
}

// match reports whether a slice (partially) matches the query.
func match(slice *setup.Slice, query string) bool {
	var term string
	switch {
	case strings.HasPrefix(query, "_"):
		query = strings.TrimPrefix(query, "_")
		term = slice.Name
	case strings.Contains(query, "_"):
		term = slice.String()
	default:
		term = slice.Package
	}
	query = strings.ReplaceAll(query, "**", "⁑")
	return strdist.Distance(term, query, distWithGlobs, 0) <= 1
}

// findSlices returns slices from the provided release that match all of the
// query strings (AND).
func findSlices(release *setup.Release, query []string) (slices []*setup.Slice, err error) {
	slices = []*setup.Slice{}
	for _, pkg := range release.Packages {
		for _, slice := range pkg.Slices {
			if slice == nil {
				continue
			}
			allMatch := true
			for _, term := range query {
				if !match(slice, term) {
					allMatch = false
					break
				}
			}
			if allMatch {
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

// distWithGlobs encodes the standard Levenshtein distance with support for
// "*", "?" and "**". However, because it works on runes "**" has to be encoded
// as "⁑" in the strings.
//
// Supported wildcards:
//
//	?  - Any one character
//	*  - Any zero or more characters
//	⁑  - Any zero or more characters
func distWithGlobs(ar, br rune) strdist.Cost {
	if ar == '⁑' || br == '⁑' {
		return strdist.Cost{SwapAB: 0, DeleteA: 0, InsertB: 0}
	}
	if ar == '*' || br == '*' {
		return strdist.Cost{SwapAB: 0, DeleteA: 0, InsertB: 0}
	}
	if ar == '?' || br == '?' {
		return strdist.Cost{SwapAB: 0, DeleteA: 1, InsertB: 1}
	}
	return strdist.StandardCost(ar, br)
}
