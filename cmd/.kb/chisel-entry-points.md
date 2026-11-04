# Preface

This document covers the CLI entry points for the `chisel` binary in the `cmd/` directory.

IMPORTANT: Read the `.kb/agents.md` file.


# Laws

## §1 - CLI boundary

Do not add core business logic to `cmd/`; commands only wire up flags and arguments and delegate to the packages in `internal/`.


# Concepts

## Scope

The `cmd/` directory contains the main binary entry points for the `chisel` application, responsible for initializing the application, wiring up the CLI subsystem, and handling version information.

## Debug commands

The `debug` subcommand group has no stability guarantee: its commands may be removed without notice and may not work on non-development systems, so they are registered as hidden.

## Version injection

The application version is injected at build time by `mkversion.sh` (via `go:generate`), which writes the generated `version_generated.go`.

## Testing hooks

Tests override the `Stdin`/`Stdout`/`Stderr`/`ReadPassword` variables instead of touching the `os` streams directly.

## Contents

- `chisel/` - Contains the primary executable entry point for the `chisel` application.
- `chisel/main.go` - Application bootstrap and CLI initialization.
- `chisel/cmd_cut.go` - Implements the `cut` command, the primary operation that extracts slices into a target root filesystem.
- `chisel/cmd_find.go` - Implements the `find` command for searching available slices by name or package.
- `chisel/cmd_info.go` - Implements the `info` command for displaying detailed metadata about specific slices.
- `chisel/cmd_debug.go` - Implements the `debug` subcommand group for additional, development-oriented commands without stability guarantees.
- `chisel/cmd_debug_check_release_archives.go` - Implements `debug check-release-archives`, which validates the release with packages content from the archive.
- `chisel/cmd_version.go` - Implements the `version` command.
- `chisel/cmd_help.go` - Implements the `help` command.
- `chisel/helpers.go` - Shared CLI utilities used across commands.
- `chisel/log.go` - Logging setup for the CLI layer.
- `mkversion.sh` - Shell script used for generating version information during builds.
- `version.go` - Application version data structures and variables.
