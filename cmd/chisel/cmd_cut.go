package main

import (
	"github.com/jessevdk/go-flags"

	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/cache"
	"github.com/canonical/chisel/internal/setup"
	"github.com/canonical/chisel/internal/slicer"
)

var shortCutHelp = "Cut a tree with selected slices"
var longCutHelp = `
The cut command uses the provided selection of package slices
to create a new filesystem tree in the root location.
`

var cutDescs = map[string]string{
	"release": "Chisel release directory",
	"root":    "Root for generated content",
	"arch":    "Package architecture",
}

type cmdCut struct {
	Release string `long:"release" value-name:"<dir>"`
	RootDir string `long:"root" value-name:"<dir>" required:"yes"`
	Arch    string `long:"arch" value-name:"<arch>"`

	Positional struct {
		SliceRefs []string `positional-arg-name:"<slice names>" required:"yes"`
	} `positional-args:"yes"`
}

func init() {
	addCommand("cut", shortCutHelp, longCutHelp, func() flags.Commander { return &cmdCut{} }, cutDescs, nil)
}

func (cmd *cmdCut) Execute(args []string) error {
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

	var release *setup.Release
	var err error
	if strings.Contains(cmd.Release, "/") {
		release, err = setup.ReadRelease(cmd.Release)
	} else {
		var label, version string
		if cmd.Release == "" {
			label, version, err = readReleaseInfo()
		} else {
			label, version, err = parseReleaseInfo(cmd.Release)
		}
		if err != nil {
			return err
		}
		release, err = setup.FetchRelease(&setup.FetchOptions{
			Label:   label,
			Version: version,
		})
	}
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
			Label:      archiveName,
			Version:    archiveInfo.Version,
			Arch:       cmd.Arch,
			Suites:     archiveInfo.Suites,
			Components: archiveInfo.Components,
			CacheDir:   cache.DefaultDir("chisel"),
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

// TODO These need testing, and maybe moving into a common file.

var releaseExp = regexp.MustCompile(`^([a-z](?:-?[a-z0-9]){2,})-([0-9]+(?:\.?[0-9])+)$`)

func parseReleaseInfo(release string) (label, version string, err error) {
	match := releaseExp.FindStringSubmatch(release)
	if match == nil {
		return "", "", fmt.Errorf("invalid release reference: %q", release)
	}
	return match[1], match[2], nil
}

func readReleaseInfo() (label, version string, err error) {
	data, err := ioutil.ReadFile("/etc/lsb-release")
	if err == nil {
		const labelPrefix = "DISTRIB_ID="
		const versionPrefix = "DISTRIB_RELEASE="
		for _, line := range strings.Split(string(data), "\n") {
			switch {
			case strings.HasPrefix(line, labelPrefix):
				label = strings.ToLower(line[len(labelPrefix):])
			case strings.HasPrefix(line, versionPrefix):
				version = line[len(versionPrefix):]
			}
			if label != "" && version != "" {
				return label, version, nil
			}
		}
	}
	return "", "", fmt.Errorf("cannot infer release via /etc/lsb-release, see the --release option")
}
