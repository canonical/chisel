package main

import (
	"github.com/jessevdk/go-flags"

	"path/filepath"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/slicer"
)

var shortSliceHelp = "Slice the release out"
var longSliceHelp = `
The slice command uses the provided selection of package slices
to create a new filesystem tree in the root location.
`

var sliceDescs = map[string]string{
	"release": "Chisel release directory",
	"root":    "Root for generated content",
}

type cmdSlice struct {
	ReleaseDir string `long:"release" value-name:"<dir>" required:"yes"`
	RootDir    string `long:"root" value-name:"<dir>" required:"yes"`

	Positional struct {
		SliceRefs []string `positional-arg-name:"<slice names>" required:"yes"`
	} `positional-args:"yes"`
}

func init() {
	addCommand("slice", shortSliceHelp, longSliceHelp, func() flags.Commander { return &cmdSlice{} }, sliceDescs, nil)
}

func (cmd *cmdSlice) Execute(args []string) error {
	if len(args) > 0 {
		return ErrExtraArgs
	}

	sliceKeys := make([]setup.SliceKey, len(cmd.Positional.SliceRefs))
	for i, sliceRef := range cmd.Positional.SliceRefs {
		sliceKey, err := setup.ParseSliceKey(sliceRef)
		if err != nil {
			return err
		}
		sliceKeys[i] = sliceKey
	}

	release, err := setup.ReadRelease(cmd.ReleaseDir)
	if err != nil {
		return err
	}

	selection, err := setup.Select(release, sliceKeys)
	if err != nil {
		return err
	}

	archives := make(map[string]archive.Archive)
	for archiveName, archiveInfo := range release.Archives {
		openArchive, err := archive.Open(&archive.Options{
			Label:    archiveName,
			Version:  archiveInfo.Version,
			CacheDir: filepath.Join(cmd.ReleaseDir, ".cache"),
			Arch:     "amd64", // TODO Option for this, implied from running system
		})
		if err != nil {
			return err
		}
		archives[archiveName] = openArchive
	}

	return slicer.Run(&slicer.RunOptions{
		Selection: selection,
		Archives:  archives,
		TargetDir: cmd.RootDir,
	})

	return printVersions()
}
