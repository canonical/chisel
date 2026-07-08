package testarchive

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"path"
	"slices"
	"strings"

	"golang.org/x/crypto/openpgp/clearsign"
	"golang.org/x/crypto/openpgp/packet"

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
	return MakeGzip(gz.Item.Content())
}

type Package struct {
	Name      string
	Version   string
	Arch      string
	Component string
	Data      []byte
	// release is set when a release adopts this package at render time.
	// Section reads the archive-wide digest kinds through it, so the package
	// always sees the release's current choice.
	release *Release
}

func (p *Package) Path() string {
	return fmt.Sprintf("pool/%s/%c/%s/%s_%subuntu1_%s.deb", p.Component, p.Name[0], p.Name, p.Name, p.Version, p.Arch)
}

func (p *Package) Walk(f func(Item) error) error {
	return CallWalkFunc(p, f)
}

func (p *Package) Section() []byte {
	if p.release == nil {
		panic("package section rendered before a release adopted the package; render via Release.Render")
	}
	content := p.Content()
	digests := strings.Builder{}
	for i, kind := range p.release.DigestKinds {
		if i > 0 {
			digests.WriteByte('\n')
		}
		fmt.Fprintf(&digests, "%s: %s", kind, makeDigest(kind, content))
	}
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
		%s
		Description: Description of %s
		Task: minimal

	`)), p.Name, p.Arch, p.Version, p.Path(), len(content), digests.String(), p.Name)
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
	Label   string
	Items   []Item
	PrivKey *packet.PrivateKey
	// DigestKinds names the digest kinds published across this archive: the
	// index table and every package it publishes. Callers must set it
	// explicitly; an empty list publishes no digest sections at all.
	DigestKinds []string
	// ByHash enables the Acquire-By-Hash flag in the Release file
	// and renders by-hash URLs alongside named paths.
	ByHash bool
}

func (r *Release) Walk(f func(Item) error) error {
	return CallWalkFunc(r, f, r.Items...)
}

func (r *Release) Path() string {
	return "InRelease"
}

func (r *Release) Section() []byte {
	return nil
}

// adoptPackages points every published package back at this release, giving
// the package sections access to the archive-wide digest kinds. Render calls
// it before walking the items.
func (r *Release) adoptPackages() error {
	return r.Walk(func(item Item) error {
		if p, ok := item.(*Package); ok {
			p.release = r
		}
		return nil
	})
}

func (r *Release) Content() []byte {
	digests := bytes.Buffer{}
	for _, kind := range r.DigestKinds {
		fmt.Fprintf(&digests, "%s:\n", kind)
		for _, item := range r.Items {
			content := item.Content()
			fmt.Fprintf(&digests, " %s  %d  %s\n", makeDigest(kind, content), len(content), item.Path())
		}
	}
	acquireByHash := ""
	if r.ByHash {
		acquireByHash = "Acquire-By-Hash: yes\n"
	}
	content := fmt.Sprintf(string(testutil.Reindent(`
		Origin: Ubuntu
		Label: %s
		Suite: %s
		Version: %s
		Codename: codename
		Date: Thu, 21 Apr 2022 17:16:08 UTC
		Architectures: amd64 arm64 armhf i386 ppc64el riscv64 s390x
		Components: main restricted universe multiverse
		Description: Ubuntu %s
		%s%s
	`)), r.Label, r.Suite, r.Version, r.Version, acquireByHash, digests.String())

	var buf bytes.Buffer
	writer, err := clearsign.Encode(&buf, r.PrivKey, nil)
	if err != nil {
		panic(err)
	}
	_, err = writer.Write([]byte(content))
	if err != nil {
		panic(err)
	}
	err = writer.Close()
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func (r *Release) Render(prefix string, content map[string][]byte) error {
	err := r.adoptPackages()
	if err != nil {
		return err
	}
	return r.Walk(func(item Item) error {
		itemPath := item.Path()
		itemContent := item.Content()
		if strings.HasPrefix(itemPath, "pool/") {
			content[path.Join(prefix, itemPath)] = itemContent
			return nil
		}
		distItemPath := path.Join(prefix, "dists", r.Suite, itemPath)
		content[distItemPath] = itemContent
		if r.ByHash && itemPath != r.Path() {
			// Real archives (ftpmaster) only publish by-hash directories for
			// the strongest hash they advertise; mirror that so tests catch
			// clients building by-hash URLs from a weaker hash.
			if kind := strongestKind(r.DigestKinds); kind != "" {
				byHashPath := path.Join(prefix, "dists", r.Suite, path.Dir(itemPath), "by-hash", kind, makeDigest(kind, itemContent))
				content[byHashPath] = itemContent
			}
		}
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

// digestKinds lists the digest kinds this package can render, strongest first.
var digestKinds = []string{"SHA512", "SHA256"}

// strongestKind returns the strongest of kinds, or "" if kinds holds none of
// digestKinds.
func strongestKind(kinds []string) string {
	for _, kind := range digestKinds {
		if slices.Contains(kinds, kind) {
			return kind
		}
	}
	return ""
}

func makeDigest(kind string, b []byte) string {
	switch kind {
	case "SHA512":
		return fmt.Sprintf("%x", sha512.Sum512(b))
	case "SHA256":
		return fmt.Sprintf("%x", sha256.Sum256(b))
	}
	panic("unknown digest kind: " + kind)
}

func MakeGzip(b []byte) []byte {
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
