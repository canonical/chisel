(slice_definitions_ref)=
# Slice definitions

{ref}`Slices <slices_explanation>` are described in slice definitions files (aka SDFs).
These are YAML files, named after the package name.


(slice_definitions_location)=
## Location

The slice definitions files are located in the `slices/` directory of
{ref}`chisel-releases_ref`. And the slices for a given package `hello` are defined in
`slices/hello.yaml`.

```{tip}
Although the `hello.yaml` file can be placed in a sub-directory of `slices/` e.g.
`slices/dir/hello.yaml`, it is generally recommended to keep them at
`slices/hello.yaml`. The {{chisel_releases_repo}} follows the latter.
```


(slice_definitions_format)=
## Format specification

(slice_definitions_format_package)=
### `package`

| Field     | Type  | Required |
| --------- | ----- | -------- |
| `package` | `str` | Required |

Indicates the package name. It must follow the
[Debian policy for package name](https://www.debian.org/doc/debian-policy/ch-binary.html#the-package-name).

As indicated above, the value must also match the YAML file basename.
For example:

```yaml
package: hello
```


(slice_definitions_format_archive)=
### `archive`

| Field     | Type  | Required | Supported values                                                      |
| --------- | ----- | -------- | --------------------------------------------------------------------- |
| `archive` | `str` | Optional | Archive name, from {ref}`archives<chisel_yaml_format_spec_archives>`. |

Specifies a particular
{ref}`archive<chisel_yaml_format_spec_archives>` from where this package should be
fetched from. If specified, Chisel fetches this package from that archive despite the
{ref}`chisel_yaml_format_spec_archives_default` and
{ref}`chisel_yaml_format_spec_archives_priority` settings in
{ref}`chisel_yaml_ref`.

The archive name must be defined in {ref}`chisel_yaml_format_spec_archives`.
For example:

```yaml
archive: ubuntu
```


(slice_definitions_format_essential)=
### `essential`

| Field       | Type        | Required | Supported values   |
| ----------- | ----------- | -------- | ------------------ |
| `essential` | `list(str)` | Optional | An existing slice. |

Lists the slices that are needed for **every slice** of the
current package. Slices in this list must be written in their full name, e.g.
`hello_copyright`. This field is similar to
{ref}`slice_definitions_format_slices_essential`, but applicable for every slice
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

| Field    | Type              | Required |
| -------- | ----------------- | -------- |
| `slices` | `dict(str, dict)` | Required |

Defines the slices of a package. 

The slice names must consist only of lower case letters(`a-z`), digits(`0-9`)
and minus (`-`) signs. They must be at least three characters long and must start
with an alphanumeric character.

For example, a slice definition called `data`, for the `ca-certificates` package,
could look like the following:

```yaml
slices:
  data:
    essential:
      - openssl_data
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


(slice_definitions_format_slices_essential)=
### `slices.<name>.essential`

| Field       | Type        | Required | Supported values   |
| ----------- | ----------- | -------- | ------------------ |
| `essential` | `list(str)` | Optional | An existing slice. |

Lists the slices that are needed and that must be installed before the current slice.
Slices in this list must be written in their full name
e.g. `hello_copyright`. This field is similar to
{ref}`slice_definitions_format_essential`, but only applicable for the current
slice.

In the following example, `libc6_libs` is a requirement for the `bins`
slice and must be installed when installing the `bins` slice.

```yaml
slices:
  bins:
    essential:
      - libc6_libs
```


(slice_definitions_format_slices_contents)=
### `slices.<name>.contents`

| Field      | Type              | Required |
| ---------- | ----------------- | -------- |
| `contents` | `dict(str, dict)` | Optional |

Describes the paths that come from this slice.

```{note}
Paths must be absolute and must start with `/`.


Also, paths can have wildcard characters (`?`, `*` and `**`), where
 * `?` matches any one character, except for `/`,
 * `*` matches zero or more characters, except for `/`, and
 * `**` matches zero or more characters, including `/`.
```

(slice_definitions_format_slices_contents_copy)=
### `slices.<name>.contents.<path>.copy`

| Field  | Type  | Required |
| ------ | ----- | -------- |
| `copy` | `str` | Optional |

The `copy` field refers to the path Chisel should copy the target path from.

In the following example, Chisel copies the `/bin/original` file from the
package onto `/bin/moved`.

```yaml
    contents:
      /bin/moved:
        copy: /bin/original
```

```{note}
This field is only applicable to paths with no wildcards, and its value
must also be an absolute path with no wildcards.
```

(slice_definitions_format_slices_contents_make)=
### `slices.<name>.contents.<path>.make`

| Field  | Type   | Required | Supported values |
| ------ | ------ | -------- | ---------------- |
| `make` | `bool` | Optional | `true`, `false`  |

If `make` is true, Chisel creates the specified directory path. Note that, the
path must be an absolute directory path with a trailing `/`. If
{ref}`mode<slice_definitions_format_slices_contents_mode>` is not specified, Chisel
creates the directory with `0755`.

```yaml
    contents:
      /path/to/dir/:
        make: true
```

```{note}
This field is only applicable for paths with no wildcards.
```

(slice_definitions_format_slices_contents_text)=
### `slices.<name>.contents.<path>.text`

| Field  | Type  | Required |
| ------ | ----- | -------- |
| `text` | `str` | Optional |

The `text` field instructs Chisel to create a text file with the specified
value as the file content. If empty, Chisel creates an empty file of 0 bytes.

In the following example, `/file` is created with the content `Hello world!`.
If {ref}`mode<slice_definitions_format_slices_contents_mode>` is not specified,
Chisel creates the file with `0644`.

```yaml
    contents:
      /file:
        text: "Hello world!"
```

```{note}
This field is only applicable for paths with no wildcards.
```

(slice_definitions_format_slices_contents_symlink)=
### `slices.<name>.contents.<path>.symlink`

| Field     | Type  | Required |
| --------- | ----- | -------- |
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

```{note}
This field is only applicable for paths with no wildcards.
```

(slice_definitions_format_slices_contents_mode)=
### `slices.<name>.contents.<path>.mode`

| Field  | Type  | Required |
| ------ | ----- | -------- |
| `mode` | `int` | Optional |

The `mode` field is used to specify the permission bits for any path Chisel
creates. It takes in a 32 bit unsigned integer, preferably in an octal value
format e.g. `0755` or `0o755`. For example:

```yaml
    contents:
      /file:
        text: "Hello world!"
        mode: 0755
```

```{note}
It can only be used with
{ref}`copy<slice_definitions_format_slices_contents_copy>`,
{ref}`make<slice_definitions_format_slices_contents_make>` and
{ref}`text<slice_definitions_format_slices_contents_text>`.

This field is only applicable for paths with no wildcards.
```

(slice_definitions_format_slices_contents_arch)=
### `slices.<name>.contents.<path>.arch`

| Field  | Type                 | Required | Supported values                                                 |
| ------ | -------------------- | -------- | ---------------------------------------------------------------- |
| `arch` | `str` or `list(str)` | Optional | `amd64`, `arm64`, `armhf`, `i386`, `ppc64el`, `riscv64`, `s390x` |

Used to specify the package architectures a path should be
installed for. This field can take a single architecture string or a list, as its
value.

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

| Field     | Type   | Required | Supported values |
| --------- | ------ | -------- | ---------------- |
| `mutable` | `bool` | Optional | `true`, `false`  |

If `mutable` f set to `true`, indicates that this path can be later
_mutated_ (modified) by the {{mutation_scripts}}.


(slice_definitions_format_slices_contents_until)=
### `slices.<name>.contents.<path>.until`

| Field   | Type  | Required | Supported values |
| ------- | ----- | -------- | ---------------- |
| `until` | `str` | Optional | `mutate`         |

The `until` field indicates that the path will be available until a certain
event takes place. The file is eventually not installed in the final root file
system.

It currently accepts only one value - `mutate`. If specified, the path will only
be available until the {{mutation_scripts}} execute, and is removed afterwards.

In the following example, `/file` will not be installed in the final root file
system but will exist throughout the execution of the {{mutation_scripts}}.

```yaml
    contents:
      /file:
        until: mutate
```


(slice_definitions_format_slices_contents_generate)=
### `slices.<name>.contents.<path>.generate`

| Field      | Type  | Required | Supported values |
| ---------- | ----- | -------- | ---------------- |
| `generate` | `str` | Optional | `manifest`       |

Used to specify the location where Chisel should produce metadata at. The path
this field applies to must not have any other fields applied to it.

The specified path must be a directory and must end with double-asterisks (`**`).
Additionally, the path must not contain any other wildcard characters except the trailing double-asterisks (`**`).

Currently, `generate` only accepts one value - `manifest`. If specified, Chisel
creates the {ref}`chisel_manifest_ref` file in that directory.

In the following example, Chisel creates the `/var/lib/chisel` directory with
`0755` mode and produces a {ref}`"manifest.wall"<chisel_manifest_ref>` file within the directory.

```yaml
    contents:
      /var/lib/chisel/**:
        generate: manifest
```


(slice_definitions_format_slices_mutate)=
### `slices.<name>.mutate`

| Field    | Type  | Required | Supported values    |
| -------- | ----- | -------- | ------------------- |
| `mutate` | `str` | Optional | {{Starlark}} script |

Describes a slice's mutation scripts. The mutation scripts are conceptually similar
to [Debian's maintainer
script](https://www.debian.org/doc/debian-policy/ch-maintainerscripts.html).

The mutation scripts are written in Google's {{Starlark}} language and are executed
after the files of every slice have been installed in the root file system. The
mutation scripts are run once per each installed slice, in the same order of
slices.

In addition to {{Starlark}}'s native syntax, Chisel introduces the following
functions:

| Function              | Return type | Description                                                      |
| --------------------- | ----------- | ---------------------------------------------------------------- |
| `content.list(d)`     | `list(str)` | Lists and returns directory `d`'s contents (similar to GNU `ls`) |
| `content.read(f)`     | `str`       | Reads a text file `f` and returns its contents                   |
| `content.write(f, s)` | -           | Writes the text content `s` to a file `f`                        |

Reusing the above {ref}`"ca-certificates_data"<slice_definitions_format_slices>` example,
Chisel initially creates the `/etc/ssl/certs/ca-certificates.crt` text file with `FIXME` as its content. When the mutation scripts execute, Chisel concatenates the contents of
every file in the `/usr/share/ca-certificates/mozilla/` directory and writes the
concatenated data to the previously created `/etc/ssl/certs/ca-certificates.crt`
file.

```yaml
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

The slice definitions files can be found in the {{chisel_releases_repo}}, or
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
