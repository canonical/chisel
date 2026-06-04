# Preface

This file serves as the local knowledge base index for the `tests/` directory. It outlines the scope and procedures for end-to-end integration testing within the Chisel repository.

# Overview

The `tests/` directory contains the integration test suite for the project. These tests exercise the actual `chisel` binary against real or mock Ubuntu package archives, verifying end-to-end behaviors such as slice extraction, manifest generation, and error handling. The suite is built on the [Spread](https://github.com/canonical/spread) framework for multi-system test execution.

# Important

- **Execution**: Integration tests are run with `spread`, not with `go test`. Each test scenario is a subdirectory containing a `task.yaml` with shell-based assertions.
- **No build tags**: Unlike Go-based integration test suites, these tests require no `//go:build` directives. They are entirely shell-driven.
- **Pre-built binary**: Spread compiles and provisions the `chisel` binary as part of the test environment setup defined in `spread.yaml` at the repository root.

# Directory

- `basic/` - Core slice extraction scenario verifying that files are correctly written to the target root filesystem and that mutation scripts are applied.
- `find/` - Tests for the `chisel find` command, covering search by slice name and package.
- `info/` - Tests for the `chisel info` command, verifying detailed slice metadata output.
- `debug-check-release-archives/` - Tests for the `chisel debug check-release-archives` command, validating archive configuration correctness.
- `pro-archives/` - Tests covering Ubuntu Pro subscription archive support (fips, fips-updates, esm-apps, esm-infra).
- `use-a-custom-chisel-release/` - Tests the ability to override the default chisel-releases with a custom release tree.
- `unmaintained/` - Edge-case tests for packages whose support window has ended.
- `unstable/` - Edge-case tests for packages from unstable or unsupported releases.

# Architecture

Each test scenario is a directory containing a `task.yaml` that defines the test steps as shell commands with assertions (typically `grep`-based). Spread handles multi-system provisioning, binary deployment, and test execution across different Ubuntu releases as configured in the root `spread.yaml`.
