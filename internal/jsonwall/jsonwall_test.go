package jsonwall_test

import (
	. "gopkg.in/check.v1"

	"bytes"

	"github.com/canonical/chisel/internal/jsonwall"
)

type DataType struct {
	A string `json:"a,omitempty"`
	C string `json:"c,omitempty"`
	B string `json:"b,omitempty"`
	D string `json:"d,omitempty"`
}

type dataTypeGet struct {
	get      any
	result   any
	notFound bool
	getError string
}

type dataTypeIter struct {
	iter    any
	results any
}

var dataTypeTests = []struct {
	summary   string
	values    []any
	options   *jsonwall.DBWriterOptions
	database  string
	dbError   string
	getOps    []dataTypeGet
	iterOps   []dataTypeIter
	prefixOps []dataTypeIter
}{{
	summary: "Zero case",
	values:  []any{},
	database: `` +
		`{"jsonwall":"1.0","count":1}` + "\n" +
		``,
	getOps: []dataTypeGet{{
		get:      &DataType{A: "foo"},
		notFound: true,
	}},
	iterOps: []dataTypeIter{{
		iter:    &DataType{A: "foo"},
		results: []DataType(nil),
	}},
	prefixOps: []dataTypeIter{{
		iter:    &DataType{A: "ba"},
		results: []DataType(nil),
	}},
}, {
	summary: "Selection of basic tests",
	values: []any{
		DataType{C: "3"},
		DataType{A: "foo", B: "1"},
		DataType{A: "baz", B: "3"},
		&DataType{C: "1"},
		&DataType{A: "bar", B: "2"},
		&DataType{A: "baz", B: "4"},
		&DataType{C: "2", B: "2"},
	},
	database: `` +
		`{"jsonwall":"1.0","count":8}` + "\n" +
		`{"a":"bar","b":"2"}` + "\n" +
		`{"a":"baz","b":"3"}` + "\n" +
		`{"a":"baz","b":"4"}` + "\n" +
		`{"a":"foo","b":"1"}` + "\n" +
		`{"c":"1"}` + "\n" +
		`{"c":"2","b":"2"}` + "\n" +
		`{"c":"3"}` + "\n" +
		``,
	getOps: []dataTypeGet{{
		get:    &DataType{A: "foo"},
		result: &DataType{A: "foo", B: "1"},
	}, {
		get:    &DataType{C: "2"},
		result: &DataType{C: "2", B: "2"},
	}},
	iterOps: []dataTypeIter{{
		iter: &DataType{A: "baz"},
		results: []DataType{
			{A: "baz", B: "3"},
			{A: "baz", B: "4"},
		},
	}},
	prefixOps: []dataTypeIter{{
		iter: &DataType{A: "ba"},
		results: []DataType{
			{A: "bar", B: "2"},
			{A: "baz", B: "3"},
			{A: "baz", B: "4"},
		},
	}},
}, {
	summary: "Schema definition",
	options: &jsonwall.DBWriterOptions{Schema: "foo"},
	values:  []any{},
	database: `` +
		`{"jsonwall":"1.0","schema":"foo","count":1}` + "\n" +
		``,
}, {
	summary: "Wrong format version",
	database: `` +
		`{"jsonwall":"2.0","count":1}` + "\n" +
		``,
	dbError: `unsupported database format: "2\.0"`,
}, {
	summary: "Compatible format version",
	database: `` +
		`{"jsonwall":"1.999","count":1}` + "\n" +
		`{"a":"foo","b":"1"}` + "\n" +
		``,
	getOps: []dataTypeGet{{
		get:    &DataType{A: "foo"},
		result: &DataType{A: "foo", B: "1"},
	}},
}, {
	summary: "Extra newlines and spaces",
	database: `` +
		`{"jsonwall":"1.0","count":2}` + " \n \n \n" +
		`{"a":"bar","b":"1"}` + " \n \n \n" +
		`{"a":"foo","b":"2"}` + " \n \n \n" +
		``,
	getOps: []dataTypeGet{{
		get:    &DataType{A: "foo"},
		result: &DataType{A: "foo", B: "2"},
	}, {
		get:    &DataType{A: "bar"},
		result: &DataType{A: "bar", B: "1"},
	}},
}, {
	summary: "No trailing newline",
	database: `` +
		`{"jsonwall":"1.0","count":2}` + "\n" +
		`{"a":"bar","b":"1"}` + "\n" +
		`{"a":"foo","b":"2"}` +
		``,
	getOps: []dataTypeGet{{
		get:    &DataType{A: "foo"},
		result: &DataType{A: "foo", B: "2"},
	}, {
		get:    &DataType{A: "bar"},
		result: &DataType{A: "bar", B: "1"},
	}},
}, {
	summary: "Invalid add request",
	values:  []any{
		42,
	},
	dbError: "invalid database value: 42",
}, {
	summary: "Invalid search request",
	values:  []any{},
	database: `` +
		`{"jsonwall":"1.0","count":1}` + "\n" +
		``,
	getOps: []dataTypeGet{{
		get:      42,
		getError: "invalid database search value: 42",
	}},
}}

func (s *S) TestDataTypeTable(c *C) {
	for _, test := range dataTypeTests {
		c.Logf("Summary: %s", test.summary)
		buf := &bytes.Buffer{}
		if test.values == nil {
			buf.WriteString(test.database)
		} else {
			dbw := jsonwall.NewDBWriter(test.options)
			for _, value := range test.values {
				err := dbw.Add(value)
				if test.dbError != "" {
					c.Assert(err, ErrorMatches, test.dbError)
				}
			}
			if len(test.values) > 0 && test.dbError != "" {
				continue
			}
			_, err := dbw.WriteTo(buf)
			c.Assert(err, IsNil)
			c.Assert(buf.String(), Equals, test.database)
		}
		db, err := jsonwall.ReadDB(buf)
		if test.dbError != "" {
			c.Assert(err, ErrorMatches, test.dbError)
			continue
		}
		c.Assert(err, IsNil)
		if test.options != nil {
			c.Assert(db.Schema(), Equals, test.options.Schema)
		}
		for _, op := range test.getOps {
			err := db.Get(op.get)
			if op.notFound {
				c.Assert(err, Equals, jsonwall.ErrNotFound)
			} else if op.getError != "" {
				c.Assert(err, ErrorMatches, op.getError)
			} else {
				c.Assert(err, IsNil)
				c.Assert(op.get, DeepEquals, op.result)
			}
		}
		for _, op := range test.iterOps {
			iter, err := db.Iterate(op.iter)
			c.Assert(err, IsNil)
			var results []DataType
			for iter.Next() {
				var result DataType
				err := iter.Get(&result)
				c.Assert(err, IsNil)
				results = append(results, result)
			}
			c.Assert(results, DeepEquals, op.results)
		}
		for _, op := range test.prefixOps {
			iter, err := db.IteratePrefix(op.iter)
			c.Assert(err, IsNil)
			var results []DataType
			for iter.Next() {
				var result DataType
				err := iter.Get(&result)
				c.Assert(err, IsNil)
				results = append(results, result)
			}
			c.Assert(results, DeepEquals, op.results)
		}
	}
}
