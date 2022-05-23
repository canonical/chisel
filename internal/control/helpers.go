package control

import (
	"regexp"
	"strconv"
	"strings"
)

var pathInfoExp = regexp.MustCompile(`([a-f0-9]{32,}) +([0-9]+) +\S+`)

func ParsePathInfo(table, path string) (digest string, size int, ok bool) {
	pos := strings.Index(table, " "+path+"\n")
	if pos == -1 {
		if !strings.HasSuffix(table, " " + path) {
			return "", -1, false
		}
		pos = len(table) - len(path)
	} else {
		pos++
	}
	eol := pos + len(path)
	for pos > 0 && table[pos] != '\n' {
		pos--
	}
	match := pathInfoExp.FindStringSubmatch(table[pos:eol])
	if match == nil {
		return "", -1, false
	}
	size, err := strconv.Atoi(match[2])
	if err != nil {
		panic("internal error: FindPathInfo regexp is wrong")
	}
	return match[1], size, true
}
