(slices_explanation)=
# Slices

## What are slices?

Since Debian packages are simply archives that can be inspected, navigated and
deconstructed, it is possible to define slices of packages that contain
minimal, complementary, loosely-coupled sets of files based on package metadata
and content. Such **package slices** are subsets of Debian packages, with their
own content and set of dependencies to other internal and external slices.

The use of package slices provides the ability to build minimal root file
system from the wider set of Ubuntu packages.

```{image} /_static/package-slices.svg
  :align: center
  :width: 75%
  :alt: Debian package slices with dependencies
```

This image illustrates the simple case where, at a package level, package _B_
depends on package _A_. However, there might be files in _A_ that _B_ doesn't
actually need, but which are provided for convenience or completeness. By
identifying the files in _A_ that are actually needed by _B_, we can divide _A_
into slices that serve this purpose. In this example, the files in the package
slice, _A_slice3_, are not needed for _B_ to function. To make package _B_
usable in the same way, it can also be divided into slices.

With these slice definitions in place, Chisel is able to extract a
highly-customised and specialised slice of the Ubuntu distribution, which one
could see as a block of stone from which we can carve and extract only the
small and relevant parts that we need to run our applications, thus keeping the
file system small and less exposed to vulnerabilities.

## Defining slices

A package's slices can be defined via a YAML slice definitions file. Check
{ref}`slice_definitions_ref` for more information about this file's format.
