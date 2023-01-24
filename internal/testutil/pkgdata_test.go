package testutil_test

import (
	"archive/tar"
	"bytes"
	"io"
	"time"

	"github.com/blakesmith/ar"
	"github.com/canonical/chisel/internal/testutil"
	"github.com/klauspost/compress/zstd"
	. "gopkg.in/check.v1"
)

type pkgdataSuite struct{}

var _ = Suite(&pkgdataSuite{})

type checkTarEntry struct {
	tarEntry    testutil.TarEntry
	checkHeader tar.Header
}

var epochStartTime time.Time = time.Unix(0, 0)

var pkgdataCheckEntries = []checkTarEntry{{
	testutil.TarEntry{
		Header: tar.Header{
			Name: "./",
		},
	},
	tar.Header{
		Typeflag: tar.TypeDir,
		Name:     "./",
		Mode:     00755,
		Uname:    "root",
		Gname:    "root",
		ModTime:  epochStartTime,
		Format:   tar.FormatGNU,
	},
}, {
	testutil.TarEntry{
		Header: tar.Header{
			Name:  "./admin/",
			Mode:  00700,
			Uname: "admin",
		},
	},
	tar.Header{
		Typeflag: tar.TypeDir,
		Name:     "./admin/",
		Mode:     00700,
		Uname:    "admin",
		Gname:    "root",
		ModTime:  epochStartTime,
		Format:   tar.FormatGNU,
	},
}, {
	testutil.TarEntry{
		Header: tar.Header{
			Name:  "./admin/password",
			Mode:  00600,
			Uname: "admin",
		},
		Content: []byte("swordf1sh"),
	},
	tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "./admin/password",
		Size:     9,
		Mode:     00600,
		Uname:    "admin",
		Gname:    "root",
		ModTime:  epochStartTime,
		Format:   tar.FormatGNU,
	},
}, {
	testutil.TarEntry{
		Header: tar.Header{
			Name:  "./admin/setpassword",
			Mode:  04711,
			Uname: "admin",
		},
		Content: []byte{0x7f, 0x45, 0x4c, 0x46, 0x02, 0x01, 0x01},
	},
	tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "./admin/setpassword",
		Size:     7,
		Mode:     04711,
		Uname:    "admin",
		Gname:    "root",
		ModTime:  epochStartTime,
		Format:   tar.FormatGNU,
	},
}, {
	testutil.TarEntry{
		Header: tar.Header{
			Name: "./data/",
		},
	},
	tar.Header{
		Typeflag: tar.TypeDir,
		Name:     "./data/",
		Mode:     00755,
		Uname:    "root",
		Gname:    "root",
		ModTime:  epochStartTime,
		Format:   tar.FormatGNU,
	},
}, {
	testutil.TarEntry{
		Header: tar.Header{
			Name: "./data/invoice.txt",
		},
		Content: []byte("$ 10"),
	},
	tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "./data/invoice.txt",
		Size:     4,
		Mode:     00644,
		Uname:    "root",
		Gname:    "root",
		ModTime:  epochStartTime,
		Format:   tar.FormatGNU,
	},
}, {
	testutil.TarEntry{
		Header: tar.Header{
			Name: "./data/logs/",
		},
	},
	tar.Header{
		Typeflag: tar.TypeDir,
		Name:     "./data/logs/",
		Mode:     00755,
		Uname:    "root",
		Gname:    "root",
		ModTime:  epochStartTime,
		Format:   tar.FormatGNU,
	},
}, {
	testutil.TarEntry{
		Header: tar.Header{
			Name:    "./data/logs/task.log",
			ModTime: time.Date(2022, 3, 1, 12, 0, 0, 0, time.Local),
		},
		Content: []byte("starting\nfinished\n"),
	},
	tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "./data/logs/task.log",
		Size:     18,
		Mode:     00644,
		Uname:    "root",
		Gname:    "root",
		ModTime:  time.Date(2022, 3, 1, 12, 0, 0, 0, time.Local),
		Format:   tar.FormatGNU,
	},
}, {
	testutil.TarEntry{
		Header: tar.Header{
			Name: "./data/shared/",
			Mode: 02777,
		},
	},
	tar.Header{
		Typeflag: tar.TypeDir,
		Name:     "./data/shared/",
		Mode:     02777,
		Uname:    "root",
		Gname:    "root",
		ModTime:  epochStartTime,
		Format:   tar.FormatGNU,
	},
}, {
	testutil.TarEntry{
		Header: tar.Header{
			Name: "./home/",
		},
	},
	tar.Header{
		Typeflag: tar.TypeDir,
		Name:     "./home/",
		Mode:     00755,
		Uname:    "root",
		Gname:    "root",
		ModTime:  epochStartTime,
		Format:   tar.FormatGNU,
	},
}, {
	testutil.TarEntry{
		Header: tar.Header{
			Name: "./home/alice/",
			Uid:  1000,
			Gid:  1000,
		},
	},
	tar.Header{
		Typeflag: tar.TypeDir,
		Name:     "./home/alice/",
		Mode:     00755,
		Uid:      1000,
		Gid:      1000,
		ModTime:  epochStartTime,
		Format:   tar.FormatGNU,
	},
}, {
	testutil.TarEntry{
		Header: tar.Header{
			Name: "./home/alice/notes",
			Uid:  1000,
		},
		Content: []byte("check the cat"),
	},
	tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "./home/alice/notes",
		Size:     13,
		Mode:     00644,
		Uid:      1000,
		Gname:    "root",
		ModTime:  epochStartTime,
		Format:   tar.FormatGNU,
	},
}, {
	testutil.TarEntry{
		Header: tar.Header{
			Name:  "./home/bob/",
			Uname: "bob",
			Uid:   1001,
		},
	},
	tar.Header{
		Typeflag: tar.TypeDir,
		Name:     "./home/bob/",
		Mode:     00755,
		Uid:      1001,
		Uname:    "bob",
		Gname:    "root",
		ModTime:  epochStartTime,
		Format:   tar.FormatGNU,
	},
}, {
	testutil.TarEntry{
		Header: tar.Header{
			Name:  "./home/bob/task.sh",
			Mode:  00700,
			Uname: "bob",
			Uid:   1001,
		},
		Content: []byte("#!/bin/sh\n"),
	},
	tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "./home/bob/task.sh",
		Size:     10,
		Mode:     00700,
		Uid:      1001,
		Uname:    "bob",
		Gname:    "root",
		ModTime:  epochStartTime,
		Format:   tar.FormatGNU,
	},
}, {
	testutil.TarEntry{
		Header: tar.Header{
			Name:     "./logs/",
			Linkname: "data/logs",
		},
	},
	tar.Header{
		Typeflag: tar.TypeSymlink,
		Name:     "./logs/",
		Linkname: "data/logs",
		Mode:     00777,
		Uname:    "root",
		Gname:    "root",
		ModTime:  epochStartTime,
		Format:   tar.FormatGNU,
	},
}, {
	testutil.TarEntry{
		Header: tar.Header{
			Typeflag: tar.TypeFifo,
			Name:     "./pipe",
		},
	},
	tar.Header{
		Typeflag: tar.TypeFifo,
		Name:     "./pipe",
		Mode:     00644,
		Uname:    "root",
		Gname:    "root",
		ModTime:  epochStartTime,
		Format:   tar.FormatGNU,
	},
}, {
	testutil.TarEntry{
		Header: tar.Header{
			Typeflag: tar.TypeReg,
			Name:     "./restricted.txt",
			Size:     3,
			ModTime:  epochStartTime,
			Format:   tar.FormatGNU,
		},
		Content: []byte("123"),
		NoFixup: true,
	},
	tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "./restricted.txt",
		Size:     3,
		ModTime:  epochStartTime,
		Format:   tar.FormatGNU,
	},
}}

