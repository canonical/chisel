package fsutil

import (
	"path/filepath"
)

// isDirPath returns whether the path refers to a directory.
// The path refers to a directory when it ends with "/", "/." or "/..", or when
// it equals "." or "..".
func isDirPath(path string) bool {
	i := len(path) - 1
	if i < 0 {
		return true
	}
	if path[i] == '.' {
		i--
		if i < 0 {
			return true
		}
		if path[i] == '.' {
			i--
			if i < 0 {
				return true
			}
		}
	}
	if path[i] == '/' {
		return true
	}
	return false
}

// Debian package tarballs present paths slightly differently to what we would
// normally classify as clean paths. While a traditional clean file path is identical
// to a clean deb package file path, the deb package directory path always ends
// with a slash. Although the change only affects directory paths, the implication
// is that a directory path without a slash is interpreted as a file path. For this
// reason, we need to be very careful and handle both file and directory paths using
// a new set of functions. We call this new path type a Slashed Path. A slashed path
// allows us to identify a file or directory simply using lexical analysis.

// SlashedPathClean takes a file or slashed directory path as input, and produces
// the shortest equivalent as output. An input path ending without a slash will be
// interpreted as a file path. Directory paths should always end with a slash.
// These functions exists because we work with slash terminated directory paths
// that come from deb package tarballs but standard library path functions
// treat slash terminated paths as unclean.
func SlashedPathClean(path string) string {
	clean := filepath.Clean(path)
	if clean != "/" && isDirPath(path) {
		clean += "/"
	}
	return clean
}

// SlashedPathDir takes a file or slashed directory path as input, cleans the
// path and returns the parent directory. An input path ending without a slash
// will be interpreted as a file path. Directory paths should always end with a slash.
// Clean is like filepath.Clean() but trailing slash is kept.
func SlashedPathDir(path string) string {
	parent := filepath.Dir(filepath.Clean(path))
	if parent != "/" {
		parent += "/"
	}
	return parent
}
