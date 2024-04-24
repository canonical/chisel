package main

import (
	"path/filepath"
	"strings"
)

const dbFile = "chisel.db"
const dbSchema = "1.0"

type dbPackage struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Digest  string `json:"sha256"`
	Arch    string `json:"arch"`
}

type dbSlice struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type dbPath struct {
	Kind      string   `json:"kind"`
	Path      string   `json:"path"`
	Mode      string   `json:"mode"`
	Slices    []string `json:"slices"`
	Hash      string   `json:"sha256,omitempty"`
	FinalHash string   `json:"final_sha256,omitempty"`
	Size      uint64   `json:"size,omitempty"`
	Link      string   `json:"link,omitempty"`
}

type dbContent struct {
	Kind  string `json:"kind"`
	Slice string `json:"slice"`
	Path  string `json:"path"`
}

// getManifestPath parses the "generate" path and returns the absolute path of
// the location to be generated.
func getManifestPath(generatePath string) string {
	dir := filepath.Clean(strings.TrimSuffix(generatePath, "**"))
	return filepath.Join(dir, dbFile)
}
