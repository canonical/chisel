# Preface

This document describes the scope of the `public/` directory, which contains Chisel's public API packages intended for consumption by external tools.

Read the top-level `.kb/agents.md` file before continuing below.

# Overview

The `public/` directory houses the two packages that form Chisel's stable public contract. They provide a generic sorted JSON-lines database format and the Chisel manifest schema built on it, enabling third-party tools such as SBOM generators and vulnerability scanners to consume Chisel output without depending on internal packages.

# Important

- When changing these packages or their dependencies, run `go run .github/scripts/external-packages-license-check.go` to verify the Apache-2.0 license boundary.

# Directory

- `jsonwall/` - Implements a generic database format with one JSON object per line, with fields and entries sorted for efficient search and iteration.
- `manifest/` - Defines Chisel manifest schema 1.0 through the `Package`, `Slice`, `Path`, and `Content` record types, and uses `jsonwall` for serialization and deserialization. The carving pipeline applies Zstandard compression when generating manifest files.
