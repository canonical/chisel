package control

import (
	"bytes"
	"io"
	"io/ioutil"
	"strings"
)


// The logic in this file is supposed to be fast so that parsing large data
// files feels instantaneous. It does that by performing a fast scan once to
// index the sections, and then rather than parsing the individual sections it
// scans fields directly on retrieval. That means the whole content is loaded
// in memory at once and without impact to the GC. Should be a good enough
// strategy for the sort of files handled, with long documents of sections
// that are relatively few fields long.

type File interface {
	Section(key string) Section
}

type Section interface {
	Get(key string) string
}

type ctrlFile struct {
	// For the moment content is cached as a string internally as it's faster
	// to convert it all at once and remaining operations will not involve
	// the GC for the individual string data.
	content    string
	sections   map[string]ctrlPos
	sectionKey string
}

func (f *ctrlFile) Section(key string) Section {
	if pos, ok := f.sections[key]; ok {
		return &ctrlSection{f.content[pos.start:pos.end]}
	}
	return nil
}

type ctrlSection struct {
	content string
}

func (s *ctrlSection) Get(key string) string {
	content := s.content
	pos := 0
	if len(content) > len(key)+1 && content[:len(key)] == key && content[len(key)] == ':' {
		// Key is on the first line.
		pos = len(key) + 1
	} else {
		prefix := "\n" + key + ":"
		pos = strings.Index(content, prefix)
		if pos < 0 {
			return ""
		}
		pos += len(prefix)
		if pos+1 > len(content) {
			return ""
		}
	}
	if content[pos] == ' ' {
		pos++
	}
	eol := strings.Index(content[pos:], "\n")
	if eol < 0 {
		eol = len(content)
	} else {
		eol += pos
	}
	value := content[pos:eol]
	if eol+1 >= len(content) || content[eol+1] != ' ' && content[eol+1] != '\t' {
		// Single line value.
		return value
	}
	// Multi line value so we'll need to allocate.
	var multi bytes.Buffer
	if len(value) > 0 {
		multi.WriteString(value)
		multi.WriteByte('\n')
	}
	for {
		pos = eol + 2
		eol = strings.Index(content[pos:], "\n")
		if eol < 0 {
			eol = len(content)
		} else {
			eol += pos
		}
		multi.WriteString(content[pos:eol])
		if eol+1 >= len(content) || content[eol+1] != ' ' && content[eol+1] != '\t' {
			break
		}
		multi.WriteByte('\n')
	}
	return multi.String()
}

type ctrlPos struct {
	start, end int
}

func ParseReader(sectionKey string, content io.Reader) (File, error) {
	data, err := ioutil.ReadAll(content)
	if err != nil {
		return nil, err
	}
	return ParseString(sectionKey, string(data))
}

func ParseString(sectionKey, content string) (File, error) {
	skey := sectionKey + ": "
	skeylen := len(skey)
	sections := make(map[string]ctrlPos)
	start := 0
	pos := start
	for pos < len(content) {
		eol := strings.Index(content[pos:], "\n")
		if eol < 0 {
			eol = len(content)
		} else {
			eol += pos
		}
		if pos+skeylen < len(content) && content[pos:pos+skeylen] == skey {
			pos += skeylen
			end := strings.Index(content[pos:], "\n\n")
			if end < 0 {
				end = len(content)
			} else {
				end += pos
			}
			sections[content[pos:eol]] = ctrlPos{start, end}
			pos = end + 2
			start = pos
		} else {
			pos = eol + 1
		}
	}
	return &ctrlFile{
		content:    content,
		sections:   sections,
		sectionKey: sectionKey,
	}, nil
}
