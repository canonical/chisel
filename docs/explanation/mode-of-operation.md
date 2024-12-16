(chisel_mo_explanation)=
# How it works

The main purpose of Chisel is to create a slice of Ubuntu - a minimal root file system 
comprising only the application and its runtime dependencies.

Such task can be accomplished with the {{cut_cmd}} command, whose workflow is depicted below:

<table style="width: 100%;">
  <colgroup>
    <col style="width: 45%;">
    <col style="width: 55%;">
  </colgroup>

<!-- MO 1 -->
  <tr>
    <td>

```{image} /_static/MO-1.svg
  :align: center
  :alt: Read and parse chisel-releases
```

</td>
    <td>

Chisel fetches, reads and validates the {ref}`chisel-release<chisel-releases_ref>`.
This includes parsing the {ref}`chisel_yaml_ref` and {ref}`slice 
definitions<slice_definitions_ref>` while validating the release and checking for conflicting paths across packages.
    
</td>
  </tr>


<!-- MO 2 -->
  <tr>
    <td>

```{image} /_static/MO-2.svg
  :align: center
  :alt: Talk to archives and fetch packages
```

</td>
    <td>

Chisel talks to the {ref}`chisel_yaml_format_spec_archives` directly.
It fetches, validates and parses their `InRelease` files.
It then resolves which archive holds the **requested** packages and fetches
the corresponding package tarballs.
    
</td>
  </tr>

<!-- MO 3 -->
  <tr>
    <td>

```{image} /_static/MO-3.svg
  :align: center
  :alt: Install package slices
```

</td>
    <td>

Chisel groups and merges all slice definitions per package. Then,
for every package, it extracts the **specified slices' paths** into
the provided root file system.

</td>
  </tr>

<!-- MO 4 -->
  <tr>
    <td>

```{image} /_static/MO-4.svg
  :align: center
  :alt: Run mutation scripts
```

</td>
    <td>

Chisel then runs the {{mutation_scripts}}. Only the
{ref}`mutable<slice_definitions_format_slices_contents_mutable>` files may be
modified at this stage. Finally, the files specified with
{ref}`until:mutate<slice_definitions_format_slices_contents_until>` are
removed from the provided root file system.

    
</td>
  </tr>
</table>
