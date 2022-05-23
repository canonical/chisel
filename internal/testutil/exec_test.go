// Copyright (c) 2014-2020 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package testutil

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"

	"gopkg.in/check.v1"
)

type fakeCommandSuite struct{}

var _ = check.Suite(&fakeCommandSuite{})

func (s *fakeCommandSuite) TestFakeCommand(c *check.C) {
	fake := FakeCommand(c, "cmd", "true")
	defer fake.Restore()
	err := exec.Command("cmd", "first-run", "--arg1", "arg2", "a space").Run()
	c.Assert(err, check.IsNil)
	err = exec.Command("cmd", "second-run", "--arg1", "arg2", "a %s").Run()
	c.Assert(err, check.IsNil)
	c.Assert(fake.Calls(), check.DeepEquals, [][]string{
		{"cmd", "first-run", "--arg1", "arg2", "a space"},
		{"cmd", "second-run", "--arg1", "arg2", "a %s"},
	})
}

func (s *fakeCommandSuite) TestFakeCommandAlso(c *check.C) {
	fake := FakeCommand(c, "fst", "")
	also := fake.Also("snd", "")
	defer fake.Restore()

	c.Assert(exec.Command("fst").Run(), check.IsNil)
	c.Assert(exec.Command("snd").Run(), check.IsNil)
	c.Check(fake.Calls(), check.DeepEquals, [][]string{{"fst"}, {"snd"}})
	c.Check(fake.Calls(), check.DeepEquals, also.Calls())
}

func (s *fakeCommandSuite) TestFakeCommandConflictEcho(c *check.C) {
	fake := FakeCommand(c, "do-not-swallow-echo-args", "")
	defer fake.Restore()

	c.Assert(exec.Command("do-not-swallow-echo-args", "-E", "-n", "-e").Run(), check.IsNil)
	c.Assert(fake.Calls(), check.DeepEquals, [][]string{
		{"do-not-swallow-echo-args", "-E", "-n", "-e"},
	})
}

func (s *fakeCommandSuite) TestFakeShellchecksWhenAvailable(c *check.C) {
	tmpDir := c.MkDir()
	fakeShellcheck := FakeCommand(c, "shellcheck", fmt.Sprintf(`cat > %s/input`, tmpDir))
	defer fakeShellcheck.Restore()

	restore := FakeShellcheckPath(fakeShellcheck.Exe())
	defer restore()

	fake := FakeCommand(c, "some-command", "echo some-command")

	c.Assert(exec.Command("some-command").Run(), check.IsNil)

	c.Assert(fake.Calls(), check.DeepEquals, [][]string{
		{"some-command"},
	})
	c.Assert(fakeShellcheck.Calls(), check.DeepEquals, [][]string{
		{"shellcheck", "-s", "bash", "-"},
	})

	scriptData, err := ioutil.ReadFile(fake.Exe())
	c.Assert(err, check.IsNil)
	c.Assert(string(scriptData), Contains, "\necho some-command\n")

	data, err := ioutil.ReadFile(filepath.Join(tmpDir, "input"))
	c.Assert(err, check.IsNil)
	c.Assert(data, check.DeepEquals, scriptData)
}

func (s *fakeCommandSuite) TestFakeNoShellchecksWhenNotAvailable(c *check.C) {
	fakeShellcheck := FakeCommand(c, "shellcheck", `echo "i am not called"; exit 1`)
	defer fakeShellcheck.Restore()

	restore := FakeShellcheckPath("")
	defer restore()

	// This would fail with proper shellcheck due to SC2086: Double quote to
	// prevent globbing and word splitting.
	fake := FakeCommand(c, "some-command", "echo $1")

	c.Assert(exec.Command("some-command").Run(), check.IsNil)

	c.Assert(fake.Calls(), check.DeepEquals, [][]string{
		{"some-command"},
	})
	c.Assert(fakeShellcheck.Calls(), check.HasLen, 0)
}
