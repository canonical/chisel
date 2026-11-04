package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/canonical/chisel/internal/cache"
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

// defaultCacheExpiry is how long a cache entry can go unused before it is
// removed, unless CHISEL_CACHE_EXPIRY says otherwise.
const defaultCacheExpiry = 7 * 24 * time.Hour

// cacheExpiry returns how long a cache entry can go unused before it is
// removed. CHISEL_CACHE_EXPIRY sets it as a number of days, and 0 disables
// expiry.
func cacheExpiry() (time.Duration, error) {
	value := os.Getenv("CHISEL_CACHE_EXPIRY")
	if value == "" {
		return defaultCacheExpiry, nil
	}
	// Limit it to 16 bits, about 179 years, so the duration cannot overflow.
	days, err := strconv.ParseUint(value, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid CHISEL_CACHE_EXPIRY %q, expected a number of days", value)
	}
	return time.Duration(days) * 24 * time.Hour, nil
}

// expireCache removes the entries of the cache in dir that have not been
// used for the time given by cacheExpiry. It is meant to run once a command
// is done with the cache.
func expireCache(dir string) {
	expiry, err := cacheExpiry()
	if err != nil {
		logf("Warning: %v; the cache is not expired", err)
		return
	}
	if expiry == 0 {
		return
	}
	// Reading an entry refreshes its timestamp, so this only drops what no
	// command has used for a while.
	cc := &cache.Cache{Dir: dir}
	if err := cc.Expire(expiry); err != nil {
		logf("Warning: cannot expire cache entries: %v", err)
	}
}
