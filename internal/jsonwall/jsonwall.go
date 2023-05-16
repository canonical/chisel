//
// Package jsonwall provides an interface to work with database files in the simple
// "jsonwall" format, which consists of a text file with one JSON object per line,
// where both the individual JSON fields and the lines themselves are sorted to
// optimize for searching and iteration.
//
// For example, the following content represents a valid jsonwall database:
//
//     {"jsonwall":"1.0","count":3}
//     {"kind":"app","name":"chisel","version":"1.0"}
//     {"kind":"app","name":"pebble","version":"1.2"}
//
// The entries in this database might be manipulated with a type such as:
//
//     type AppEntry struct {
//             Kind string    `json:"kind"`
//             Name string    `json:"name,omitempty"`
//             Version string `json:"version,omitempty"`
//     }
//
// Such data types have two important characteristics: fields must be defined in
// the order that will be used when searching, and every optional field must be
// tagged with `omitempty`.
//
// With that in place, the database may be accessed as:
//
//     app := AppEntry{Kind: "app", Name: "chisel"}
//     if db.Get(&app) == nil {
//             fmt.Println(app.Name, "version:", app.Version)
//     }
//
// Iteration works similarly:
//
//     app := AppEntry{Kind: "app"}
//     if iter, err := db.Iter(&app); err == nil {
//             for iter.Next() {
//                     if iter.Get(&app) == nil {
//                             fmt.Println(app.Name, "version:", app.Version)
//                     }
//             }
//     }
//
package jsonwall

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"sort"
	"strings"
)

// DBWriter holds in memory the state of a database while it's being prepared
// for serialization and implements the WriterTo interface for assembling it.
type DBWriter struct {
	options *DBWriterOptions
	entries [][]byte
}

type DBWriterOptions struct {
	// Schema is included in the database header to help the decoding
	// process. The value is made available when reading, and is not
	// internally interpreted.
	Schema string
}

// NewDBWriter returns a database writer that can assemble new databases.
func NewDBWriter(options *DBWriterOptions) *DBWriter {
	if options == nil {
		options = &DBWriterOptions{}
	}
	return &DBWriter{options: options}
}

// Add encodes the provided value as a JSON object and includes the resulting
// data into the database being created.
func (dbw *DBWriter) Add(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data) == 0 || data[0] != '{' {
		return fmt.Errorf("invalid database value: %#v", value)
	}
	dbw.entries = append(dbw.entries, data)
	return nil
}

type sortableEntries [][]byte

func (e sortableEntries) Less(i, j int) bool { return bytes.Compare(e[i], e[j]) < 0 }
func (e sortableEntries) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }
func (e sortableEntries) Len() int           { return len(e) }

type jsonwallHeader struct {
	Version string `json:"jsonwall"`
	Schema  string `json:"schema,omitempty"`
	Count   int    `json:"count"`
}

const jsonwallVersion = "1.0"

func (dbw *DBWriter) writeHeader(w io.Writer, count int) (int, error) {
	data, err := json.Marshal(&jsonwallHeader{
		Version: jsonwallVersion,
		Schema:  dbw.options.Schema,
		Count:   count,
	})
	if err != nil {
		return 0, fmt.Errorf("internal error: cannot marshal database header: %w", err)
	}
	return w.Write(append(data, '\n'))
}

