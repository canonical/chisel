package testarchive

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"path"
	"strings"

	"github.com/canonical/chisel/internal/testutil"
)

type Item interface {
	Path() string
	Walk(f func(Item) error) error
	Section() []byte
	Content() []byte
}

func CallWalkFunc(this Item, f func(Item) error, items ...Item) error {
	if this != nil {
		err := f(this)
		if err != nil {
			return err
		}
	}
	for _, item := range items {
		err := item.Walk(f)
		if err != nil {
			return err
		}
	}
	return nil
}

type Gzip struct {
	Item Item
}

func (gz *Gzip) Path() string {
	return gz.Item.Path() + ".gz"
}

func (gz *Gzip) Walk(f func(Item) error) error {
	return CallWalkFunc(gz, f, gz.Item)
}

func (gz *Gzip) Section() []byte {
	return gz.Item.Section()
}

func (gz *Gzip) Content() []byte {
	return makeGzip(gz.Item.Content())
}

type Package struct {
	Name      string
	Version   string
	Arch      string
	Component string
	Data      []byte
}

func (p *Package) Path() string {
	return fmt.Sprintf("pool/%s/%c/%s/%s_%subuntu1_%s.deb", p.Component, p.Name[0], p.Name, p.Name, p.Version, p.Arch)
}

func (p *Package) Walk(f func(Item) error) error {
	return CallWalkFunc(p, f)
}

func (p *Package) Section() []byte {
	content := p.Content()
	section := fmt.Sprintf(string(testutil.Reindent(`
		Package: %s
		Architecture: %s
		Version: %s
		Priority: required
		Essential: yes
		Section: admin
		Origin: Ubuntu
		Installed-Size: 10
		Filename: %s
		Size: %d
		SHA256: %s
		Description: Description of %s
		Task: minimal

	`)), p.Name, p.Arch, p.Version, p.Path(), len(content), makeSha256(content), p.Name)
	return []byte(section)
}

func (p *Package) Content() []byte {
	if len(p.Data) == 0 {
		return []byte(p.Name + " " + p.Version + " data")
	}
	return p.Data
}

type Release struct {
	Suite   string
	Version string
	Items   []Item
}

func (r *Release) Walk(f func(Item) error) error {
	return CallWalkFunc(r, f, r.Items...)
}

func (r *Release) Path() string {
	return "Release"
}

func (r *Release) Section() []byte {
	return nil
}

func (r *Release) Content() []byte {
	digests := bytes.Buffer{}
	for _, item := range r.Items {
		content := item.Content()
		digests.WriteString(fmt.Sprintf(" %s  %d  %s\n", makeSha256(content), len(content), item.Path()))
	}
	content := fmt.Sprintf(string(testutil.Reindent(`
		Origin: Ubuntu
		Label: Ubuntu
		Suite: %s
		Version: %s
		Codename: codename
		Date: Thu, 21 Apr 2022 17:16:08 UTC
		Architectures: amd64 arm64 armhf i386 ppc64el riscv64 s390x
		Components: main restricted universe multiverse
		Description: Ubuntu %s
		SHA256:
		%s
	`)), r.Suite, r.Version, r.Version, digests.String())

	return []byte(content)
}

func (r *Release) Render(prefix string, content map[string][]byte) error {
	return r.Walk(func(item Item) error {
		itemPath := item.Path()
		if strings.HasPrefix(itemPath, "pool/") {
			itemPath = path.Join(prefix, itemPath)
		} else {
			itemPath = path.Join(prefix, "dists", r.Suite, itemPath)
		}
		content[itemPath] = item.Content()
		return nil
	})
}

func MergeSections(items []Item) []byte {
	buf := bytes.Buffer{}
	for _, item := range items {
		buf.Write(item.Section())
	}
	return buf.Bytes()
}

type PackageIndex struct {
	Component string
	Arch      string
	Packages  []Item
}

func (pi *PackageIndex) Path() string {
	return fmt.Sprintf("%s/binary-%s/Packages", pi.Component, pi.Arch)
}

func (pi *PackageIndex) Walk(f func(Item) error) error {
	return CallWalkFunc(pi, f, pi.Packages...)
}

func (pi *PackageIndex) Section() []byte {
	return nil
}

func (pi *PackageIndex) Content() []byte {
	return MergeSections(pi.Packages)
}

func makeSha256(b []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

func makeGzip(b []byte) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write(b)
	if err != nil {
		panic(err)
	}
	err = gz.Close()
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}
