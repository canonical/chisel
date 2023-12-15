package testutil

import (
	"bytes"
	"strings"
)

// Reindent deindents the provided string and replaces tabs with spaces
// so yaml inlined into tests works properly when decoded.
func Reindent(in string) []byte {
	var buf bytes.Buffer
	var trim string
	var trimSet bool
	for _, line := range strings.Split(in, "\n") {
		if !trimSet {
			trimmed := strings.TrimLeft(line, "\t")
			if trimmed == "" {
				continue
			}
			if trimmed[0] == ' ' {
				panic("Space used in indent early in string:\n" + in)
			}

			trim = line[:len(line)-len(trimmed)]
			trimSet = true

			if trim == "" {
				return []uint8(strings.ReplaceAll(in, "\t", "    ") + "\n")
			}
		}
		trimmed := strings.TrimPrefix(line, trim)
		if len(trimmed) == len(line) && strings.TrimLeft(line, "\t ") != "" {
			panic("Line not indented consistently:\n" + line)
		}
		trimmed = strings.ReplaceAll(trimmed, "\t", "    ")
		buf.WriteString(trimmed)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// PrefixEachLine indents each line in the provided string with the prefix.
func PrefixEachLine(text string, prefix string) string {
	var result strings.Builder
	lines := strings.Split(text, "\n")
	lastNewline := false
	if strings.HasSuffix(text, "\n") {
		// Skip iterating over the empty line.
		lines = lines[:len(lines)-1]
		lastNewline = true
	}
	for i, line := range lines {
		result.WriteString(prefix)
		result.WriteString(line)
		if i == len(lines)-1 && !lastNewline {
			// Do not add the last newline if the text did not have it to begin with.
			continue
		} else {
			result.WriteString("\n")
		}
	}
	return result.String()
}
