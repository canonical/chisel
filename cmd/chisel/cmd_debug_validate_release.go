package main

import (
	"fmt"

	"github.com/jessevdk/go-flags"
)

var (
	shortValidateReleaseHelp = "Validate a Chisel release"
	longValidateReleaseHelp  = `
The validate-release command performs the static validation of a Chisel
release, checking the slice definition files (SDFs) and the chisel.yaml
file for structural issues without downloading any package.

It is a lightweight way to ensure that a release being authored or
modified is well formed. For a deeper validation that also downloads
packages and checks for content conflicts, see the
"check-release-archives" command.

By default it fetches the slices for the same Ubuntu version as the
current host, unless the --release flag is used. Pointing --release to a
local directory avoids any network access.
`
)

var validateReleaseDescs = map[string]string{
	"release": "Chisel release name or directory (e.g. ubuntu-22.04)",
}

type cmdDebugValidateRelease struct {
	Release string `long:"release" value-name:"<branch|dir>"`
}

func init() {
	addDebugCommand("validate-release", shortValidateReleaseHelp, longValidateReleaseHelp, func() flags.Commander { return &cmdDebugValidateRelease{} }, validateReleaseDescs, nil)
}

func (cmd *cmdDebugValidateRelease) Execute(args []string) error {
	if len(args) > 0 {
		return ErrExtraArgs
	}

	_, err := obtainRelease(cmd.Release)
	if err != nil {
		return err
	}

	fmt.Fprintln(Stdout, "Release is valid")
	return nil
}
