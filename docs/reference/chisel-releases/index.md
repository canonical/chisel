(chisel-releases_ref)=
# chisel-releases

Chisel uses **slice definition files** (aka SDFs) to define the slices of packages.
SDFs are YAML files, and there is one per package and per Ubuntu release, named
after the package name.

For a given Ubuntu release, the collection of SDFs plus a configuration file
named `chisel.yaml` form what is called a _chisel-release_.

The {{chisel_releases_repo}} contains a number of branches for various
_chisel-releases_, matching the corresponding Ubuntu releases.

A _chisel-release_ is simply a directory with the following structure:

```
├── chisel.yaml
└── slices
    ├── pkgA.yaml
    ├── pkgB.yaml
    └── ...
```

The following pages provide more details on:

```{toctree}
:maxdepth: 1

chisel.yaml
slice-definitions
```
