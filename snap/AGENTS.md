# Preface

This document describes the scope of the `snap/` directory within the Chisel repository, providing context for automated agents navigating and modifying the snapcraft configuration.

Read the top-level `.kb/agents.md` file before continuing below.

# Overview

The `snap/` directory contains the configuration necessary to build and package Chisel as a snap application. It defines the application metadata, execution confinement, and compilation steps for the `chisel` application using Go.

# Directory

- `snapcraft.yaml` - The primary manifest file defining the snap package. It configures classic confinement, utilizes the Go plugin, and outlines the build steps which depend on `cmd/mkversion.sh` for version injection.
