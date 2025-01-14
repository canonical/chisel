package main

import (
	"fmt"

	"github.com/jessevdk/go-flags"

	"github.com/canonical/chisel/cmd"
)

var shortVersionHelp = "Show version details"
var longVersionHelp = `
Show the tool version and exit.
`

type cmdVersion struct{}

func init() {
	addCommand("version", shortVersionHelp, longVersionHelp, func() flags.Commander { return &cmdVersion{} }, nil, nil)
}

func (cmd cmdVersion) Execute(args []string) error {
	if len(args) > 0 {
		return ErrExtraArgs
	}

	return printVersions()
}

func printVersions() error {
	fmt.Fprintf(Stdout, "%s\n", cmd.Version)
	return nil
}
