package main_test

import (
	"bytes"
	"os"
	"testing"

	"golang.org/x/crypto/ssh/terminal"

	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/cmd"
	"github.com/canonical/chisel/internal/testutil"

	chisel "github.com/canonical/chisel/cmd/chisel"
)

// Hook up check.v1 into the "go test" runner
func Test(t *testing.T) { TestingT(t) }

type BaseChiselSuite struct {
	testutil.BaseTest
	stdin     *bytes.Buffer
	stdout    *bytes.Buffer
	stderr    *bytes.Buffer
	password  string
}

func (s *BaseChiselSuite) readPassword(fd int) ([]byte, error) {
	return []byte(s.password), nil
}

func (s *BaseChiselSuite) SetUpTest(c *C) {
	s.BaseTest.SetUpTest(c)

	s.stdin = bytes.NewBuffer(nil)
	s.stdout = bytes.NewBuffer(nil)
	s.stderr = bytes.NewBuffer(nil)
	s.password = ""

	chisel.Stdin = s.stdin
	chisel.Stdout = s.stdout
	chisel.Stderr = s.stderr
	chisel.ReadPassword = s.readPassword

	s.AddCleanup(chisel.FakeIsStdoutTTY(false))
	s.AddCleanup(chisel.FakeIsStdinTTY(false))
}

func (s *BaseChiselSuite) TearDownTest(c *C) {
	chisel.Stdin = os.Stdin
	chisel.Stdout = os.Stdout
	chisel.Stderr = os.Stderr
	chisel.ReadPassword = terminal.ReadPassword

	s.BaseTest.TearDownTest(c)
}

func (s *BaseChiselSuite) Stdout() string {
	return s.stdout.String()
}

func (s *BaseChiselSuite) Stderr() string {
	return s.stderr.String()
}

func (s *BaseChiselSuite) ResetStdStreams() {
	s.stdin.Reset()
	s.stdout.Reset()
	s.stderr.Reset()
}

func fakeArgs(args ...string) (restore func()) {
	old := os.Args
	os.Args = args
	return func() { os.Args = old }
}

func fakeVersion(v string) (restore func()) {
	old := cmd.Version
	cmd.Version = v
	return func() { cmd.Version = old }
}

type ChiselSuite struct {
	BaseChiselSuite
}

var _ = Suite(&ChiselSuite{})
