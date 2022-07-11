package archive_test

import (
	. "gopkg.in/check.v1"

	"debug/elf"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/deb"
)

// TODO Implement local test server instead of using live archive.

var elfToDebArch = map[elf.Machine]string{
	elf.EM_386:     "i386",
	elf.EM_AARCH64: "arm64",
	elf.EM_ARM:     "armhf",
	elf.EM_PPC64:   "ppc64el",
	elf.EM_RISCV:   "riscv64",
	elf.EM_S390:    "s390x",
	elf.EM_X86_64:  "amd64",
}

func (s *S) checkArchitecture(c *C, arch string, binaryPath string) {
	file, err := elf.Open(binaryPath)
	c.Assert(err, IsNil)
	defer file.Close()

	binaryArch := elfToDebArch[file.Machine]
	c.Assert(binaryArch, Equals, arch)
}

func (s *S) testOpenArchiveArch(c *C, arch string) {
	options := archive.Options{
		Label:    "ubuntu",
		Version:  "22.04",
		CacheDir: c.MkDir(),
		Arch:     arch,
	}

	archive, err := archive.Open(&options)
	c.Assert(err, IsNil)

	extractDir := c.MkDir()

	pkg, err := archive.Fetch("hostname")
	c.Assert(err, IsNil)

	err = deb.Extract(pkg, &deb.ExtractOptions{
		Package:   "hostname",
		TargetDir: extractDir,
		Extract: map[string][]deb.ExtractInfo{
			"/usr/share/doc/hostname/copyright": {
				{Path: "/copyright"},
			},
			"/bin/hostname": {
				{Path: "/hostname"},
			},
		},
	})
	c.Assert(err, IsNil)

	data, err := ioutil.ReadFile(filepath.Join(extractDir, "copyright"))
	c.Assert(err, IsNil)

	copyrightTop := "This package was written by Peter Tobias <tobias@et-inf.fho-emden.de>\non Thu, 16 Jan 1997 01:00:34 +0100."
	c.Assert(strings.HasPrefix(string(data), copyrightTop), Equals, true)

	s.checkArchitecture(c, arch, filepath.Join(extractDir, "hostname"))
}

func (s *S) TestOpenArchive(c *C) {
	for _, arch := range elfToDebArch {
		s.testOpenArchiveArch(c, arch)
	}
}
