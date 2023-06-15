package db_test

import (
	"sort"

	"github.com/canonical/chisel/internal/db"
	. "gopkg.in/check.v1"
)

type testEntry struct {
	S string          `json:"s,omitempty"`
	I int64           `json:"i,omitempty"`
	L []string        `json:"l,omitempty"`
	M map[string]bool `json:"m,omitempty"`
}

var saveLoadTestCase = []testEntry{
	{"", 0, nil, nil},
	{"hello", -1, nil, nil},
	{"", 0, nil, nil},
	{"", 100, []string{"a", "b"}, nil},
	{"", 0, nil, map[string]bool{"a": true, "b": false}},
	{"abc", 123, []string{"foo", "bar"}, nil},
}

func (s *S) TestSaveLoadRoundTrip(c *C) {
	// To compare expected and obtained entries we first wrap the original
	// entries in wrappers with increasing K. When we read the wrappers back
	// they may be in different order because jsonwall sorts them serialized
	// as JSON. So we sort them by K to compare them in the original order.

	type wrapper struct {
		// test values
		testEntry
		// sort key for comparison
		K int `json:"key"`
	}

	// wrap the entries with increasing K
	expected := make([]wrapper, len(saveLoadTestCase))
	for i, entry := range saveLoadTestCase {
		expected[i] = wrapper{entry, i}
	}

	workDir := c.MkDir()
	dbw := db.New()
	for _, entry := range expected {
		err := dbw.Add(entry)
		c.Assert(err, IsNil)
	}
	err := db.Save(dbw, workDir)
	c.Assert(err, IsNil)

	dbr, err := db.Load(workDir)
	c.Assert(err, IsNil)
	c.Assert(dbr.Schema(), Equals, db.Schema)

	iter, err := dbr.Iterate(nil)
	c.Assert(err, IsNil)

	obtained := make([]wrapper, 0, len(expected))
	for iter.Next() {
		var wrapped wrapper
		err := iter.Get(&wrapped)
		c.Assert(err, IsNil)
		obtained = append(obtained, wrapped)
	}

	// sort the entries by K to get the original order
	sort.Slice(obtained, func(i, j int) bool {
		return obtained[i].K < obtained[j].K
	})
	c.Assert(obtained, DeepEquals, expected)
}
