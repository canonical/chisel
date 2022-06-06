package scripts

import (
	"go.starlark.net/starlark"

	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type Value = starlark.Value

type RunOptions struct {
	Label      string
	Namespace  map[string]Value
	Script     string
}

func Run(opts *RunOptions) error {
	thread := &starlark.Thread{Name: opts.Label}
	globals, err := starlark.ExecFile(thread, opts.Label, opts.Script, opts.Namespace)
	_ = globals
	return err
}

type ContentValue struct {
	RootDir    string
	CheckRead  func(path string) error
	CheckWrite func(path string) error
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
		return starlark.NewBuiltin("content.read", c.Read), nil
	case "write":
		return starlark.NewBuiltin("content.write", c.Write), nil
	}
	return nil, nil
}

func (c *ContentValue) AttrNames() []string {
	return []string{"read", "write"}
}

// Content methods
// --------------------------------------------------------------------------

const (
	checkNone  = 0
	checkWrite = 1
	checkRead  = 2
)

func (c *ContentValue) realPath(path starlark.String, checkWhat int) (string, error) {
	fpath := path.GoString()
	if !filepath.IsAbs(fpath) {
		return "", fmt.Errorf("content path must be absolute, got: %s", path.GoString())
	}
	cpath := filepath.Clean(fpath)
	if c.CheckRead != nil && checkWhat&checkRead != 0 {
		err := c.CheckRead(cpath)
		if err != nil {
			return "", err
		}
	}
	if c.CheckWrite != nil && checkWhat&checkWrite != 0 {
		err := c.CheckWrite(cpath)
		if err != nil {
			return "", err
		}
	}
	rpath := filepath.Join(c.RootDir, fpath)
	if !filepath.IsAbs(rpath) || !strings.HasPrefix(rpath, c.RootDir+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid content path: %s", path.GoString())
	}
	return rpath, nil
}

func (c *ContentValue) polishError(path starlark.String, err error) error {
	if e, ok := err.(*os.PathError); ok {
		e.Path = path.GoString()
	}
	return err
}

func (c *ContentValue) Read(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.String
	err := starlark.UnpackArgs("Content.read", args, kwargs, "path", &path)
	if err != nil {
		return nil, err
	}

	fpath, err := c.realPath(path, checkRead)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadFile(fpath)
	if err != nil {
		return nil, c.polishError(path, err)
	}
	return starlark.String(data), nil
}

func (c *ContentValue) Write(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path starlark.String
	var data starlark.String
	err := starlark.UnpackArgs("Content.write", args, kwargs, "path", &path, "data", &data)
	if err != nil {
		return nil, err
	}

	fpath, err := c.realPath(path, checkWrite)
	if err != nil {
		return nil, err
	}
	fdata := []byte(data.GoString())

	// No mode parameter for now as slices are supposed to list files
	// explicitly instead.
	err = ioutil.WriteFile(fpath, fdata, 0644)
	if err != nil {
		return nil, c.polishError(path, err)
	}
	return starlark.None, nil
}
