package setup

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"reflect"
)

const slicesDBPath = "/var/lib/chisel/chisel.db"

type ChiselDB struct {
	installedSlices []SliceKey
}

// Read and store the slices that have been already installed
func ReadDB(rootdir string) (*ChiselDB, error) {
	file, err := os.Open(path.Join(rootdir, slicesDBPath))
	if err != nil {
		if os.IsNotExist(err) {
			return &ChiselDB{}, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	db := ChiselDB{}

	for scanner.Scan() {
		line := scanner.Text()
		sliceKey, err := ParseSliceKey(line)
		if err != nil {
			return nil, err
		}
		db.installedSlices = append(db.installedSlices, sliceKey)
	}

	return &db, nil
}

// check if a slice has already been installed
func (db* ChiselDB) IsSliceInstalled(slice SliceKey) (bool) {
	for _, installedSlice := range db.installedSlices {
		if reflect.DeepEqual(installedSlice, slice) {
			return true
		}
	}
	return false
}

// append newly installed slices to the file and update the database
func (db *ChiselDB) WriteInstalledSlices(rootdir string, slices []*Slice) (error) {
	dbPath := path.Join(rootdir, slicesDBPath)

	err := os.MkdirAll(filepath.Dir(dbPath), 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	file, err := os.OpenFile(dbPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	for _, slice := range slices {
		_, err = writer.WriteString(slice.String() + "\n")
		if err != nil {
			return err
		}

		sliceKey, err := ParseSliceKey(slice.String())
		if err != nil {
			return err
		}
		db.installedSlices = append(db.installedSlices, sliceKey)
	}

	return nil
}