package testutil

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/canonical/chisel/internal/fsutil"
)

func TreeDump(dir string) map[string]string {
	var inodes []uint64
	pathsByInodes := make(map[uint64][]string)
	result := make(map[string]string)
	dirfs := os.DirFS(dir)
	err := fs.WalkDir(dirfs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk error: %w", err)
		}
		if path == "." {
			return nil
		}
		fpath := filepath.Join(dir, path)
		finfo, err := d.Info()
		if err != nil {
			return fmt.Errorf("cannot get stat info for %q: %w", fpath, err)
		}
		fperm := finfo.Mode() & fs.ModePerm
		ftype := finfo.Mode() & fs.ModeType
		if finfo.Mode()&fs.ModeSticky != 0 {
			fperm |= 01000
		}
		var resultEntry string
		switch ftype {
		case fs.ModeDir:
			path = "/" + path + "/"
			resultEntry = fmt.Sprintf("dir %#o", fperm)
		case fs.ModeSymlink:
			lpath, err := os.Readlink(fpath)
			if err != nil {
				return err
			}
			path = "/" + path
			resultEntry = fmt.Sprintf("symlink %s", lpath)
		case 0: // Regular
			data, err := os.ReadFile(fpath)
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
			path = "/" + path
			resultEntry = entry
		default:
			return fmt.Errorf("unknown file type %d: %s", ftype, fpath)
		}
		result[path] = resultEntry
		if ftype != fs.ModeDir {
			stat, ok := finfo.Sys().(*syscall.Stat_t)
			if !ok {
				return fmt.Errorf("cannot get syscall stat info for %q", fpath)
			}
			inode := stat.Ino
			if len(pathsByInodes[inode]) == 1 {
				inodes = append(inodes, inode)
			}
			pathsByInodes[inode] = append(pathsByInodes[inode], path)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}

	// Append identifiers to paths who share an inode e.g. hard links.
	for i := 0; i < len(inodes); i++ {
		paths := pathsByInodes[inodes[i]]
		for _, path := range paths {
			result[path] = fmt.Sprintf("%s <%d>", result[path], i+1)
		}
	}
	return result
}

// TreeDumpEntry the file information in the same format as [testutil.TreeDump].
func TreeDumpEntry(entry *fsutil.Entry) string {
	fperm := entry.Mode.Perm()
	if entry.Mode&fs.ModeSticky != 0 {
		fperm |= 01000
	}
	switch entry.Mode.Type() {
	case fs.ModeDir:
		return fmt.Sprintf("dir %#o", fperm)
	case fs.ModeSymlink:
		return fmt.Sprintf("symlink %s", entry.Link)
	case 0:
		// Regular file.
		if entry.Size == 0 {
			return fmt.Sprintf("file %#o empty", entry.Mode.Perm())
		} else {
			return fmt.Sprintf("file %#o %s", fperm, entry.SHA256[:8])
		}
	default:
		panic(fmt.Errorf("unknown file type %d: %s", entry.Mode.Type(), entry.Path))
	}
}
