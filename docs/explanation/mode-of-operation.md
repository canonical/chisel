(chisel_mo_explanation)=
# Mode of operation

Chisel is useful for creating minimal root file system based on Ubuntu packages.
The {{cut_cmd}} accomplishes this in the following way:

```{image} /_static/mode-of-operation.png
  :align: center
  :alt: Chisel mode of operation
```

1. Chisel (fetches and) reads the {ref}`chisel-releases_ref` directory. It
  parses the {ref}`chisel_yaml_ref` and {ref}`slice_definitions_ref` files. It
  also validates the release, including checking for conflicting paths across
  packages. Finally, it finds the order of the slices to be installed based on
  {ref}`slice_definitions_format_slices_essential`.

2. Based on the {ref}`chisel_yaml_format_spec_archives` configurations, Chisel
  talks to the archives and fetches, validates and parses the `InRelease` files.
  It then figures out the archive which holds each necessary package and fetches
  the package tarball.

3. For every package, Chisel extracts and creates the specified files in the
  package's slices. Chisel does this once per package. Thus, if there were
  multiple slices of the same package to be installed, the definitions are
  merged in a way so that they can be extracted/created together.

4. After extracting the files from the slices, Chisel runs the
  {{mutation_scripts}} for every slice in the same order of slices. Only the
  {ref}`mutable<slice_definitions_format_slices_contents_mutable>` files may be
  modified in this step. Finally, the files specified with
  {ref}`until:mutate<slice_definitions_format_slices_contents_until>` are
  removed from the root file system.
