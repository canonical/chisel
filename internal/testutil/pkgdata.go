package testutil

import (
	"archive/tar"
	"bytes"
	"strings"
	"time"

	"github.com/blakesmith/ar"
	"github.com/klauspost/compress/zstd"
)

var PackageData = map[string][]byte{}

var baseFilesPackageEntries = []TarEntry{
	Dir(0755, "./"),
	Dir(0755, "./etc/"),
	Reg(0644, "./etc/debian_version", "bullseye/sid\n"),
	Lnk(0644, "./etc/os-release", "../usr/lib/os-release"),
	Dir(0755, "./etc/default/"),
	Dir(0755, "./etc/dpkg/"),
	Dir(0755, "./etc/dpkg/origins/"),
	Reg(0644, "./etc/dpkg/origins/ubuntu", `Vendor: Ubuntu
Vendor-URL: http://www.ubuntu.com/
Bugs: https://bugs.launchpad.net/ubuntu/+filebug
Parent: Debian
`),
	Reg(0644, "./etc/dpkg/origins/debian", `Vendor: Debian
Vendor-URL: http://www.debian.org/
Bugs: debbugs://bugs.debian.org
`),
	Dir(0755, "./usr/"),
	Dir(0755, "./usr/bin/"),
	Reg(0775, "./usr/bin/hello", `#!/bin/sh
echo "Hello world"
`),
	Dir(0755, "./usr/lib/"),
	Reg(0644, "./usr/lib/os-release", `NAME="Ubuntu"
VERSION="20.04.4 LTS (Focal Fossa)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 20.04.4 LTS"
VERSION_ID="20.04"
HOME_URL="https://www.ubuntu.com/"
SUPPORT_URL="https://help.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
VERSION_CODENAME=focal
UBUNTU_CODENAME=focal
`),
	Dir(0755, "./usr/share/"),
	Dir(0755, "./usr/share/doc/"),
	Dir(0755, "./usr/share/doc/base-files/"),
	Reg(0644, "./usr/share/doc/base-files/copyright", `This is the Debian GNU/Linux prepackaged version of the Debian Base System
Miscellaneous files. These files were written by Ian Murdock
<imurdock@debian.org> and Bruce Perens <bruce@pixar.com>.

This package was first put together by Bruce Perens <Bruce@Pixar.com>,
from his own sources.

The GNU Public Licenses in /usr/share/common-licenses were taken from
ftp.gnu.org and are copyrighted by the Free Software Foundation, Inc.

The Artistic License in /usr/share/common-licenses is the one coming
from Perl and its SPDX name is "Artistic License 1.0 (Perl)".


Copyright (C) 1995-2011 Software in the Public Interest.

This program is free software; you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation; either version 2 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

On Debian GNU/Linux systems, the complete text of the GNU General
Public License can be found in `+"`/usr/share/common-licenses/GPL'.\n"),
	Dir(01777, "./tmp/"),
}

func init() {
	PackageData["base-files"] = MustMakeDeb(baseFilesPackageEntries)
}

type TarEntry struct {
	Header  tar.Header
	NoFixup bool
	Content []byte
}

var zeroTime time.Time
var epochStartTime time.Time = time.Unix(0, 0)

func fixupTarEntry(entry *TarEntry) {
	if entry.NoFixup {
		return
	}
	hdr := &entry.Header
	if hdr.Typeflag == 0 {
		if hdr.Linkname != "" {
			hdr.Typeflag = tar.TypeSymlink
		} else if strings.HasSuffix(hdr.Name, "/") {
			hdr.Typeflag = tar.TypeDir
		} else {
			hdr.Typeflag = tar.TypeReg
		}
	}
	if hdr.Mode == 0 {
		switch hdr.Typeflag {
		case tar.TypeDir:
			hdr.Mode = 0755
		case tar.TypeSymlink:
			hdr.Mode = 0777
		default:
			hdr.Mode = 0644
		}
	}
	if hdr.Size == 0 && entry.Content != nil {
		hdr.Size = int64(len(entry.Content))
	}
	if hdr.Uid == 0 && hdr.Uname == "" {
		hdr.Uname = "root"
	}
	if hdr.Gid == 0 && hdr.Gname == "" {
		hdr.Gname = "root"
	}
	if hdr.ModTime == zeroTime {
		hdr.ModTime = epochStartTime
	}
	if hdr.Format == 0 {
		hdr.Format = tar.FormatGNU
	}
}

func makeTar(entries []TarEntry) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, entry := range entries {
		fixupTarEntry(&entry)
		if err := tw.WriteHeader(&entry.Header); err != nil {
			return nil, err
		}
		if entry.Content != nil {
			if _, err := tw.Write(entry.Content); err != nil {
				return nil, err
			}
		}
	}
	return buf.Bytes(), nil
}

func compressBytesZstd(input []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer, err := zstd.NewWriter(&buf)
	if err != nil {
		return nil, err
	}
	if _, err = writer.Write(input); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func MakeDeb(entries []TarEntry) ([]byte, error) {
	var buf bytes.Buffer

	tarData, err := makeTar(entries)
	if err != nil {
		return nil, err
	}
	compTarData, err := compressBytesZstd(tarData)
	if err != nil {
		return nil, err
	}

	writer := ar.NewWriter(&buf)
	if err := writer.WriteGlobalHeader(); err != nil {
		return nil, err
	}
	dataHeader := ar.Header{
		Name: "data.tar.zst",
		Mode: 0644,
		Size: int64(len(compTarData)),
	}
	if err := writer.WriteHeader(&dataHeader); err != nil {
		return nil, err
	}
	if _, err = writer.Write(compTarData); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func MustMakeDeb(entries []TarEntry) []byte {
	data, err := MakeDeb(entries)
	if err != nil {
		panic(err)
	}
	return data
}

// Reg is a shortcut for creating a regular file TarEntry structure (with
// tar.Typeflag set tar.TypeReg). Reg stands for "REGular file".
func Reg(mode int64, path, content string) TarEntry {
	return TarEntry{
		Header: tar.Header{
			Typeflag: tar.TypeReg,
			Name:     path,
			Mode:     mode,
		},
		Content: []byte(content),
	}
}

// Dir is a shortcut for creating a directory TarEntry structure (with
// tar.Typeflag set to tar.TypeDir). Dir stands for "DIRectory".
func Dir(mode int64, path string) TarEntry {
	return TarEntry{
		Header: tar.Header{
			Typeflag: tar.TypeDir,
			Name:     path,
			Mode:     mode,
		},
	}
}

// Lnk is a shortcut for creating a symbolic link TarEntry structure (with
// tar.Typeflag set to tar.TypeSymlink). Lnk stands for "symbolic LiNK".
func Lnk(mode int64, path, target string) TarEntry {
	return TarEntry{
		Header: tar.Header{
			Typeflag: tar.TypeSymlink,
			Name:     path,
			Mode:     mode,
			Linkname: target,
		},
	}
}
