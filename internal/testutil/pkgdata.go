package testutil

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/blakesmith/ar"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

var PackageData = map[string][]byte{}

func init() {
	for name, pkg := range pkgs {
		pkg.init(name)
		data, err := pkg.buildDeb()
		if err != nil {
			panic(err)
		}
		PackageData[name] = data
	}
}

type tarEntry struct {
	name     string
	mode     int64
	linkname string

	// Use this field for complete definition of tar header. If it is not
	// nil, it is used verbatim (except Size) and name, mode and linkname
	// fields are ignored.
	header *tar.Header

	source  string
	content []byte
}

type pkgDef struct {
	sourceDir   string
	compressExt string // "gz", "xz", "zst" or empty, empty means "zst"
	readSource  func(pkg *pkgDef, path string) ([]byte, error)
	tarTemplate tar.Header
	entries     []tarEntry
}

var defaultTarTemplate = tar.Header{
	Uid:     0,
	Gid:     0,
	Uname:   "root",
	Gname:   "root",
	ModTime: time.Unix(0, 0),
	Format:  tar.FormatGNU,
}

var pkgs = map[string]pkgDef{
	"base-files": pkgDef{
		entries: []tarEntry{
			{name: "./"},
			{name: "./bin/"},
			{name: "./boot/"},
			{name: "./dev/"},
			{name: "./etc/"},
			{name: "./etc/debian_version"},
			{name: "./etc/default/"},
			{name: "./etc/dpkg/"},
			{name: "./etc/dpkg/origins/"},
			{name: "./etc/dpkg/origins/debian"},
			{name: "./etc/dpkg/origins/ubuntu"},
			{name: "./etc/host.conf"},
			{name: "./etc/issue"},
			{name: "./etc/issue.net"},
			{name: "./etc/legal"},
			{name: "./etc/lsb-release"},
			{name: "./etc/profile.d/"},
			{name: "./etc/profile.d/01-locale-fix.sh"},
			{name: "./etc/skel/"},
			{name: "./etc/update-motd.d/"},
			{name: "./etc/update-motd.d/00-header", mode: 00755},
			{name: "./etc/update-motd.d/10-help-text", mode: 00755},
			{name: "./etc/update-motd.d/50-motd-news", mode: 00755},
			{name: "./home/"},
			{name: "./lib/"},
			{name: "./lib/systemd/"},
			{name: "./lib/systemd/system/"},
			{name: "./lib/systemd/system/motd-news.service"},
			{name: "./lib/systemd/system/motd-news.timer"},
			{name: "./proc/"},
			{name: "./root/", mode: 00700},
			{name: "./run/"},
			{name: "./sbin/"},
			{name: "./sys/"},
			{name: "./tmp/", mode: 01777},
			{name: "./usr/"},
			{name: "./usr/bin/"},
			{name: "./usr/bin/hello", mode: 00775},
			{name: "./usr/games/"},
			{name: "./usr/include/"},
			{name: "./usr/lib/"},
			{name: "./usr/lib/os-release"},
			{name: "./usr/sbin/"},
			{name: "./usr/share/"},
			{name: "./usr/share/dict/"},
			{name: "./usr/share/doc/"},
			{name: "./usr/share/doc/base-files/"},
			{name: "./usr/share/doc/base-files/copyright"},
			{name: "./usr/share/info/"},
			{name: "./usr/share/man/"},
			{name: "./usr/share/misc/"},
			{name: "./usr/src/"},
			{name: "./var/"},
			{name: "./var/backups/"},
			{name: "./var/cache/"},
			{name: "./var/lib/"},
			{name: "./var/lib/dpkg/"},
			{name: "./var/lib/misc/"},
			{name: "./var/local/", mode: 02775},
			{name: "./var/lock/", mode: 01777},
			{name: "./var/log/"},
			{name: "./var/run/"},
			{name: "./var/spool/"},
			{name: "./var/tmp/", mode: 01777},
			{name: "./etc/os-release", linkname: "../usr/lib/os-release"},
		},
	},
	"copyright-symlink-libssl3": pkgDef{
		entries: []tarEntry{
			{name: "./"},
			{name: "./usr/"},
			{name: "./usr/lib/"},
			{name: "./usr/lib/x86_64-linux-gnu/"},
			{name: "./usr/lib/x86_64-linux-gnu/libssl.so.3", mode: 00755, content: []byte{}},
			{name: "./usr/share/"},
			{name: "./usr/share/doc/"},
			{name: "./usr/share/doc/copyright-symlink-libssl3/"},
			{name: "./usr/share/doc/copyright-symlink-libssl3/copyright", content: []byte{}},
		},
	},
	"copyright-symlink-openssl": pkgDef{
		entries: []tarEntry{
			{name: "./"},
			{name: "./etc/"},
			{name: "./etc/ssl/"},
			{name: "./etc/ssl/openssl.cnf", content: []byte{}},
			{name: "./usr/"},
			{name: "./usr/bin/"},
			{name: "./usr/bin/openssl", mode: 00755, content: []byte{}},
			{name: "./usr/share/"},
			{name: "./usr/share/doc/"},
			{name: "./usr/share/doc/copyright-symlink-openssl/"},
			{name: "./usr/share/doc/copyright-symlink-openssl/copyright", linkname: "../libssl3/copyright"},
		},
	},
}

