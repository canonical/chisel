# Chisel

**Chisel** is a tool for carving and cutting Debian packages.

It can derive a minimal Ubuntu-like Linux distribution using a release
database that defines "slices" of Debian packages. Slices enable users to
cherry-pick just the files they need from the Ubuntu archives, and combine them
to create a new root file system which can be packaged.

If you need a way to create a minimal root file system based on Ubuntu
packages, Chisel might be for you. It handles package slice dependency,
fetching Ubuntu Pro packages, and allows you to create a fully operational but
minimal root file system for your application.

Chisel is particularly useful for developers who are creating minimal
containers such as {{Rocks}}, or {{Docker}} images.

---------

## In this documentation

````{grid} 1 1 2 2

```{grid-item-card} [Tutorial](tutorial/getting-started)

**Start here**: a hands-on introduction to Chisel for new users
```

```{grid-item-card} [How-to guides](how-to/index)

**Step-by-step guides** covering key operations and common tasks
```

````

````{grid} 1 1 2 2
:reverse:

```{grid-item-card} [Reference](reference/index)

**Technical information** - specifications, syntax, architecture
```

```{grid-item-card} [Explanations](explanation/index)

**Discussion and clarification** of key topics
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
