package setup

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"reflect"
)

const slicesDBPath = "/var/lib/chisel/chisel.db"

// Read and store the slices that have been already installed
var installedSlices []SliceKey

func ReadInstalledSlices(rootdir string) (error) {
	file, err := os.Open(path.Join(rootdir, slicesDBPath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		sliceKey, err := ParseSliceKey(line)
		if err != nil {
			return err
		}
		installedSlices = append(installedSlices, sliceKey)
	}

	return nil
}

func IsSliceInstalled(slice SliceKey) (bool) {
	for _, installedSlice := range installedSlices {
		if reflect.DeepEqual(installedSlice, slice) {
			return true
		}
	}
	return false
}

func WriteInstalledSlices(rootdir string, slices []*Slice) (error) {
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
	}

	return nil
}