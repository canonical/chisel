(chisel-releases_ref)=
# chisel-releases

Chisel uses slice definition files to define the slices for a package. The
collection of these files with a configuration file named `chisel.yaml` is
called a _chisel release_. The {{chisel_releases}} repository contains a number
of releases for various Ubuntu versions.

A _chisel release_ typically has the following directory structure:

```
├── chisel.yaml
└── slices
    ├── pkgA.yaml
    ├── pkgB.yaml
    └── ...
```

The following pages describe in details:

```{toctree}
:maxdepth: 1

chisel.yaml
```
