package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var licenseRegexp = regexp.MustCompile("// SPDX-License-Identifier: ([^\\s]*)$")

func fileLicense(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		matches := licenseRegexp.FindStringSubmatch(line)
		if len(matches) > 0 {
			return matches[1], nil
		}
	}

	return "", nil
}

func checkDirLicense(path string, valid string) error {
	return filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		license, err := fileLicense(path)
		if err != nil {
			return err
		}
		if license == "" {
			return fmt.Errorf("cannot find a valid license in %q", path)
		}
		if license != valid {
			return fmt.Errorf("expected %q to be %q, got %q", path, valid, license)
		}
		return nil
	})
}

func run() error {
	// Check external packages licenses.
	err := checkDirLicense("public", "Apache-2.0")
	if err != nil {
		return fmt.Errorf("invalid license in exported package: %s", err)
	}

	// Check the internal dependencies of the external packages.
	output, err := exec.Command("sh", "-c", "go list -deps -test ./public/*").Output()
	if err != nil {
		return err
	}
	lines := strings.Split(string(output), "\n")
	var internalPkgs []string
	for _, line := range lines {
		if strings.Contains(line, "github.com/canonical/chisel/internal") {
			internalPkgs = append(internalPkgs, strings.TrimPrefix(line, "github.com/canonical/chisel/"))
		}
	}
	for _, pkg := range internalPkgs {
		err := checkDirLicense(pkg, "Apache-2.0")
		if err != nil {
			return fmt.Errorf("invalid license in depedency %q: %s", pkg, err)
		}
	}

	return nil
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
