package main

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/jessevdk/go-flags"
)

var shortHelpHelp = "Show help about a command"
var longHelpHelp = `
The help command displays information about commands.
`

// addHelp adds --help like what go-flags would do for us, but hidden
func addHelp(parser *flags.Parser) error {
	var help struct {
		ShowHelp func() error `short:"h" long:"help"`
	}
	help.ShowHelp = func() error {
		// this function is called via --help (or -h). In that
		// case, parser.Command.Active should be the command
		// on which help is being requested (like "chisel foo
		// --help", active is foo), or nil in the toplevel.
		if parser.Command.Active == nil {
			// this means *either* a bare 'chisel --help',
			// *or* 'chisel --help command'
			//
			// If we return nil in the first case go-flags
			// will throw up an ErrCommandRequired on its
			// own, but in the second case it'll go on to
			// run the command, which is very unexpected.
			//
			// So we force the ErrCommandRequired here.

			// toplevel --help gets handled via ErrCommandRequired
			return &flags.Error{Type: flags.ErrCommandRequired}
		}
		// not toplevel, so ask for regular help
		return &flags.Error{Type: flags.ErrHelp}
	}
	hlpgrp, err := parser.AddGroup("Help Options", "", &help)
	if err != nil {
		return err
	}
	hlpgrp.Hidden = true
	hlp := parser.FindOptionByLongName("help")
	hlp.Description = "Show this help message"
	hlp.Hidden = true

	return nil
}

type cmdHelp struct {
	All        bool `long:"all"`
	Manpage    bool `long:"man" hidden:"true"`
	Positional struct {
		Subs []string `positional-arg-name:"<command>"`
	} `positional-args:"yes"`
	parser *flags.Parser
}

func init() {
	addCommand("help", shortHelpHelp, longHelpHelp, func() flags.Commander { return &cmdHelp{} },
		map[string]string{
			"all": "Show a short summary of all commands",
			"man": "Generate the manpage",
		}, nil)
}

func (cmd *cmdHelp) setParser(parser *flags.Parser) {
	cmd.parser = parser
}

// manfixer is a hackish way to fix drawbacks in the generated manpage:
// - no way to get it into section 8
// - duplicated TP lines that break older groff (e.g. 14.04), lp:1814767
type manfixer struct {
	bytes.Buffer
	done bool
}

func (w *manfixer) Write(buf []byte) (int, error) {
	if !w.done {
		w.done = true
		if bytes.HasPrefix(buf, []byte(".TH chisel 1 ")) {
			// io.Writer.Write must not modify the buffer, even temporarily
			n, _ := w.Buffer.Write(buf[:9])
			w.Buffer.Write([]byte{'8'})
			m, err := w.Buffer.Write(buf[10:])
			return n + m + 1, err
		}
	}
	return w.Buffer.Write(buf)
}

var tpRegexp = regexp.MustCompile(`(?m)(?:^\.TP\n)+`)

func (w *manfixer) flush() error {
	str := tpRegexp.ReplaceAllLiteralString(w.Buffer.String(), ".TP\n")
	_, err := io.Copy(Stdout, strings.NewReader(str))
	return err
}

func (cmd cmdHelp) Execute(args []string) error {
	if len(args) > 0 {
		return ErrExtraArgs
	}
	if cmd.Manpage {
		// you shouldn't try to to combine --man with --all nor a
		// subcommand, but --man is hidden so no real need to check.
		out := &manfixer{}
		cmd.parser.WriteManPage(out)
		err := out.flush()
		return err
	}
	if cmd.All {
		if len(cmd.Positional.Subs) > 0 {
			return fmt.Errorf("help accepts a command, or '--all', but not both.")
		}
		printLongHelp(cmd.parser)
		return nil
	}

	var subcmd = cmd.parser.Command
	for _, subname := range cmd.Positional.Subs {
		subcmd = subcmd.Find(subname)
		if subcmd == nil {
			sug := "chisel help"
			if x := cmd.parser.Command.Active; x != nil && x.Name != "help" {
				sug = "chisel help " + x.Name
			}
			return fmt.Errorf("unknown command %q, see '%s'.", subname, sug)
		}
		// this makes "chisel help foo" work the same as "chisel foo --help"
		cmd.parser.Command.Active = subcmd
	}
	if subcmd != cmd.parser.Command {
		return &flags.Error{Type: flags.ErrHelp}
	}
	return &flags.Error{Type: flags.ErrCommandRequired}
}

type helpCategory struct {
	Label       string
	Description string
	Commands    []string
}

// helpCategories helps us by grouping commands
var helpCategories = []helpCategory{{
	Label:       "Basic",
	Description: "general operations",
	Commands:    []string{"find", "help", "version"},
}, {
	Label:       "Action",
	Description: "make things happen",
	Commands:    []string{"cut"},
}}

var (
	longChiselDescription = strings.TrimSpace(`
Chisel can slice a Linux distribution using a release database
and construct a new filesystem using the finely defined parts.
`)
	chiselUsage               = "Usage: chisel <command> [<options>...]"
	chiselHelpCategoriesIntro = "Commands can be classified as follows:"
	chiselHelpAllFooter       = "For more information about a command, run 'chisel help <command>'."
	chiselHelpFooter          = "For a short summary of all commands, run 'chisel help --all'."
)

func printHelpHeader() {
	fmt.Fprintln(Stdout, longChiselDescription)
	fmt.Fprintln(Stdout)
	fmt.Fprintln(Stdout, chiselUsage)
	fmt.Fprintln(Stdout)
	fmt.Fprintln(Stdout, chiselHelpCategoriesIntro)
}

func printHelpAllFooter() {
	fmt.Fprintln(Stdout)
	fmt.Fprintln(Stdout, chiselHelpAllFooter)
}

func printHelpFooter() {
	printHelpAllFooter()
	fmt.Fprintln(Stdout, chiselHelpFooter)
}

// this is called when the Execute returns a flags.Error with ErrCommandRequired
func printShortHelp() {
	printHelpHeader()
	fmt.Fprintln(Stdout)
	maxLen := 0
	for _, categ := range helpCategories {
		if l := utf8.RuneCountInString(categ.Label); l > maxLen {
			maxLen = l
		}
	}
	for _, categ := range helpCategories {
		fmt.Fprintf(Stdout, "%*s: %s\n", maxLen+2, categ.Label, strings.Join(categ.Commands, ", "))
	}
	printHelpFooter()
}

// this is "chisel help --all"
func printLongHelp(parser *flags.Parser) {
	printHelpHeader()
	maxLen := 0
	for _, categ := range helpCategories {
		for _, command := range categ.Commands {
			if l := len(command); l > maxLen {
				maxLen = l
			}
		}
	}

	// flags doesn't have a LookupCommand?
	commands := parser.Commands()
	cmdLookup := make(map[string]*flags.Command, len(commands))
	for _, cmd := range commands {
		cmdLookup[cmd.Name] = cmd
	}

	for _, categ := range helpCategories {
		fmt.Fprintln(Stdout)
		fmt.Fprintf(Stdout, "  %s (%s):\n", categ.Label, categ.Description)
		for _, name := range categ.Commands {
			cmd := cmdLookup[name]
			if cmd == nil {
				fmt.Fprintf(Stderr, "??? Cannot find command %q mentioned in help categories, please report!\n", name)
			} else {
				fmt.Fprintf(Stdout, "    %*s  %s\n", -maxLen, name, cmd.ShortDescription)
			}
		}
	}
	printHelpAllFooter()
}