// WriteTo assembles the current database state and writes it to w.
func (dbw *DBWriter) WriteTo(w io.Writer) (n int64, err error) {
	m, err := dbw.writeHeader(w, len(dbw.entries)+1)
	n += int64(m)
	if err != nil {
		return n, err
	}
	sort.Sort(sortableEntries(dbw.entries))
	for _, entry := range dbw.entries {
		m, err := w.Write(entry)
		n += int64(m)
		if err == nil {
			m, err = w.Write([]byte{'\n'})
			n += int64(m)
		}
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// ReadDB reads into memory the database from the provided r.
func ReadDB(r io.Reader) (*DB, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	record := 0
	for i := range data {
		if data[i] == '\n' {
			record = i + 1
			break
		}
	}
	var header jsonwallHeader
	err = json.Unmarshal(data[:record], &header)
	if err != nil {
		return nil, fmt.Errorf("invalid database content")
	}
	if !strings.HasPrefix(header.Version, jsonwallVersion[:strings.Index(jsonwallVersion, ".")+1]) {
		return nil, fmt.Errorf("unsupported database format: %q", header.Version)
	}
	if header.Count > len(data)/8 {
		// The header helps pre-allocating an index of the right size,
		// but it could trivially be abused to cause an OOM situation.
		header.Count = 0
	}
	db := &DB{schema: header.Schema, data: data}
	db.index = make([]int, 0, header.Count)
	for i := range data {
		if data[i] == '\n' && i+1 < len(data) && data[i+1] == '{' {
			db.index = append(db.index, i+1)
		}
	}
	db.count = len(db.index)
	db.index = append(db.index, len(db.data))
	return db, nil
}

// DB holds an in-memory read-only database ready for querying.
type DB struct {
	schema string
	data   []byte
	index  []int
	count  int
}

// Schema returns the optional schema value that was provided when writing the database.
// This information is available to help with the decoding of the data, and is not
// internally interpreted in any way.
func (db *DB) Schema() string {
	return db.schema
}

func (db *DB) prefix(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil || len(data) == 0 || data[0] != '{' {
		return nil, fmt.Errorf("invalid database search value: %#v", value)
	}
	data[len(data)-1] = ','
	return data, nil
}

var ErrNotFound = fmt.Errorf("value not found in database")

func (db *DB) search(prefix []byte) (i int, err error) {
	i = sort.Search(db.count, func(i int) bool {
		res := bytes.Compare(db.data[db.index[i]:], prefix) >= 0
		return res
	})
	if db.match(i, prefix) {
		return i, nil
	}
	return -1, ErrNotFound
}

func (db *DB) iterate(prefix []byte) *Iterator {
	i := sort.Search(db.count, func(i int) bool {
		res := bytes.Compare(db.data[db.index[i]:], prefix) >= 0
		return res
	})
	return &Iterator{db: db, prefix: prefix, pos: i, next: i}
}

func (db *DB) match(i int, prefix []byte) bool {
	if i < 0 || i >= db.count {
		return false
	}
	return bytes.HasPrefix(db.data[db.index[i]:], prefix)
}

func (db *DB) decode(i int, value any) error {
	return json.Unmarshal(db.data[db.index[i]:db.index[i+1]], value)
}

// Get encodes the provided value as JSON, finds the first entry in the
// database with initial fields exactly matching that encoding, and then
// decodes the entry back into the provided value.
func (db *DB) Get(value any) error {
	prefix, err := db.prefix(value)
	if err != nil {
		return err
	}
	i, err := db.search(prefix)
	if err != nil {
		return err
	}
	return db.decode(i, value)
}

// Iterate encodes the provided value as JSON and returns an iterator that will
// go over every entry in the database with initial fields that exactly match
// that encoding. An iterator is still returned even if no entries match the
// provided value.
func (db *DB) Iterate(value any) (*Iterator, error) {
	if value == nil {
		return &Iterator{db: db}, nil
	}
	prefix, err := db.prefix(value)
	if err != nil {
		return nil, err
	}
	return db.iterate(prefix), nil
}

// IteratePrefix works similarly to Iterate, except that after encoding the
// provided value as JSON, its last encoded field must be a string that will be
// matched as a prefix of the respective database entry field, instead of being
// matched exactly.
func (db *DB) IteratePrefix(value any) (*Iterator, error) {
	prefix, err := db.prefix(value)
	if err != nil {
		return nil, err
	}
	if !bytes.HasSuffix(prefix, []byte{'"', ','}) {
		return nil, fmt.Errorf("cannot iterate prefix: last field is not a string")
	}
	prefix = prefix[:len(prefix)-2]
	return db.iterate(prefix), nil
}

type Iterator struct {
	db     *DB
	prefix []byte
	pos    int
	next   int
}

// Next positions the iterator on the next available entry for decoding and returns
// whether such an entry was found. Next must also be called for the first entry of
// the iteration as the result set might be empty.
func (iter *Iterator) Next() bool {
	iter.pos = iter.next
	if iter.db.match(iter.pos, iter.prefix) {
		iter.next++
		return true
	}
	return false

}

// Get decodes the current entry into the provided value. The Next method must
// always be called first.
func (iter *Iterator) Get(value any) error {
	if iter.pos == iter.next {
		return ErrNotFound
	}
	return iter.db.decode(iter.pos, value)
}
