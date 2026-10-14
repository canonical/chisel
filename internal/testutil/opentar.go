package testutil

import (
	"archive/tar"
	"fmt"
	"io"

	"gopkg.in/check.v1"
)

func CheckOpenTar(c *check.C, openTar func() (io.ReadCloser, error)) {
	first, err := openTar()
	c.Assert(err, check.IsNil, check.Commentf("first call failed"))
	entries, err := readTarEntries(first)
	c.Assert(err, check.IsNil, check.Commentf("cannot read first stream"))
	err = first.Close()
	c.Assert(err, check.IsNil, check.Commentf("cannot close first stream"))

	second, err := openTar()
	c.Assert(err, check.IsNil, check.Commentf("second call failed"))
	entries2, err := readTarEntries(second)
	c.Assert(err, check.IsNil, check.Commentf("cannot read second stream"))
	err = second.Close()
	c.Assert(err, check.IsNil, check.Commentf("cannot close second stream"))

	c.Assert(entries2, check.DeepEquals, entries, check.Commentf("second stream differs from the first"))
}

// readTarEntries reads all entries of a tar stream into a comparable form.
func readTarEntries(tarStream io.Reader) ([]string, error) {
	tarReader := tar.NewReader(tarStream)
	var entries []string
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return entries, nil
		}
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, err
		}
		entries = append(entries, fmt.Sprintf("%c %s %o %q", header.Typeflag, header.Name, header.Mode, string(content)))
	}
}
