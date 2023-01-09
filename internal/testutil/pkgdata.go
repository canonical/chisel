package testutil

import (
	"archive/tar"
	"bytes"
	"encoding/base64"
	"time"

	"github.com/blakesmith/ar"
	"github.com/klauspost/compress/zstd"
)

var PackageData = map[string][]byte{}

func init() {
	baseFilesData, err := base64.StdEncoding.DecodeString(baseFilesBase64)
	if err != nil {
		panic("broken package base64")
	}
	PackageData["base-files"] = baseFilesData
}

var baseFilesBase64 = `
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
`

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
		} else if hdr.Name[len(hdr.Name)-1] == '/' {
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
