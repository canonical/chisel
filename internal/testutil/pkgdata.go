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
	Dir(0755, "./a1/"),
	Reg(0644, "./a1/f1", "a1f1"),
	Reg(0644, "./a1/f2", "a1f2"),
	Dir(0755, "./a1/b1/"),
	Reg(0644, "./a1/b1/f1", "a1b1f1"),
	Reg(0644, "./a1/b1/f2", "a1b1f2"),
	Dir(0755, "./a1/b1/c1/"),
	Reg(0644, "./a1/b1/c1/f1", "a1b1c1f1"),
	Dir(0755, "./a2/"),
	Dir(0755, "./a2/b1/"),
	Reg(0644, "./a2/b1/f1", "a2b1f1"),
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
