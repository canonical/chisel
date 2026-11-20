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
	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		const labelPrefix = "ID="
		const versionPrefix = "VERSION_ID="
		for _, line := range strings.Split(string(data), "\n") {
			switch {
			case strings.HasPrefix(line, labelPrefix):
				label = strings.ToLower(strings.Trim(line[len(labelPrefix):], `"'`))
			case strings.HasPrefix(line, versionPrefix):
				version = strings.Trim(line[len(versionPrefix):], `"'`)
			}
			if label != "" && version != "" {
				return label, version, nil
			}
		}
	}
	return "", "", fmt.Errorf("cannot infer release via /etc/os-release, see the --release option")
}

// obtainRelease returns the Chisel release information matching the provided string,
// fetching it if necessary. The provided string should be either:
// * "<name>-<version>",
// * the path to a directory containing a previously fetched release,
// * "" and Chisel will attempt to read the release label from the host.
func obtainRelease(releaseStr string) (release *setup.Release, err error) {
	if strings.Contains(releaseStr, "/") {
		release, err = setup.ReadRelease(releaseStr)
	} else {
		var label, version string
		if releaseStr == "" {
			label, version, err = readReleaseInfo()
		} else {
			label, version, err = parseReleaseInfo(releaseStr)
		}
		if err != nil {
			return nil, err
		}
		release, err = setup.FetchRelease(&setup.FetchOptions{
			Label:   label,
			Version: version,
		})
	}
	if err != nil {
		return nil, err
	}
	return release, nil
}
