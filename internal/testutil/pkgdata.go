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

var TestPackageEntries = []TarEntry{
	Dir(0755, "./"),
	Dir(0755, "./dir/"),
	Reg(0644, "./dir/file", "12u3q0wej	ajsd"),
	Reg(0644, "./dir/other-file", "kasjdf0"),
	Dir(0755, "./dir/nested/"),
	Reg(0644, "./dir/nested/file", "0jqei"),
	Reg(0644, "./dir/nested/other-file", "1"),
	Dir(0755, "./dir/several/"),
	Dir(0755, "./dir/several/levels/"),
	Dir(0755, "./dir/several/levels/deep/"),
	Reg(0644, "./dir/several/levels/deep/file", "129i381		"),
	Dir(0755, "./other-dir/"),
	Dir(01777, "./parent/"),
	Dir(0764, "./parent/permissions/"),
	Reg(0755, "./parent/permissions/file", "ajse0"),
}

var OtherPackageEntries = []TarEntry{
	Dir(0755, "./"),
	Reg(0644, "./file", "masfdko"),
}

func init() {
	PackageData["test-package"] = MustMakeDeb(TestPackageEntries)
	PackageData["other-package"] = MustMakeDeb(OtherPackageEntries)
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
