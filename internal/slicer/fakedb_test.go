package slicer_test

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/canonical/chisel/internal/db"
)

// fakeDB is used to compare a list of DB objects created by the slicer against
// a list of expected DB objects. We don't care about the order in which slicer
// creates DB objects. In real usage, they will be reordered by the jsonwall
// database anyway. We only care about the set of objects created. So we record
// the created objects and put them into fakeDB and put the expected objects
// into another fakeDB. Then, we compare both sets as sorted lists obtained
// from fakeDB.values().
//
// Since DB object types are not ordered nor comparable (Path has pointers), we
// keep different types of objects in different slices and sort these slices
// with a comparison function appropriate for each type.

type fakeDB struct {
	packages []db.Package
	slices   []db.Slice
	paths    []db.Path
	contents []db.Content
}

func (p *fakeDB) add(value any) error {
	switch v := value.(type) {
	case db.Package:
		p.packages = append(p.packages, v)
	case db.Slice:
		p.slices = append(p.slices, v)
	case db.Path:
		p.paths = append(p.paths, v)
	case db.Content:
		p.contents = append(p.contents, v)
	default:
		return fmt.Errorf("invalid DB type %T", v)
	}
	return nil
}

func (p *fakeDB) values() []any {
	sort.Slice(p.packages, func(i, j int) bool {
		x1 := p.packages[i].Name
		x2 := p.packages[j].Name
		return x1 < x2
	})
	sort.Slice(p.slices, func(i, j int) bool {
		x1 := p.slices[i].Name
		x2 := p.slices[j].Name
		return x1 < x2
	})
	sort.Slice(p.paths, func(i, j int) bool {
		x1 := p.paths[i].Path
		x2 := p.paths[j].Path
		return x1 < x2
	})
	sort.Slice(p.contents, func(i, j int) bool {
		x1 := p.contents[i].Slice
		x2 := p.contents[j].Slice
		y1 := p.contents[i].Path
		y2 := p.contents[j].Path
		return x1 < x2 || (x1 == x2 && y1 < y2)
	})
	i := 0
	vals := make([]any, len(p.packages)+len(p.slices)+len(p.paths)+len(p.contents))
	for _, v := range p.packages {
		vals[i] = v
		i++
	}
	for _, v := range p.slices {
		vals[i] = v
		i++
	}
	for _, v := range p.paths {
		vals[i] = v
		i++
	}
	for _, v := range p.contents {
		vals[i] = v
		i++
	}
	return vals
}

//nolint:unused
func (p *fakeDB) dumpValues(w io.Writer) {
	for _, v := range p.values() {
		switch t := v.(type) {
		case db.Package:
			fmt.Fprintln(w, "db.Package{")
			fmt.Fprintf(w, "\tName: %#v,\n", t.Name)
			fmt.Fprintf(w, "\tVersion: %#v,\n", t.Version)
			if t.SHA256 != "" {
				fmt.Fprintf(w, "\tSHA256: %#v,\n", t.SHA256)
			}
			if t.Arch != "" {
				fmt.Fprintf(w, "\tArch: %#v,\n", t.Arch)
			}
			fmt.Fprintln(w, "},")
		case db.Slice:
			fmt.Fprintln(w, "db.Slice{")
			fmt.Fprintf(w, "\tName: %#v,\n", t.Name)
			fmt.Fprintln(w, "},")
		case db.Path:
			fmt.Fprintln(w, "db.Path{")
			fmt.Fprintf(w, "\tPath: %#v,\n", t.Path)
			fmt.Fprintf(w, "\tMode: %#o,\n", t.Mode)
			fmt.Fprintf(w, "\tSlices: %#v,\n", t.Slices)
			if t.SHA256 != nil {
				fmt.Fprint(w, "\tSHA256: &[...]byte{")
				for i, b := range t.SHA256 {
					if i%8 == 0 {
						fmt.Fprint(w, "\n\t\t")
					} else {
						fmt.Fprint(w, " ")
					}
					fmt.Fprintf(w, "%#02x,", b)
				}
				fmt.Fprintln(w, "\n\t},")
			}
			if t.FinalSHA256 != nil {
				fmt.Fprint(w, "\tFinalSHA256: &[...]byte{")
				for i, b := range t.FinalSHA256 {
					if i%8 == 0 {
						fmt.Fprint(w, "\n\t\t")
					} else {
						fmt.Fprint(w, " ")
					}
					fmt.Fprintf(w, "%#02x,", b)
				}
				fmt.Fprintln(w, "\n\t},")
			}
			if t.Size != 0 {
				fmt.Fprintf(w, "\tSize: %d,\n", t.Size)
			}
			if t.Link != "" {
				fmt.Fprintf(w, "\tLink: %#v,\n", t.Link)
			}
			fmt.Fprintln(w, "},")
		case db.Content:
			fmt.Fprintln(w, "db.Content{")
			fmt.Fprintf(w, "\tSlice: %#v,\n", t.Slice)
			fmt.Fprintf(w, "\tPath: %#v,\n", t.Path)
			fmt.Fprintln(w, "},")
		default:
			panic(fmt.Sprintf("invalid DB value %#v", v))
		}
	}
}

//nolint:unused
func (p *fakeDB) dump() string {
	var buf strings.Builder
	fmt.Fprintln(&buf, "-----BEGIN DB DUMP-----")
	p.dumpValues(&buf)
	fmt.Fprintln(&buf, "-----END DB DUMP-----")
	return buf.String()
}
