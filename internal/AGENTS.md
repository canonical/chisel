# Preface

This is the index file for the `internal/` directory's knowledge base. It provides context about the core internal packages of Chisel, encompassing slice orchestration, package setup, extraction, archive fetching, caching, filesystem operations, manifest generation, and supporting utilities.

Read the top-level `.kb/agents.md` file before continuing below.

# Overview

The `internal/` directory houses the core business logic, components, and utilities of Chisel. It coordinates slice selection, dependency resolution, package fetching, file extraction, filesystem mutations, and manifest output. These packages are not part of Chisel's public API and must not be imported by external consumers.

# Architecture

Chisel's carving pipeline is coordinated by `slicer/`:

```mermaid
flowchart LR
    CLI --> Release["Acquire and validate release"]
    Release --> Select["Select and order slices"]
    Select --> Fetch["Choose archives and fetch packages"]
    Fetch --> Extract["Extract selected content"]
    Extract --> Create["Create generated content"]
    Create --> Mutate["Run ordered mutations"]
    Mutate --> Manifest["Generate requested manifests"]
```

# Directory

- `slicer/` - Main orchestrator for a Chisel run. Receives a slice selection, drives all other internal packages (setup, archive, cache, deb, fsutil, scripts, manifestutil) to completion, and writes the final filesystem and manifest.
- `setup/` - Parses chisel-releases YAML definitions into the `Release` model and resolves slice dependencies, path conflicts, and package contention.
- `deb/` - Debian package utilities: data tarball access, version comparison, and architecture handling.
- `tarball/` - Extracts selected files from a package data tarball into a target directory.
- `archive/` - Manages remote Ubuntu package archive sources over HTTP/HTTPS.
- `cache/` - Content-addressable on-disk store keyed by SHA256 or SHA384 digest, with digest verification and last-use timestamps. It exposes expiry support, but Chisel does not invoke expiry automatically.
- `fsutil/` - Core filesystem operations for writing files, directories, and symlinks into the target root filesystem.
- `manifestutil/` - Generates the Chisel manifest.
- `scripts/` - Executes Starlark mutation scripts defined in slice definitions.
- `control/` - Parser for Debian control files (the metadata sections embedded in `.deb` archives).
- `strdist/` - String distance and glob matching utilities. Implements a configurable edit-distance algorithm (`Distance`) with pluggable cost functions, and a `GlobPath` function that uses that algorithm to match file paths against patterns supporting `?`, `*`, and `**` wildcards.
- `pgputil/` - Decodes and validates PGP signatures on package archive metadata. Wraps `golang.org/x/crypto/openpgp`.
- `testutil/` - Shared test helpers used across unit tests: mock archive builders, composable content checkers, file presence and permission validators, tree dumpers, and permutation utilities.
- `apacheutil/` - Apache-2.0-licensed slice-naming utilities shared across package boundaries.
- `apachetestutil/` - Apache-2.0-licensed manifest test helpers used by tests in the `public/` packages.