//go:embed all:pkgdata
var pkgdataFS embed.FS

func (pkg *pkgDef) init(pkgName string) error {
	if pkg.tarTemplate.Format == 0 {
		pkg.tarTemplate = defaultTarTemplate
	}
	if pkg.sourceDir == "" {
		pkg.sourceDir = "pkgdata/" + pkgName
	}
	if pkg.readSource == nil {
		pkg.readSource = func(pkg *pkgDef, path string) ([]byte, error) {
			localPath := filepath.Join(pkg.sourceDir, path)
			return pkgdataFS.ReadFile(localPath)
		}
	}
	switch pkg.compressExt {
	case "gz", "xz", "zst":
	case "":
		pkg.compressExt = "zst"
	default:
		return fmt.Errorf("unknown compression: %s", pkg.compressExt)
	}
	return nil
}

func (pkg *pkgDef) writeTarEntry(tw *tar.Writer, entry *tarEntry) error {
	var err error
	header := entry.header

	if header == nil {
		newHeader := pkg.tarTemplate
		header = &newHeader

		header.Typeflag = tar.TypeReg
		header.Name = entry.name
		header.Linkname = entry.linkname
		header.Mode = entry.mode & 01777

		if entry.linkname != "" {
			// use entry.header for hardlinks
			header.Typeflag = tar.TypeSymlink
		} else if entry.name[len(entry.name)-1] == '/' {
			header.Typeflag = tar.TypeDir
		}

		// use entry.header or bits above 1777 for zero mode
		if entry.mode == 0 {
			switch header.Typeflag {
			case tar.TypeLink:
				header.Mode = 0777
			case tar.TypeDir:
				header.Mode = 0755
			default:
				header.Mode = 0644
			}
		}
	}

	var content []byte

	if header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA {
		content = entry.content

		if content == nil {
			sourcePath := entry.source
			if sourcePath == "" {
				sourcePath = header.Name
			}
			if content, err = pkg.readSource(pkg, sourcePath); err != nil {
				return err
			}
		}

		header.Size = int64(len(content))

	}

	if err = tw.WriteHeader(header); err != nil {
		return err
	}

	if content != nil {
		if _, err = tw.Write(content); err != nil {
			return err
		}
	}

	return nil
}

func (pkg *pkgDef) makeTar(entries []tarEntry) ([]byte, error) {
	var err error
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	for _, entry := range entries {
		if err = pkg.writeTarEntry(tw, &entry); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func (pkg *pkgDef) makeCompressed(input []byte) ([]byte, error) {
	var err error
	var buf bytes.Buffer
	var writer io.WriteCloser

	switch pkg.compressExt {
	case "gz":
		writer = gzip.NewWriter(&buf)
	case "xz":
		writer, err = xz.NewWriter(&buf)
	case "zst":
		writer, err = zstd.NewWriter(&buf)
	}
	if err != nil {
		return nil, err
	}

	if _, err = writer.Write(input); err != nil {
		return nil, err
	}

	writer.Close()

	return buf.Bytes(), nil
}

func (pkg *pkgDef) makeDeb(input []byte) ([]byte, error) {
	var err error
	var buf bytes.Buffer
	writer := ar.NewWriter(&buf)

	headerTemplate := ar.Header{
		ModTime: time.Unix(0, 0),
		Mode:    0644,
	}

	if err = writer.WriteGlobalHeader(); err != nil {
		return nil, err
	}

	markerContent := []byte("2.0\n")
	markerHeader := headerTemplate
	markerHeader.Name = "debian-binary"
	markerHeader.Size = int64(len(markerContent))

	if err = writer.WriteHeader(&markerHeader); err != nil {
		return nil, err
	}
	if _, err := writer.Write(markerContent); err != nil {
		return nil, err
	}

	// currently unused (invalid) dummy control file entry
	controlHeader := headerTemplate
	controlHeader.Name = "control.tar." + pkg.compressExt

	if err = writer.WriteHeader(&controlHeader); err != nil {
		return nil, err
	}

	dataHeader := headerTemplate
	dataHeader.Name = "data.tar." + pkg.compressExt
	dataHeader.Size = int64(len(input))

	if err = writer.WriteHeader(&dataHeader); err != nil {
		return nil, err
	}
	if _, err = writer.Write([]byte(input)); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (pkg *pkgDef) buildDeb() ([]byte, error) {
	data, err := pkg.makeTar(pkg.entries)
	if err != nil {
		return nil, err
	}
	data, err = pkg.makeCompressed(data)
	if err != nil {
		return nil, err
	}
	data, err = pkg.makeDeb(data)
	if err != nil {
		return nil, err
	}
	return data, nil
}
