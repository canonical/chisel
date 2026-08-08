# Preface

This repository hosts Chisel, a software tool for carving and cutting Debian packages. This `AGENTS.md` is the root index into the dispersed knowledge base; read it to orient yourself, then follow the links below for details.

Read the top-level `.kb/agents.md` file before continuing below.


# Directory

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


# Documents

- `.kb/agents.md` - General rules for the knowledge base reading and writing.
- `cmd/AGENTS.md` - CLI entry points.
- `internal/AGENTS.md` - Core internal packages.
- `public/AGENTS.md` - Public API packages.
- `snap/AGENTS.md` - Snap packaging.
- `tests/AGENTS.md` - Integration testing.
