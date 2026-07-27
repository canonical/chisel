// Package pkgutil provides types and helpers describing packages
// independently of where they come from (a Debian archive, a store, ...).
package pkgutil

import (
	"github.com/canonical/chisel/internal/cache"
)

// Info describes a package as obtained from its source.
type Info struct {
	Name    string // Name as known by the source (e.g. "curl")
	Version string
	// Revision further identifies the package when the source versions are not
	// unique on their own. It is zero when the source does not use revisions.
	Revision   int
	Arch       string
	DigestKind cache.DigestKind
	Digest     string
}
