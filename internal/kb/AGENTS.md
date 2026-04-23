# Preface

This is the index file for the `internal/` directory's knowledge base. It provides context about the core internal packages of Chisel, encompassing slice orchestration, package setup, extraction, archive fetching, caching, filesystem operations, manifest generation, and supporting utilities.

# Overview

The `internal/` directory houses the core business logic, components, and utilities of Chisel. It coordinates slice selection, dependency resolution, package fetching, file extraction, filesystem mutations, and manifest output. These packages are not part of Chisel's public API and must not be imported by external consumers.

# Directory

- `slicer/` - Main orchestrator for a Chisel run. Receives a slice selection, drives all other internal packages (setup, archive, cache, deb, fsutil, scripts, manifestutil) to completion, and writes the final filesystem and manifest.
- `setup/` - Parses chisel-releases YAML slice definitions, builds the `Release` data model (packages, slices, dependencies), and performs dependency resolution using Tarjan's topological sort algorithm. Also handles fetching package index metadata from remote archives.
- `deb/` - Extracts files from `.deb` archives (AR format with tar/gzip/xz/zstd inner layers). Handles multiple compression formats and preserves file permissions and ordering.
- `archive/` - Manages remote Ubuntu package archive sources over HTTP/HTTPS. Handles PGP signature verification of package indices, credential management for authenticated repositories (e.g. Ubuntu Pro), and HTTP-level caching.
- `cache/` - Content-addressable on-disk cache keyed by SHA256 digest. Stores extracted files and uses hardlinks to the target filesystem to avoid redundant copies. Respects `XDG_CACHE_HOME`.
- `fsutil/` - Core filesystem operations for writing files, directories, and symlinks into the target root filesystem, with correct ownership, permissions, and SHA256 generation during writes.
- `manifestutil/` - Generates the Chisel manifest: a ZSTD-compressed file in jsonwall format recording every installed package, slice, and file. The default filename is `manifest.wall`.
- `scripts/` - Executes Starlark mutation scripts defined in slice definitions. Scripts run after extraction and can transform or clean up files within the target filesystem.
- `control/` - Fast, memory-efficient parser for Debian control files (the metadata sections embedded in `.deb` archives). Uses a two-pass approach: index sections first, then retrieve fields directly on access.
- `strdist/` - String distance and glob matching utilities. Implements a configurable edit-distance algorithm (`Distance`) with pluggable cost functions, and a `GlobPath` function that uses that algorithm to match file paths against patterns supporting `?`, `*`, and `**` wildcards.
- `pgputil/` - Decodes and validates PGP signatures on package archive metadata. Wraps `golang.org/x/crypto/openpgp`.
- `testutil/` - Shared test helpers used across unit tests: mock archive builders, composable content checkers, file presence and permission validators, tree dumpers, and permutation utilities.
- `apacheutil/` - Shared slice-naming utilities (`SliceKey`, name-format regexps, `ParseSliceKey`). The "apache" prefix signals that this package carries an Apache-2.0 license, which is required because it is a transitive dependency of the `public/` packages; a CI script enforces Apache-2.0 on all internal packages reachable from `public/`.
- `apachetestutil/` - Test helpers for reading manifest contents (`DumpManifestContents`), carrying the same Apache-2.0 license requirement as `apacheutil/` because it is depended on by tests in the `public/` packages.

# Architecture

Chisel's internal packages form a directed dependency chain driven by `slicer/`:

```
slicer/ → setup/ (resolve slice deps) → archive/ (fetch package indices)
        → cache/ + deb/ (extract .deb to cache by SHA256)
        → fsutil/ (hardlink/copy from cache to target rootfs)
        → scripts/ (apply Starlark mutations)
        → manifestutil/ (write manifest.wall)
```

`control/` is used by `deb/` and `archive/` for parsing Debian metadata. `pgputil/` is used by `archive/` for signature verification. `strdist/` is used at the CLI layer and in `setup/` for error reporting. `testutil/` is test-only and has no production dependents.
