package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/canonical/chisel/internal/jsonwall"
	"github.com/klauspost/compress/zstd"
)

const schema = "0.1"

// New creates a new Chisel DB writer with the proper schema.
func New() *jsonwall.DBWriter {
	options := jsonwall.DBWriterOptions{Schema: schema}
	return jsonwall.NewDBWriter(&options)
}

func getDBPath(root string) string {
	return filepath.Join(root, ".chisel.db")
}

// Save uses the provided writer dbw to write the Chisel DB into the standard
// path under the provided root directory.
func Save(dbw *jsonwall.DBWriter, root string) (err error) {
	dbPath := getDBPath(root)
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot save state to %q: %w", dbPath, err)
		}
	}()
	f, err := os.OpenFile(dbPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	// chmod the existing file
	if err = f.Chmod(0644); err != nil {
		return
	}
	zw, err := zstd.NewWriter(f)
	if err != nil {
		return
	}
	if _, err = dbw.WriteTo(zw); err != nil {
		return
	}
	return zw.Close()
}

// Load reads a Chisel DB from the standard path under the provided root
// directory. If the Chisel DB doesn't exist, the returned error satisfies
// errors.Is(err, fs.ErrNotExist))
func Load(root string) (db *jsonwall.DB, err error) {
	dbPath := getDBPath(root)
	defer func() {
		if err != nil {
			err = fmt.Errorf("cannot load state from %q: %w", dbPath, err)
		}
	}()
	f, err := os.Open(dbPath)
	if err != nil {
		return
	}
	defer f.Close()
	zr, err := zstd.NewReader(f)
	if err != nil {
		return
	}
	defer zr.Close()
	db, err = jsonwall.ReadDB(zr)
	if err != nil {
		return nil, err
	}
	if s := db.Schema(); s != schema {
		return nil, fmt.Errorf("invalid schema %#v", s)
	}
	return
}