func (s *pkgdataSuite) TestMakeDeb(c *C) {
	var size int64
	var err error

	inputEntries := make([]testutil.TarEntry, len(pkgdataCheckEntries))
	for i, checkEntry := range pkgdataCheckEntries {
		inputEntries[i] = checkEntry.tarEntry
	}
	debBytes, err := testutil.MakeDeb(inputEntries)
	c.Assert(err, IsNil)

	debBuf := bytes.NewBuffer(debBytes)
	arReader := ar.NewReader(debBuf)

	arHeader, err := arReader.Next()
	c.Assert(err, IsNil)
	c.Assert(arHeader.Name, Equals, "data.tar.zst")
	c.Assert(arHeader.Mode, Equals, int64(0644))
	c.Assert(int(arHeader.Size), testutil.IntGreaterThan, 0)

	var tarZstdBuf bytes.Buffer
	size, err = io.Copy(&tarZstdBuf, arReader)
	c.Assert(err, IsNil)
	c.Assert(int(size), testutil.IntGreaterThan, 0)

	var tarBuf bytes.Buffer
	zstdReader, err := zstd.NewReader(&tarZstdBuf)
	size, err = zstdReader.WriteTo(&tarBuf)
	c.Assert(err, IsNil)
	c.Assert(int(size), testutil.IntGreaterThan, 0)

	tarReader := tar.NewReader(&tarBuf)

	for _, checkEntry := range pkgdataCheckEntries {
		tarHeader, err := tarReader.Next()
		c.Assert(err, IsNil)
		c.Assert(*tarHeader, DeepEquals, checkEntry.checkHeader)
		var dataBuf bytes.Buffer
		size, err = io.Copy(&dataBuf, tarReader)
		c.Assert(err, IsNil)
		if checkEntry.tarEntry.Content != nil {
			c.Assert(dataBuf.Bytes(), DeepEquals, checkEntry.tarEntry.Content)
		} else {
			c.Assert(int(size), Equals, 0)
		}
	}

	_, err = tarReader.Next()
	c.Assert(err, Equals, io.EOF)

	_, err = arReader.Next()
	c.Assert(err, Equals, io.EOF)
}
