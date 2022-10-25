package scripts_test

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	. "gopkg.in/check.v1"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarktest"

	"github.com/canonical/chisel/internal/scripts"
)

func testModuleLoader(thread *starlark.Thread, moduleName string) (starlark.StringDict, error) {
	if moduleName == "assert.star" {
		return starlarktest.LoadAssertModule()
	}
	return scripts.LoadModule(thread, moduleName)
}

func execStarFile(c *C, path string) starlark.StringDict {
	thread := &starlark.Thread{Load: testModuleLoader}
	starlarktest.SetReporter(thread, c)
	env, err := starlark.ExecFile(thread, path, nil, nil)
	c.Assert(err, IsNil)
	return env
}

func callTestFunctions(c *C, env starlark.StringDict) {
	for name, value := range env {
		thread := &starlark.Thread{}
		starlarktest.SetReporter(thread, c)
		if value.Type() == "function" && strings.HasPrefix(name, "test_") {
			_, err := starlark.Call(thread, value, []starlark.Value{}, []starlark.Tuple{})
			c.Assert(err, IsNil)
		}
	}
}

func runStarFileTest(c *C, path string) {
	env := execStarFile(c, path)
	callTestFunctions(c, env)
}

func runStarDirTest(c *C, dir string) {
	fileList, err := ioutil.ReadDir(dir)
	if err != nil {
		c.Errorf("error listing directory %#v, ioutil.ReadDir: %v", dir, err)
	}
	for _, file := range fileList {
		path := filepath.Join(dir, file.Name())
		if strings.HasSuffix(path, ".star") {
			runStarFileTest(c, path)
		}
	}
}

func (s *S) TestStarFiles(c *C) {
	runStarDirTest(c, "testdata")
}
