package scripts

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"

	"github.com/canonical/chisel/internal/fsutil"
)

type Value = starlark.Value

type RunOptions struct {
	Label     string
	Namespace map[string]Value
	Script    string
}

func Run(opts *RunOptions) error {
	thread := &starlark.Thread{Name: opts.Label}
	fileOptions := &syntax.FileOptions{
		TopLevelControl: true,
		GlobalReassign:  true,
	}
	globals, err := starlark.ExecFileOptions(fileOptions, thread, opts.Label, opts.Script, opts.Namespace)
	_ = globals
	return err
}

type ContentValue struct {
	RootDir    string
	CheckRead  func(path string) error
	CheckWrite func(path string) error
	// OnWrite has to be called after a successful write with the entry resulting
	// from the write.
	OnWrite func(entry *fsutil.Entry) error
}

// Content starlark.Value interface
// --------------------------------------------------------------------------

func (c *ContentValue) String() string {
	return "Content{...}"
}

func (c *ContentValue) Type() string {
	return "Content"
}

func (c *ContentValue) Freeze() {
}

func (c *ContentValue) Truth() starlark.Bool {
	return true
}

func (c *ContentValue) Hash() (uint32, error) {
	return starlark.String(c.RootDir).Hash()
}

// Content starlark.HasAttrs interface
// --------------------------------------------------------------------------

var _ starlark.HasAttrs = new(ContentValue)

func (c *ContentValue) Attr(name string) (Value, error) {
	switch name {
	case "read":
		return starlark.NewBuiltin("Content.read", c.Read), nil
	case "write":
		return starlark.NewBuiltin("Content.write", c.Write), nil
	case "list":
		return starlark.NewBuiltin("Content.list", c.List), nil
	}
	return nil, nil
}

func (c *ContentValue) AttrNames() []string {
	return []string{"read", "write", "list"}
}

// Content methods
// --------------------------------------------------------------------------

type Check uint

const (
	CheckNone = 0
	CheckRead = 1 << iota
	CheckWrite
)

func (c *ContentValue) RealPath(path string, what Check) (string, error) {
	if !filepath.IsAbs(c.RootDir) {
		return "", fmt.Errorf("internal error: content defined with relative root: %s", c.RootDir)
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("content path must be absolute, got: %s", path)
	}
	cpath := filepath.Clean(path)
	if cpath != "/" && strings.HasSuffix(path, "/") {
		cpath += "/"
	}
	if c.CheckRead != nil && what&CheckRead != 0 {
		err := c.CheckRead(cpath)
		if err != nil {
			return "", err
		}
	}
	if c.CheckWrite != nil && what&CheckWrite != 0 {
		err := c.CheckWrite(cpath)
		if err != nil {
			return "", err
		}
	}
	rpath := filepath.Join(c.RootDir, cpath)
	if lname, err := os.Readlink(rpath); err == nil {
		lpath := filepath.Join(filepath.Dir(rpath), filepath.Clean(lname))
		lrel, err := filepath.Rel(c.RootDir, lpath)
		if err != nil || !filepath.IsAbs(lpath) {
			return "", fmt.Errorf("invalid content symlink: %s", path)
		}
		return c.RealPath("/"+lrel, what)
	}
	return rpath, nil
}

func (c *ContentValue) polishError(path starlark.String, err error) error {
	if e, ok := err.(*os.PathError); ok {
		e.Path = path.GoString()
	}
	return err
}

func (c *ContentValue) Read(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (Value, error) {
	var path starlark.String
	err := starlark.UnpackArgs("Content.read", args, kwargs, "path", &path)
	if err != nil {
		return nil, err
	}

	fpath, err := c.RealPath(path.GoString(), CheckRead)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(fpath)
	if err != nil {
		return nil, c.polishError(path, err)
	}
	return starlark.String(data), nil
}

func (c *ContentValue) Write(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (Value, error) {
	var path starlark.String
	var data starlark.String
	err := starlark.UnpackArgs("Content.write", args, kwargs, "path", &path, "data", &data)
	if err != nil {
		return nil, err
	}

	fpath, err := c.RealPath(path.GoString(), CheckWrite)
	if err != nil {
		return nil, err
	}
	fdata := []byte(data.GoString())

	// No mode parameter for now as slices are supposed to list files
	// explicitly instead.
	entry, err := fsutil.Create(&fsutil.CreateOptions{
		Root: "/",
		Path: fpath,
		Data: bytes.NewReader(fdata),
		Mode: 0644,
	})
	if err != nil {
		return nil, c.polishError(path, err)
	}
	err = c.OnWrite(entry)
	if err != nil {
		return nil, err
	}
	return starlark.None, nil
}

func (c *ContentValue) List(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (Value, error) {
	var path starlark.String
	err := starlark.UnpackArgs("Content.list", args, kwargs, "path", &path)
	if err != nil {
		return nil, err
	}

	dpath := path.GoString()
	if !strings.HasSuffix(dpath, "/") {
		dpath += "/"
	}
	fpath, err := c.RealPath(dpath, CheckRead)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(fpath)
	if err != nil {
		return nil, c.polishError(path, err)
	}
	values := make([]Value, len(entries))
	for i, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		values[i] = starlark.String(name)
	}
	return starlark.NewList(values), nil
}
