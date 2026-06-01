// SPDX-License-Identifier: Apache-2.0

package apacheutil

import (
	"fmt"
	"regexp"
)

type SliceKey struct {
	Package string
	Kind    string
	Slice   string
}

func (s SliceKey) String() string {
	if s.Kind != "" {
		return s.Kind + "/" + s.Package + "_" + s.Slice
	}
	return s.Package + "_" + s.Slice
}

// PkgKey returns the qualified package key used for Release.Packages lookups.
func (s SliceKey) PkgKey() string {
	if s.Kind != "" {
		return s.Kind + "/" + s.Package
	}
	return s.Package
}

// FnameExp matches the slice definition file basename.
var FnameExp = regexp.MustCompile(`^([a-z0-9](?:-?[.a-z0-9+]){1,})\.yaml$`)

// SnameExp matches only the slice name, without the leading package name.
var SnameExp = regexp.MustCompile(`^([a-z](?:-?[a-z0-9]){2,})$`)

// knameExp matches the slice full name in pkg_slice or kind/pkg_slice format.
var knameExp = regexp.MustCompile(`^(?:([a-z0-9](?:-?[.a-z0-9+]){0,})/)?([a-z0-9](?:-?[.a-z0-9+]){1,})_([a-z](?:-?[a-z0-9]){2,})$`)

func ParseSliceKey(sliceKey string) (SliceKey, error) {
	match := knameExp.FindStringSubmatch(sliceKey)
	if match == nil {
		return SliceKey{}, fmt.Errorf("invalid slice reference: %q", sliceKey)
	}
	return SliceKey{Package: match[2], Kind: match[1], Slice: match[3]}, nil
}
