package testutil

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
)

func TreeDump(dir string) map[string]string {
	result := make(map[string]string)
	dirfs := os.DirFS(dir)
	err := fs.WalkDir(dirfs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk error: %w", err)
		}
		if path == "." {
			return nil
		}
		finfo, err := d.Info()
		if err != nil {
			return fmt.Errorf("cannot get stat info for %q: %w", path, err)
		}
		fperm := finfo.Mode() & fs.ModePerm
		ftype := finfo.Mode() & fs.ModeType
		if finfo.Mode()&fs.ModeSticky != 0 {
			fperm |= 01000
		}
		fpath := filepath.Join(dir, path)
		switch ftype {
		case fs.ModeDir:
			result["/"+path+"/"] = fmt.Sprintf("dir %#o", fperm)
		case fs.ModeSymlink:
			lpath, err := os.Readlink(fpath)
			if err != nil {
				return err
			}
			result["/"+path] = fmt.Sprintf("symlink %s", lpath)
		case 0: // Regular
			data, err := ioutil.ReadFile(fpath)
			if err != nil {
				return fmt.Errorf("cannot read file: %w", err)
			}
			var entry string
			if len(data) == 0 {
				entry = fmt.Sprintf("file %#o empty", fperm)
			} else {
				sum := sha256.Sum256(data)
				entry = fmt.Sprintf("file %#o %.4x", fperm, sum)
			}
			result["/"+path] = entry
		default:
			return fmt.Errorf("unknown file type %d: %s", ftype, fpath)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return result
}
