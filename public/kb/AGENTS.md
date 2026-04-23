# Preface

This document describes the scope of the `public/` directory, which contains Chisel's public API packages intended for consumption by external tools.

# Overview

The `public/` directory houses the two packages that form Chisel's stable public contract. These packages define the on-disk format and data schema for the Chisel manifest, enabling third-party tools such as SBOM generators and vulnerability scanners to consume Chisel output without depending on internal packages.

# Directory

- `jsonwall/` - Defines and implements the "jsonwall" database format: a ZSTD-compressed text file with one JSON object per line, with fields sorted for efficient search and iteration. This is the storage format used for the Chisel manifest.
- `manifest/` - Defines the manifest entry schema (schema version 1.0), including the `Package`, `Slice`, and `File` record types with their fields (`Kind`, `Name`, `Version`, `Digest`, `Arch`, etc.). Integrates with `jsonwall` for serialization and deserialization.
