(chisel_manifest_ref)=
# Chisel Manifest

The Chisel manifest is a ZSTD-compressed file which lists the metadata about
installed packages, slices and files. The uncompressed file is in the
["jsonwall" format](chisel_manifest_format) - a collection of JSON objects,
one-per-line.

When building fine-grained and yet functional root file systems, Chisel creates
this manifest as a way for ensuring file-level integrity and trace all the
installed package slices, in a format that 3rd party tools (such as SBOM
generators and vulnerability scanners) can work with.


(chisel_manifest_location)=
## Location of the manifest

Chisel manifests may be generated anywhere in the newly created root file system.
To specify the location, Chisel must be instructed to install a slice where at least
one of its contents points to a path with the property
{ref}`generate: manifest<slice_definitions_format_slices_contents_generate>`.

When such a slice is installed, a `manifest.wall` file is generated at the specified
path. If there are multiple paths of this kind being installed, a manifest will be
created in each one of them.

(chisel_manifest_pre-defined_location)=
### Pre-defined location

There is a pre-defined slice named `base-files_chisel` that is available in all
supported Ubuntu releases in the {{chisel_releases_repo}}.


```yaml
package: base-files
slices:
  chisel:
    contents:
      /var/lib/chisel/**: {generate: manifest}
  ...
```

Installing the `base-files_chisel` slice produces a manifest at
`/var/lib/chisel/manifest.wall`.


(chisel_manifest_format)=
## Manifest format

The uncompressed manifest is a "jsonwall" file. This is a custom database file
format where there is one JSON object per line, and both the individual JSON
fields and the lines themselves (except the
{ref}`header<chisel_manifest_jsonwall_header>`) are sorted, a lexicographic order,
to optimize for searching and iterating over the manifest.


(chisel_manifest_jsonwall_header)=
### Header

The "jsonwall" header is a single JSON object on the first line of the file. For example:

```json
{"jsonwall":"1.0","schema":"1.0","count":84}
```

Where:

| Field      | Type  | Required | Description                                                       |
| ---------- | ----- | -------- | ----------------------------------------------------------------- |
| `jsonwall` | `str` | required | version of jsonwall.                                              |
| `schema`   | `str` | required | schema version that Chisel manifest uses.                         |
| `count`    | `int` | required | number of JSON entries in this file, including the header itself. |


(chisel_manifest_packages)=
### Packages

For each package installed, a JSON object with `"kind":"package"` is present in
the manifest. For example:

```json
{"kind":"package","name":"hello","version":"2.10-3build1","sha256":"e68cf4365b7aa9c4e2af4af6eee1710d6f967059b7b4af62786e8870d7366333","arch":"amd64"}
```

Where:

| Field     | Type  | Required | Description                                                   |
| --------- | ----- | -------- | ------------------------------------------------------------- |
| `kind`    | `str` | required | type of JSON object -- must always be `package` for packages. |
| `name`    | `str` | required | name of the package.                                          |
| `version` | `str` | required | version of the package.                                       |
| `sha256`  | `str` | required | digest of the package (in hex format).                        |
| `arch`    | `str` | required | architecture of the package.                                  |


(chisel_manifest_slices)=
### Slices

For each slice installed in the file system, a JSON object with `"kind":"slice"`
is present in the manifest. For example:

```json
{"kind":"slice","name":"hello_bins"}
```

Where:

| Field  | Type  | Required | Description                                               |
| ------ | ----- | -------- | --------------------------------------------------------- |
| `kind` | `str` | required | type of JSON object -- must always be `slice` for slices. |
| `name` | `str` | required | name of the slice, in the `pkg_slice` format.             |


(chisel_manifest_paths)=
### Paths

For each path (file, directory, symlink, etc.) that Chisel installs in the file
system, a JSON object with `"kind":"path"` is present in the manifest. For
example:

```json
{"kind":"path","path":"/etc/ssl/certs/ca-certificates.crt","mode":"0644","slices":["ca-certificates_data"],"sha256":"8f2adf96b87e9da120f700d292f446ffe20062d9f57eaa2449ae67a09af970c3","final_sha256":"6d84ab71cb726c0641b0af84303c316e3fa50db941dc8507d09045eb2fa5d238","size":219342}
{"kind":"path","path":"/lib64","mode":"0777","slices":["base-files_lib"],"link":"usr/lib64"}
{"kind":"path","path":"/run/","mode":"0755","slices":["base-files_var"]}
{"kind":"path","path":"/usr/bin/hello","mode":"0755","slices":["hello_bins"],"sha256":"d288b98ce5f0a3981ea833f3b1d6484dfdde9ee36a00ee3b50bd3a9f7b01f75f","size":26856}
```

Where:

| Field          | Type   | Required | Description                                                                                                                                                                              |
| -------------- | ------ | -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `kind`         | `str`  | required | type of JSON object -- must always be `path` for paths.                                                                                                                                  |
| `path`         | `str`  | required | location of the path.                                                                                                                                                                    |
| `mode`         | `str`  | required | permissions of the path, in an octal value format.                                                                                                                                       |
| `slices`       | `list` | required | the slices that have added or modified this path.                                                                                                                                        |
| `sha256`       | `str`  | optional | original checksum of the file as in the Debian package (in hex format). This attribute is required for all regular files, except the `manifest.wall` file itself, which is an exception. |
| `final_sha256` | `str`  | optional | checksum of the file after it has been modified during installation (in hex format). This attribute is required only for files that have been mutated.                                   |
| `size`         | `int`  | optional | final size of the file, in bytes. This attribute is required for regular files, except the `manifest.wall` file itself, which is an exception.                                           |
| `link`         | `str`  | optional | the target, if the file is a symbolic link.                                                                                                                                              |


(chisel_manifest_list_of_paths_in_slice)=
### List of {ref}`Paths<chisel_manifest_paths>` under a Slice

To state the paths that a slice has added/modified, JSON objects with
`"kind":"content"` are used. For example:

```json
{"kind":"content","slice":"hello_bins","path":"/usr/bin/hello"}
```

Where:

| Field   | Type  | Required | Description                                                         |
| ------- | ----- | -------- | ------------------------------------------------------------------- |
| `kind`  | `str` | required | type of JSON object -- must always be `content` for slice contents. |
| `slice` | `str` | required | name of the slice.                                                  |
| `path`  | `str` | required | location of the path.                                               |
