# Preface

This document covers the public API packages in the `public/` directory.

IMPORTANT: Read the `.kb/agents.md` file.


# Laws

## §1 - License boundary

Packages in `public/` and their dependencies must remain Apache-2.0 licensed.


# Concepts

## Scope

The `public/` directory houses the two packages that form Chisel's stable public contract. They provide a generic sorted JSON-lines database format and the Chisel manifest schema built on it, enabling third-party tools such as SBOM generators and vulnerability scanners to consume Chisel output without depending on internal packages.

## License check

When changing these packages or their dependencies, run `go run .github/scripts/external-packages-license-check.go` to verify the license boundary.

## Contents

- `jsonwall/` - Implements a generic database format with one JSON object per line, with fields and entries sorted for efficient search and iteration.
- `manifest/` - Defines Chisel manifest schema 1.0 through the `Package`, `Slice`, `Path`, and `Content` record types, and uses `jsonwall` for serialization and deserialization. The carving pipeline applies Zstandard compression when generating manifest files.
