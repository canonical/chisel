# Preface

This document holds the top-level overview of the Chisel repository.

IMPORTANT: Read the `.kb/agents.md` file.


# Laws

## §1 - Deterministic behavior

Behavior must be deterministic and predictable, failing on the side of correctness and preserving existing user content by default.

## §2 - Provider independence

Chisel's operation and logic, including validation, must not rely on package contents or on information from package providers (currently the archive, in the future the bin store). Do as much as possible without fetching from package providers, so that operations stay fast and can work without network access.

## §3 - Compatibility

Do not break compatibility of the chisel-releases format or the public manifest packages, backward or forward, without a compelling reason.


# Concepts

## Purpose

Chisel cuts fine-grained slices out of Debian packages: it takes a selection of slices defined in chisel-releases, resolves their dependencies, downloads the corresponding packages from Ubuntu archives, extracts the selected files into a target root filesystem, runs Starlark mutation scripts, and generates manifests at paths requested by the selected slice definitions. The codebase is split across command entry points, an internals tree holding the carving pipeline, and public packages defining the manifest format.

## Compatibility guidance

When introducing a new feature or fixing a bug, consider both backward and forward compatibility: new behavior must keep working with previous Chisel releases, and parsing of release YAML files must accept or reject values so that future format evolutions remain possible.

## Contents

- `cmd/` - Binary entry points for the `chisel` application.
- `internal/` - Core internal packages: slice orchestration, setup, extraction, archive, cache, and supporting utilities.
- `public/` - Public API packages for external consumers.
- `snap/` - Snap packaging configuration for Chisel.
- `tests/` - End-to-end integration test suite.
- `docs/` - Documentation assets.
- `go.mod` - Go module definition.
- `go.sum` - Go dependency checksums.
- `spread.yaml` - Spread test configuration.
- `workshop.yaml` - Workshop configuration.
- `.golangci.yaml` - golangci-lint configuration.
- `README.md` - Project readme.
- `SECURITY.md` - Security policy.
- `LICENSE` - Project license.
