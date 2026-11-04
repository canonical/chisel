# Preface

This document covers the snap packaging configuration in the `snap/` directory.

IMPORTANT: Read the `.kb/agents.md` file.


# Laws

## §1 - Strict confinement

The snap must use strict confinement.


# Concepts

## Scope

The `snap/` directory contains the configuration necessary to build and package Chisel as a snap application. It defines the application metadata, execution confinement, and compilation steps for the `chisel` application using Go.

## Contents

- `snapcraft.yaml` - The primary manifest file defining the snap package. It configures strict confinement, utilizes the Go plugin, and outlines the build steps which depend on `cmd/mkversion.sh` for version injection.
