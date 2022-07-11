package deb

import (
	"fmt"
	"runtime"
)

type archPair struct {
	goArch  string
	debArch string
}

var knownArchs = []archPair{
	{"386", "i386"},
	{"amd64", "amd64"},
	{"arm", "armhf"},
	{"arm64", "arm64"},
	{"ppc64le", "ppc64el"},
	{"riscv64", "riscv64"},
	{"s390x", "s390x"},
}

var platformGoArch = runtime.GOARCH

func InferArch() (string, error) {
	for _, arch := range knownArchs {
		if arch.goArch == platformGoArch {
			return arch.debArch, nil
		}
	}
	return "", fmt.Errorf("cannot infer package architecture from current platform architecture: %s", platformGoArch)
}

func ValidateArch(debArch string) error {
	for _, arch := range knownArchs {
		if arch.debArch == debArch {
			return nil
		}
	}
	return fmt.Errorf("invalid package architecture: %s", debArch)
}
