# Preface

This document describes the scope of the `snap/` directory within the Chisel repository, providing context for automated agents navigating and modifying the snapcraft configuration.

# Directory

- `snapcraft.yaml` - The primary manifest file defining the snap package. It configures classic confinement, utilizes the Go plugin, and outlines the build steps which depend on `cmd/mkversion.sh` for version injection.
