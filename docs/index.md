# Chisel

**Chisel** is a tool for carving and cutting Ubuntu packages.

It is built on the idea of {{package_slices}} - minimal, complimentary, and
loosely coupled sets of files, based on the package's metadata and content.
Slices are basically subsets of the Ubuntu packages, with their own content and
set of dependencies to other internal and external slices.

Chisel is able to extract a highly customised and specialised _Slice_ of the
Ubuntu distribution, which one could see as a block of stone from which we can
carve and extract the small and relevant parts we need to run our applications.

It operates similar to a package manager, but for package slices, thus being
particularly useful for supporting developers in the creation of smaller but
equally functional container images.

---------

## In this documentation

````{grid} 1 1 2 2

```{grid-item-card} [Tutorial](tutorial/getting-started)

**Get started** - become familiar with Chisel by slicing Ubuntu packages to create
a minimal root file system.
```

```{grid-item-card} [How-to guides](how-to/index)

**Step-by-step guides** - learn key operations and common tasks.
```

````

````{grid} 1 1 2 2
:reverse:

```{grid-item-card} [Reference](reference/index)

**Technical information** - understand the CLI commands, slice definitions files
and Chisel manifests.
```

```{grid-item-card} [Explanations](explanation/index)

**Discussion and clarification** - explore Chisel's mode of operation and learn
about fundamental topics such as package slices.
```

````

---------

## Project and community

Chisel is free software and released under {{AGPL3}}.

The Chisel project is sponsored by {{Canonical}}.

- [Code of conduct](https://ubuntu.com/community/ethos/code-of-conduct)
- [Contribute](https://github.com/canonical/chisel)


```{toctree}
:hidden:
:maxdepth: 2

Tutorial <tutorial/getting-started>
how-to/index
explanation/index
reference/index
```
