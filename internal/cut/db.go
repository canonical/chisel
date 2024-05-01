package cut

import (
	"path/filepath"
	"strings"

	"github.com/canonical/chisel/internal/setup"
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

// getManifestPath parses the "generate" glob path to get the regular path to its
// directory.
func getManifestPath(generatePath string) string {
	dir := filepath.Clean(strings.TrimSuffix(generatePath, "**"))
	return filepath.Join(dir, dbFile)
}

// locateManifestSlices finds the paths marked with "generate:manifest" and
// returns a map from said path to all the slices that declare it.
func locateManifestSlices(slices []*setup.Slice) map[string][]*setup.Slice {
	manifestSlices := make(map[string][]*setup.Slice)
	for _, s := range slices {
		for path, info := range s.Contents {
			if info.Generate == setup.GenerateManifest {
				if manifestSlices[path] == nil {
					manifestSlices[path] = []*setup.Slice{}
				}
				manifestSlices[path] = append(manifestSlices[path], s)
			}
		}
	}
	return manifestSlices
}
