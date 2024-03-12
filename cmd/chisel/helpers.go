package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/canonical/chisel/internal/setup"
)

// TODO These need testing

var releaseExp = regexp.MustCompile(`^([a-z](?:-?[a-z0-9]){2,})-([0-9]+(?:\.?[0-9])+)$`)

func parseReleaseInfo(release string) (label, version string, err error) {
	match := releaseExp.FindStringSubmatch(release)
	if match == nil {
		return "", "", fmt.Errorf("invalid release reference: %q", release)
	}
	return match[1], match[2], nil
}

func readReleaseInfo() (label, version string, err error) {
	data, err := os.ReadFile("/etc/lsb-release")
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

// readOrFetchRelease takes a release branch name or a release directory path.
// It fetches or reads the chisel-release depending on the nature of input and
// returns the release and release label.
// If the input is empty, it tries to read the release label from the host
// system and fetch the chisel-release accordingly.
func readOrFetchRelease(releaseStr string) (release *setup.Release, releaseLabel string, err error) {
	if strings.Contains(releaseStr, "/") {
		release, err = setup.ReadRelease(releaseStr)
		releaseLabel = releaseStr
	} else {
		var label, version string
		if releaseStr == "" {
			label, version, err = readReleaseInfo()
		} else {
			label, version, err = parseReleaseInfo(releaseStr)
		}
		if err != nil {
			return nil, "", err
		}
		release, err = setup.FetchRelease(&setup.FetchOptions{
			Label:   label,
			Version: version,
		})
		releaseLabel = label + "-" + version
	}
	if err != nil {
		return nil, "", err
	}
	return release, releaseLabel, nil
}
