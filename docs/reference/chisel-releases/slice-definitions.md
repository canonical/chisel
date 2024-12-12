(slice_definitions_ref)=
# Slice definitions

{ref}`Slices <slices_explanation>` are described in slice definition files.
These are YAML files, named by the package.


(slice_definitions_location)=
## Location

The slice definition files are located in the `slices/` directory of
{ref}`chisel-releases_ref`. The slices of package `pkg` are defined in
`slices/pkg.yaml`.

```{tip}
Although the `pkg.yaml` file can be placed in a sub-directory of `slices/` e.g.
`slices/dir/pkg.yaml`, it is generally recommended to keep them at
`slices/pkg.yaml`. The {{chisel_releases_repo}} maintains the latter.
```


(slice_definitions_format)=
## Format specification

This section describes the format of a slice definition file.


(slice_definitions_format_package)=
### `package`

| Field | Type | Required |
| - | - | - |
| `package` | `str` | Required |

The top-level `package` field contains the package name. It must follow the
[Debian policy for package name](https://www.debian.org/doc/debian-policy/ch-binary.html#the-package-name).

As indicated above, the value must match the file basename.

```yaml
package: <str>
```


(slice_definitions_format_archive)=
### `archive`

| Field | Type | Required |
| - | - | - |
| `archive` | `str` | Optional |

The top-level `archive` field specifies a particular
{ref}`archive<chisel_yaml_format_spec_archives>` this package should be fetched
from. If specified, Chisel fetches this package from that archive despite
{ref}`chisel_yaml_format_spec_archives_default` and
{ref}`chisel_yaml_format_spec_archives_priority` settings in
{ref}`chisel_yaml_ref`.

The archive name must be defined in {ref}`chisel_yaml_format_spec_archives`.

```yaml
archive: <str>
```


(slice_definitions_format_essential)=
### `essential`

| Field | Type | Required |
| - | - | - |
| `essential` | `list(str)` | Optional |

The `essential` field lists the slices that are needed for every slice of the
current package. Slices in this list must be written in their full name i.e.
`pkg_slice`. This field is similar to
{ref}`slice_definitions_format_slices_essential`, but applicable for each slice
within the package.

In the following example, the `hello_copyright` slice is an _essential_ for
every slice including the `hello_bins` slice.

```yaml
package: hello
essential:
  - hello_copyright
slices:
  bins:
    contents:
      ...
  copyright:
    ...
```


(slice_definitions_format_slices)=
### `slices`

| Field | Type | Required |
| - | - | - |
| `slices` | `dict(str, dict)` | Required |

The top-level `slices` field describes the slices of a package. The slice names
must consist only of lower case letters(`a-z`), digits(`0-9`) and minus (`-`)
signs. They must be at least three characters long and must start with an
alphanumeric character.

```yaml
slices:
  <name>:
    essential:
      - ...
    contents:
      ...
    mutate: |
      ...
```


(slice_definitions_format_slices_essential)=
### `slices.<name>.essential`

| Field | Type | Required |
| - | - | - |
| `essential` | `list(str)` | Optional |

The `essential` field lists the slices that are needed and must be installed
before the current slice. Slices in this list must be written in their full name
i.e. `pkg_slice`. This field is similar to
{ref}`slice_definitions_format_essential`, but only applicable for the current
slice.

In the following example, `libc6_libs` slice is a requirement for the `bins`
slice and must be installed when installing the `bins` slice.

```yaml
slices:
  bins:
    essential:
      - libc6_libs
```


(slice_definitions_format_slices_contents)=
### `slices.<name>.contents`

| Field | Type | Required |
| - | - | - |
| `contents` | `dict(str, dict)` | Optional |

The `contents` field describes the paths that comes from this slice.

- Paths must be absolute and must start with `/`.
- Paths can have wildcard characters (`?`, `*` and `**`).
  * `?` matches any one character, except for `/`.
  * `*` matches zero or more characters, except for `/`.
  * `**` matches zero or more characters, including `/`.

```yaml
    contents:
      <path>:
        ...
```


(slice_definitions_format_slices_contents_copy)=
### `slices.<name>.contents.<path>.copy`

| Field | Type | Required |
| - | - | - |
| `copy` | `str` | Optional |

The `copy` field refers to the path Chisel should copy this current path from.

In the following example, Chisel copies the `/bin/original` file from the
package onto `/bin/moved`.

```yaml
    contents:
      /bin/moved:
        copy: /bin/original
```

This field is only applicable for paths with no wildcards, and the value
must be an absolute path with no wildcards.


(slice_definitions_format_slices_contents_make)=
### `slices.<name>.contents.<path>.make`

| Field | Type | Required | Supported values |
| - | - | - | - |
| `make` | `bool` | Optional | `true`, `false` |

If `make` is true, Chisel creates the specified directory path. Note that, the
path must be an absolute directory path with a trailing `/`. If
{ref}`mode<slice_definitions_format_slices_contents_mode>` is unspecified, Chisel
creates the directory with `0755`.

```yaml
    contents:
      /path/to/dir/:
        make: true
```

This field is only applicable for paths with no wildcards.


(slice_definitions_format_slices_contents_text)=
### `slices.<name>.contents.<path>.text`

| Field | Type | Required | Supported values |
| - | - | - | - |
| `text` | `str` | Optional | Zero or more characters |

The `text` field instructs Chisel to create a text file with the specified
value as the file content. If empty, Chisel creates an empty file of 0 bytes.

In the following example, `/file` is created with the content `Hello world!`.
If {ref}`mode<slice_definitions_format_slices_contents_mode>` is unspecified,
Chisel creates the file with `0644`.

```yaml
    contents:
      /file:
        text: "Hello world!"
```

This field is only applicable for paths with no wildcards.


(slice_definitions_format_slices_contents_symlink)=
### `slices.<name>.contents.<path>.symlink`

| Field | Type | Required |
| - | - | - |
| `symlink` | `str` | Optional |

The `symlink` field is used to create symbolic links. If specified, Chisel
creates a symlink to the target path specified by the `symlink` value. The value
must be an absolute path with no wildcards.

In the following example, Chisel creates the symlink `/link` which points to
`/file`.

```yaml
    contents:
      /link:
        symlink: /file
```

This field is only applicable for paths with no wildcards.


(slice_definitions_format_slices_contents_mode)=
### `slices.<name>.contents.<path>.mode`

| Field | Type | Required |
| - | - | - |
| `mode` | `int` | Optional |

The `mode` field is used to specify the permission bits for any path Chisel
creates. It takes in a 32 bit unsigned integer, preferably in an octal value
format e.g. `0755` or `0o755`. It can be used with
{ref}`copy<slice_definitions_format_slices_contents_copy>`,
{ref}`make<slice_definitions_format_slices_contents_make>` and
{ref}`text<slice_definitions_format_slices_contents_text>`.

```yaml
    contents:
      /file:
        text: "Hello world!"
        mode: 0755
```

This field is only applicable for paths with no wildcards.


(slice_definitions_format_slices_contents_arch)=
### `slices.<name>.contents.<path>.arch`

| Field | Type | Required | Supported values |
| - | - | - | - |
| `arch` | `str` or `list(str)` | Optional | `amd64`, `arm64`, `armhf`, `i386`, `ppc64el`, `riscv64`, `s390x` |

The `arch` field is used to specify the package architectures a path should be
installed for. This field can take a single architecture string as the value or
a list.

In the following example, `/foo` will be installed for `i386` installations and
`/bar` will be installed for `amd64` or `arm64` installations.

```yaml
    contents:
      /foo:
        arch: i386
      /bar:
        arch: [amd64, arm64]
```


(slice_definitions_format_slices_contents_mutable)=
### `slices.<name>.contents.<path>.mutable`

| Field | Type | Required | Supported values |
| - | - | - | - |
| `mutable` | `bool` | Optional | `true`, `false` |

The `mutable` flag, if set to `true`, indicates that this path can be later
_mutated_ (modified) in the {{mutation_scripts}}.


(slice_definitions_format_slices_contents_until)=
### `slices.<name>.contents.<path>.until`

| Field | Type | Required | Supported values |
| - | - | - | - |
| `until` | `str` | Optional | `mutate` |

The `until` field indicates that the path will be available until a certain
event takes place. The file is eventually not installed in the final root file
system.

It currently accepts only one value - `mutate`. If specified, the path will only
be available till the {{mutation_scripts}} execute and removed afterwards.

In the following example, `/file` will not be installed in the final root file
system but will exist throughout the mutation scripts execution.

```yaml
    contents:
      /file:
        until: mutate
```


(slice_definitions_format_slices_contents_generate)=
### `slices.<name>.contents.<path>.generate`

| Field | Type | Required | Supported values |
| - | - | - | - |
| `generate` | `str` | Optional | `manifest` |

The `generate` field is used to specify a location Chisel should produce
metadata at. This field can only be used on certain type of paths and those
paths must not have any other fields specified to it.

The specified path must be a directory and must end with double-asterisks (`**`)
which matches everything inside that directory. Additionally, the path must not
contain any other wildcard characters except the trailing double-asterisks
(`**`).

Currently, `generate` only accepts one value - `manifest`. If specified, Chisel
creates the {ref}`chisel_manifest_ref` file `manifest.wall` in that directory.

In the following example, Chisel creates the `/var/lib/chisel` directory with
`0755` mode and produces a `manifest.wall` file within the directory which
contains the compressed manifest.

```yaml
    contents:
      /var/lib/chisel/**:
        generate: manifest
```


(slice_definitions_format_slices_mutate)=
### `slices.<name>.mutate`

| Field | Type | Required | Supported values |
| - | - | - | - |
| `mutate` | `str` | Optional | {{Starlark}} script |

The `mutate` field describes a slice's mutation scripts. The mutation scripts
are similar to that of [Debian's `postinst` maintainer
script](https://www.debian.org/doc/debian-policy/ch-maintainerscripts.html), but
these ones are written in Google's {{Starlark}} language.

The mutation scripts are executed after the files of every slice have been
installed in the root file system. The mutation scripts are run once per each
installed slice, in the same order of slices.

In addition to {{Starlark}}'s native syntax, Chisel introduces the following
functions which can used in the mutation scripts:

| Function | Return type | Description |
| - | - | - |
| `content.list(d)` | `list(str)` | Lists and returns directory `d` contents (similar to GNU `ls`) |
| `content.read(f)` | `str` | Reads a text file `f` and returns its contents |
| `content.write(f, s)` | - | Writes a file `f` with text content `s` |

In the following example of `ca-certificates_data` slice, Chisel initially
creates the `/etc/ssl/certs/ca-certificates.crt` text file with `FIXME` as its
content. When the mutation scripts execute, Chisel concatenates the contents of
every file in `/usr/share/ca-certificates/mozilla/` directory and writes the
concatenated data to the previously created `/etc/ssl/certs/ca-certificates.crt`
file.

```yaml
package: ca-certificates
slices:
  data:
    essential:
      - ...
    contents:
      /etc/ssl/certs/ca-certificates.crt: {text: FIXME, mutable: true}
      /usr/share/ca-certificates/mozilla/: {until: mutate}
      /usr/share/ca-certificates/mozilla/**: {until: mutate}
    mutate: |
      certs_dir = "/usr/share/ca-certificates/mozilla/"
      certs = [
        content.read(certs_dir + path) for path in content.list(certs_dir)
      ]
      content.write("/etc/ssl/certs/ca-certificates.crt", "".join(certs))
```

Due to the usage of `until`, the `/usr/share/ca-certificates/mozilla/` directory
and the files inside are not present in the final root file system.


(slice_definitions_example)=
## Example

The slice definition files can be found in the {{chisel_releases_repo}}, or
inspected via the {{info_cmd}}. Here is a short example of the `hello` package
slice definitions:

```yaml
package: hello

essential:
  - hello_copyright

slices:
  bins:
    essential:
      - libc6_libs
    contents:
      /usr/bin/hello:

  copyright:
    contents:
      /usr/share/doc/hello/copyright:
```
