# help command

The **help** command displays information about commands.

## Usage

```{terminal}
:scroll:
:input: chisel help --help

Usage:
  chisel help [help-OPTIONS] [<command>...]

The help command displays information about commands.

[help command options]
      --all          Show a short summary of all commands
```

## Example

```{terminal}
:scroll:
:input: chisel help --all

Chisel can slice a Linux distribution using a release database
and construct a new filesystem using the finely defined parts.

Usage: chisel <command> [<options>...]

Commands can be classified as follows:

  Basic (general operations):
    find     Find existing slices
    info     Show information about package slices
    help     Show help about a command
    version  Show version details

  Action (make things happen):
    cut      Cut a tree with selected slices

For more information about a command, run 'chisel help <command>'.
```
