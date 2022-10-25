package scripts

import (
	"fmt"
	"regexp"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

var reModuleName = "re.star"

var reModule = starlark.StringDict{
	"re": &starlarkstruct.Module{
		Name: "re",
		Members: starlark.StringDict{
			"compile": starlark.NewBuiltin("re.compile", reCompile),
			"find":    starlark.NewBuiltin("re.find", reFind),
			"findall": starlark.NewBuiltin("re.findall", reFindAll),
			"split":   starlark.NewBuiltin("re.split", reSplit),
			"sub":     starlark.NewBuiltin("re.sub", reSub),
		},
	},
}

func unmarshalInt(in starlark.Int) (int, error) {
	var out int = 0
	err := starlark.AsInt(in, &out)
	return out, err
}

func regexpFind(regex *regexp.Regexp, str starlark.String) (starlark.Value, error) {
	match := regex.FindStringSubmatchIndex(string(str))
	if match == nil {
		return nil, nil
	}
	return &reMatch{str: string(str), match: match}, nil
}

func regexpFindAll(regex *regexp.Regexp, str starlark.String, limit starlark.Int) (starlark.Value, error) {
	limitInt, err := unmarshalInt(limit)
	if err != nil {
		return nil, err
	}
	matches := regex.FindAllStringSubmatchIndex(string(str), limitInt)
	resultArray := make([]starlark.Value, len(matches))
	for i, match := range matches {
		resultArray[i] = &reMatch{str: string(str), match: match}
	}
	return starlark.NewList(resultArray), nil
}

func regexpSplit(regex *regexp.Regexp, str starlark.String, limit starlark.Int) (starlark.Value, error) {
	limitInt, err := unmarshalInt(limit)
	if err != nil {
		return nil, err
	}
	parts := regex.Split(string(str), limitInt)
	resultArray := make([]starlark.Value, len(parts))
	for i, part := range parts {
		resultArray[i] = starlark.String(part)
	}
	return starlark.NewList(resultArray), nil
}

func regexpSub(regex *regexp.Regexp, repl starlark.String, str starlark.String) (starlark.Value, error) {
	return starlark.String(regex.ReplaceAllString(string(str), string(repl))), nil
}

func reCompile(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern starlark.String
	if err := starlark.UnpackArgs("re.compile", args, kwargs, "pattern", &pattern); err != nil {
		return starlark.None, err
	}
	return newPatterng(pattern)
}

func reFind(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern, str starlark.String
	if err := starlark.UnpackArgs("re.find", args, kwargs, "pattern", &pattern, "string", &str); err != nil {
		return starlark.None, err
	}
	regex, err := regexp.Compile(string(pattern))
	if err != nil {
		return nil, err
	}
	return regexpFind(regex, str)
}

func reFindAll(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern, str starlark.String
	var limit = starlark.MakeInt(-1)
	if err := starlark.UnpackArgs("re.findall", args, kwargs, "pattern", &pattern, "string", &str, "limit?", &limit); err != nil {
		return starlark.None, err
	}
	regex, err := regexp.Compile(string(pattern))
	if err != nil {
		return nil, err
	}
	return regexpFindAll(regex, str, limit)
}

func reSplit(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern, str starlark.String
	var limit = starlark.MakeInt(-1)
	if err := starlark.UnpackArgs("re.split", args, kwargs, "pattern", &pattern, "string", &str, "limit?", &limit); err != nil {
		return starlark.None, err
	}
	regex, err := regexp.Compile(string(pattern))
	if err != nil {
		return nil, err
	}
	return regexpSplit(regex, str, limit)
}

func reSub(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern, repl, str starlark.String
	if err := starlark.UnpackArgs("re.sub", args, kwargs, "pattern", &pattern, "repl", &repl, "string", &str); err != nil {
		return starlark.None, err
	}
	regex, err := regexp.Compile(string(pattern))
	if err != nil {
		return nil, err
	}
	return regexpSub(regex, repl, str)
}

// reMatchIterator

type reMatchIterator struct {
	m *reMatch
	i int
}

var _ starlark.Iterator = (*reMatchIterator)(nil)

func (it *reMatchIterator) Next(p *starlark.Value) bool {
	if it.i < it.m.Len() {
		*p = it.m.Index(it.i)
		it.i++
		return true
	}
	return false

}
func (it *reMatchIterator) Done() {}

// reMatch

type reMatch struct {
	str   string
	match []int
}

var _ starlark.Value = (*reMatch)(nil)

func (m *reMatch) String() string {
	return m.groupsInternal().String()
}

func (m *reMatch) Type() string {
	return "re.Match"
}

func (m *reMatch) Freeze() {
}

func (m *reMatch) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable: %s", m.Type())
}

