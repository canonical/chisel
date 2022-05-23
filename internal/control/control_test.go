package control_test

import (
	"github.com/canonical/chisel/internal/control"

	"bytes"
	"io/ioutil"
	"testing"

	. "gopkg.in/check.v1"
)

var testFile = `` +
	`Section: one
Line: line for one
Multi:
 multi line
 for one

Line: line for two
Multi: multi line
 for two
Section: two

Multi:
 multi line
 for three
Line: line for three
Section: three

Section: four
Multi: 
 Space at EOL above
  Extra space
	One tab
`

var testFileResults = map[string]map[string]string{
	"one": map[string]string{
		"Section": "one",
		"Line":    "line for one",
		"Multi":   "multi line\nfor one",
	},
	"two": map[string]string{
		"Section": "two",
		"Line":    "line for two",
		"Multi":   "multi line\nfor two",
	},
	"three": map[string]string{
		"Section": "three",
		"Line":    "line for three",
		"Multi":   "multi line\nfor three",
	},
	"four": map[string]string{
		"Multi": "Space at EOL above\n Extra space\nOne tab",
	},
}

func (s *S) TestParseString(c *C) {
	file, err := control.ParseString("Section", testFile)
	c.Assert(err, IsNil)

	for skey, svalues := range testFileResults {
		section := file.Section(skey)
		for key, value := range svalues {
			c.Assert(section.Get(key), Equals, value, Commentf("Section %q / Key %q", skey, key))
		}
	}
}

func (s *S) TestParseReader(c *C) {
	file, err := control.ParseReader("Section", bytes.NewReader([]byte(testFile)))
	c.Assert(err, IsNil)

	for skey, svalues := range testFileResults {
		section := file.Section(skey)
		for key, value := range svalues {
			c.Assert(section.Get(key), Equals, value, Commentf("Section %q / Key %q", skey, key))
		}
	}
}

func BenchmarkParse(b *testing.B) {
	data, err := ioutil.ReadFile("Packages")
	if err != nil {
		b.Fatalf("cannot open Packages file: %v", err)
	}
	content := string(data)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		control.ParseString("Package", content)
	}
}

func BenchmarkSectionGet(b *testing.B) {
	data, err := ioutil.ReadFile("Packages")
	if err != nil {
		b.Fatalf("cannot open Packages file: %v", err)
	}
	content := string(data)
	file, err := control.ParseString("Package", content)
	if err != nil {
		panic(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		section := file.Section("util-linux")
		value := section.Get("Description")
		if value != "miscellaneous system utilities" {
			b.Fatalf("Unexpected package description: %q", value)
		}
	}
}
