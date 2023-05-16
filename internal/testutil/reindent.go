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
