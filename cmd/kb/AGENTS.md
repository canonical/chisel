# Preface

This document serves as the local knowledge base index for the `cmd/` directory. It outlines the scope and purpose of the entry points for the `chisel` binary.

# Overview

The `cmd/` directory contains the main binary entry points for the `chisel` application. Instead of housing core business logic, this directory is responsible for initializing the application, wiring up the CLI subsystem, and handling version information. Core logic is deferred to the packages in `internal/`.

# Directory

- `chisel/` - Contains the primary executable entry point for the `chisel` application.
- `chisel/main.go` - Application bootstrap and CLI initialization using `jessevdk/go-flags`.
- `chisel/cmd_cut.go` - Implements the `cut` command, the primary operation that extracts slices into a target root filesystem.
- `chisel/cmd_find.go` - Implements the `find` command for searching available slices by name or package.
- `chisel/cmd_info.go` - Implements the `info` command for displaying detailed metadata about specific slices.
- `chisel/cmd_debug.go` - Implements the `debug` subcommand group for internal diagnostics.
- `chisel/cmd_debug_check_release_archives.go` - Implements `debug check-release-archives`, which validates archive configurations in a chisel-releases tree.
- `chisel/cmd_version.go` - Implements the `version` command.
- `chisel/cmd_help.go` - Implements the `help` command.
- `chisel/helpers.go` - Shared CLI utilities used across commands.
- `chisel/log.go` - Logging setup for the CLI layer.
- `mkversion.sh` - Shell script used for generating version information during builds.
- `version.go` - Application version data structures and variables.
