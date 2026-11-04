# Preface

This document covers the end-to-end integration test suite in the `tests/` directory.

IMPORTANT: Read the `.kb/agents.md` file.


# Laws

## §1 - End-to-end scope

Integration tests must exercise the compiled `chisel` binary end to end, not package internals in isolation.

## §2 - Spread runner

Integration tests must be run with `spread`.


# Concepts

## Scope

The `tests/` directory contains the Spread integration test suite, covering behavior that cannot be validated within individual Go packages.

## Running the suite

Tests are not run with `go test`. Each test scenario is a subdirectory containing a `task.yaml` with shell-based assertions, and requires no `//go:build` directives. Spread compiles and provisions the `chisel` binary as part of the test environment setup defined in `spread.yaml` at the repository root.

## Contents

- `basic/` - Core slice extraction scenario verifying that files are correctly written to the target root filesystem and that mutation scripts are applied.
- `find/` - Tests for the `chisel find` command, covering search by slice name and package.
- `info/` - Tests for the `chisel info` command, verifying detailed slice metadata output.
- `debug-check-release-archives/` - Tests detection of package-content conflicts that cannot be identified from slice definitions alone.
- `pro-archives/` - Tests covering Ubuntu Pro subscription archive support (fips, fips-updates, esm-apps, esm-infra).
- `use-a-custom-chisel-release/` - Tests the ability to override the default chisel-releases with a custom release tree.
- `unmaintained/` - Tests for releases whose support window has ended.
- `unstable/` - Tests for packages from unstable or unsupported releases.