func (m *reMatch) Truth() starlark.Bool {
	return true
}

var _ starlark.Indexable = (*reMatch)(nil)

func (m *reMatch) Index(i int) starlark.Value {
	return starlark.String(m.str[m.match[2*i]:m.match[2*i+1]])
}

func (m *reMatch) Len() int {
	return len(m.match) / 2
}

var _ starlark.Iterable = (*reMatch)(nil)

func (m *reMatch) Iterate() starlark.Iterator {
	return &reMatchIterator{m: m, i: 0}
}

var _ starlark.HasAttrs = (*reMatch)(nil)

func (r *reMatch) Attr(name string) (starlark.Value, error) {
	switch name {
	case "groups":
		return starlark.NewBuiltin("re.Match.groups", r.groups), nil
	}
	return nil, nil
}

func (r *reMatch) AttrNames() []string {
	return []string{"groups"}
}

func (m *reMatch) groupsInternal() starlark.Value {
	groups := make([]starlark.Value, m.Len(), m.Len())
	for i := 0; i < m.Len(); i++ {
		groups[i] = m.Index(i)
	}
	return starlark.Tuple(groups)
}

func (m *reMatch) groups(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
	return m.groupsInternal(), nil
}

// rePattern

type rePattern struct {
	regex *regexp.Regexp
}

func newPatterng(pattern starlark.String) (*rePattern, error) {
	re, err := regexp.Compile(string(pattern))
	if err != nil {
		return nil, err
	}
	return &rePattern{regex: re}, nil
}

var _ starlark.Value = (*rePattern)(nil)

func (r *rePattern) String() string {
	return r.regex.String()
}

func (r *rePattern) Type() string {
	return "re.Pattern"
}

func (r *rePattern) Freeze() {
}

func (r *rePattern) Hash() (uint32, error) {
	return starlark.String(r.regex.String()).Hash()
}

func (r *rePattern) Truth() starlark.Bool {
	return true
}

var _ starlark.HasAttrs = (*rePattern)(nil)

func (r *rePattern) Attr(name string) (starlark.Value, error) {
	switch name {
	case "find":
		return starlark.NewBuiltin("re.Pattern.find", r.find), nil
	case "findall":
		return starlark.NewBuiltin("re.Pattern.findall", r.findall), nil
	case "split":
		return starlark.NewBuiltin("re.Pattern.split", r.split), nil
	case "sub":
		return starlark.NewBuiltin("re.Pattern.sub", r.split), nil
	}
	return nil, nil
}

func (r *rePattern) AttrNames() []string {
	return []string{"find", "findall", "split"}
}

func (r *rePattern) find(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var str starlark.String
	if err := starlark.UnpackArgs("re.Pattern.find", args, kwargs, "string", &str); err != nil {
		return starlark.None, err
	}
	return regexpFind(r.regex, str)
}

func (r *rePattern) findall(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var str starlark.String
	var limit = starlark.MakeInt(-1)
	if err := starlark.UnpackArgs("re.Pattern.findall", args, kwargs, "string", &str, "limit?", &limit); err != nil {
		return starlark.None, err
	}
	return regexpFindAll(r.regex, str, limit)
}

func (r *rePattern) split(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var str starlark.String
	var limit = starlark.MakeInt(-1)
	if err := starlark.UnpackArgs("re.Pattern.split", args, kwargs, "string", &str, "limit?", &limit); err != nil {
		return starlark.None, err
	}
	return regexpSplit(r.regex, str, limit)
}

func (r *rePattern) sub(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var repl, str starlark.String
	if err := starlark.UnpackArgs("re.Pattern.sub", args, kwargs, "repl", &repl, "string", &str); err != nil {
		return starlark.None, err
	}
	return regexpSub(r.regex, repl, str)
}
