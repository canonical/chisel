package fsutil

import (
	"path/filepath"
	"strings"
)

// These functions exists because we work with slash terminated directory paths
// that come from deb package tarballs but standard library path functions
// treat slash terminated paths as unclean.

// Like filepath.Dir() but trailing slash on input is ignored and the return
// value always includes trailing slash.
//
// Comparison of filepath.Dir() with fsutil.Dir():
//
//	filepath.Dir("/foo/bar/")	== "/foo/bar"
//	filepath.Dir("/foo/bar")	== "/foo"
//
//	fsutil.Dir("/foo/bar")		== "/foo/"
//	fsutil.Dir("/foo/bar/")		== "/foo/"
func Dir(path string) string {
	parent := filepath.Dir(filepath.Clean(path))
	if parent != "/" {
		parent += "/"
	}
	return parent
}

// Like filepath.Clean() but keeps trailing slash.
//
// Comparison of filepath.Clean() with fsutil.Clean():
//
//	filepath.Clean("/foo/bar")	== "/foo/bar"
//	filepath.Clean("/foo/bar/")	== "/foo/bar"
//	filepath.Clean("/foo/bar/.//)	== "/foo/bar"
//
//	fsutil.Clean("/foo/bar")	== "/foo/bar"
//	fsutil.Clean("/foo/bar/")	== "/foo/bar/"
//	fsutil.Clean("/foo/bar/.//")	== "/foo/bar/"
func Clean(path string) string {
	clean := filepath.Clean(path)
	if strings.HasSuffix(path, "/") {
		clean += "/"
	}
	return clean
}
