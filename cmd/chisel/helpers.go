package main

import (
	"fmt"
	"os"
	"path/filepath"
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

// snapHostRoot is where snapd exposes the host's root filesystem inside a
// strictly confined snap. The snap's own /usr comes from the base snap, so
// /etc/os-release, a symlink into /usr/lib, describes the base and not the host.
const snapHostRoot = "/var/lib/snapd/hostfs"

func readReleaseInfo() (label, version string, err error) {
	root := "/"
	hint := "see the --release option"
	if os.Getenv("SNAP") != "" {
		root = snapHostRoot
		hint = "connect the chisel:host-os-release plug or see the --release option"
	}
	label, version, err = readOSRelease(root)
	if err != nil {
		return "", "", fmt.Errorf("cannot infer release via os-release: %w; %s", err, hint)
	}
	return label, version, nil
}

// readOSRelease reads ID and VERSION_ID from the os-release file of the
// filesystem at root, preferring /etc over /usr/lib as per os-release(5).
func readOSRelease(root string) (label, version string, err error) {
	for _, path := range []string{"/etc/os-release", "/usr/lib/os-release"} {
		var data []byte
		data, err = readFileAt(root, path)
		if err != nil {
			continue
		}
		label, version = parseOSRelease(data)
		if label == "" || version == "" {
			return "", "", fmt.Errorf("cannot find ID and VERSION_ID in %s", path)
		}
		return label, version, nil
	}
	return "", "", err
}

// readFileAt reads path from the filesystem at root, resolving symlinks
// against root so that /etc/os-release -> /usr/lib/os-release stays inside it.
func readFileAt(root, path string) ([]byte, error) {
	const maxLinks = 40
	for i := 0; ; i++ {
		target, err := os.Readlink(filepath.Join(root, path))
		if err != nil {
			return os.ReadFile(filepath.Join(root, path))
		}
		if i == maxLinks {
			return nil, fmt.Errorf("too many levels of symbolic links: %s", filepath.Join(root, path))
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		// Cleaning the rooted path stops ".." from climbing out of root.
		path = filepath.Clean("/" + target)
	}
}

func parseOSRelease(data []byte) (label, version string) {
	const labelPrefix = "ID="
	const versionPrefix = "VERSION_ID="
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, labelPrefix):
			label = strings.ToLower(strings.Trim(line[len(labelPrefix):], `"'`))
		case strings.HasPrefix(line, versionPrefix):
			version = strings.Trim(line[len(versionPrefix):], `"'`)
		}
	}
	return label, version
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
