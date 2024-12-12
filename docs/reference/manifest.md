(chisel_manifest_ref)=
# Chisel Manifest

Chisel manifest is a ZSTD-compressed file which lists the metadata about
installed packages, slices and files. The uncompressed file is in the
["jsonwall" format](/explanation/jsonwall) - a collection of JSON objects,
one-per-line. The lines except the header are sorted in lexicographical-order
for better compression.


(chisel_manifest_location)=
## Location of the manifest

Chisel manifest may be generated anywhere in the newly created root file system.
To specify the location, the
{ref}`slice_definitions_format_slices_contents_generate` keyword is used in a
slice definition.  If the slice is installed, a `manifest.wall` file is
generated at the specified location.

Multiple paths may be specified and a manifest will be created in each of those
paths, if the respective slices are installed.


(chisel_manifest_pre-defined_location)=
### Pre-defined location

The `base-files_chisel` slice lists the `/var/lib/chisel` directory as the
location to generate the manifest at.

```yaml
package: base-files
slices:
  chisel:
    contents:
      /var/lib/chisel/**: {generate: manifest}
    ...
```

This slice is available in all supported branches of the upstream
{{chisel_releases_repo}}. Thus, installing the `base-files_chisel` slice
produces a manifest at `/var/lib/chisel/manifest.wall`.


(chisel_manifest_format)=
## Manifest format

The uncompressed manifest file consists of a few different types of JSON
objects.


(chisel_manifest_jsonwall_header)=
### jsonwall Header

The header is a single JSON object on the first line in the following format:

```json
{"jsonwall":"1.0","schema":"1.0","count":84}
```

This JSON object has the following attributes:

| Field | Type | Required | Description |
| - | - | - | - |
| `jsonwall` | `str` | required | the version of jsonwall. |
| `schema` | `str` | required | the schema version that Chisel manifest uses. |
| `count` | `int` | required | the number of JSON entries (or, lines) in this file, including the header itself. |


(chisel_manifest_packages)=
### Packages

For each package installed, a JSON object with `"kind":"package"` is present in
the manifest.

```json
{"kind":"package","name":"foo","version":"1.0-2","sha256":"abcd...","arch":"amd64"}
```

These JSON objects have the following attributes:

| Field | Type | Required | Description |
| - | - | - | - |
| `kind` | `str` | required | type of JSON object -- the value is always `package`. |
| `name` | `str` | required |	name of the package. |
| `version` | `str` | required |	version of the package. |
| `sha256` | `str` | required | digest of the package (in hex format). |
| `arch` | `str` | required |	architecture of the package. |


(chisel_manifest_slices)=
### Slices

For each slice installed in the file system, a JSON object with `"kind":"slice"`
is present in the manifest.

```json
{"kind":"slice","name":"foo_bar"}
```

These JSON objects have two attributes:

| Field | Type | Required | Description |
| - | - | - | - |
| `kind` | `str` | required |	type of JSON object -- the value is always `slice`. |
| `name` | `str` | required |	name of the slice, in the `pkg_slice` format. |


(chisel_manifest_paths)=
### Paths

For each path (file, directory, symlink etc.) that Chisel installs in the file
system, a JSON object with `"kind":"path"` is present in the manifest.

```json
{"kind":"path","path":"/etc/","mode":"0755","slices":["foo_bar","abc_xyz"]}
{"kind":"path","path":"/etc/foo","mode":"0644","sha256":"abcd...","final_sha256":"abcd...","size":1234,"slices":["foo_bar"]}
{"kind":"path","path":"/etc/bar","mode":"0777","link":"/etc/foo","slices":["foo_bar"]}
```

These JSON objects have the following attributes:

| Field | Type | Required | Description |
| - | - | - | - |
| `kind` | `str` | required | type of JSON object -- the value is always `path`. |
| `path` | `str` | required | location of the path. |
| `mode` | `str` | required | the permissions of the path, in an octal value format. |
| `slices` | `list` | required | the slices that have added or modified this path. |
| `sha256` | `str` | optional | the original checksum of the file as in the package (in hex format). This attribute is required for all regular files, except the `manifest.wall` file itself, which is an exception. |
| `final_sha256` | `str` | optional |	the checksum of the file after it has been modified during installation (in hex format). This attribute is required only for files that have been mutated. |
| `size` | `int` | optional | the final size of the file, in bytes. This attribute is required for regular files, except the `manifest.wall` file itself, which is an exception. |
| `link` | `str` | optional | the target, if the file is a symbolic link. |

```{note}
Unless explicitly included in the requested slices, the parent directories that
are implicitly created in the file system are not written in the Chisel
manifest.  In the {ref}`example above<chisel_manifest_paths>`, it is assumed
that `/etc/` is explicitly defined, with the same properties, in both `foo_bar`
and `abc_xyz`.
```


(chisel_manifest_list_of_paths_in_slice)=
### List of Paths under a Slice

To state the path that a slice has added/modified, JSON objects with
`"kind":"content"` are used.

```json
{"kind":"content","slice":"foo_bar","path":"/etc/foo"}
```

It has the following attributes:

| Field | Type | Required | Description |
| - | - | - | - |
| `kind` | `str` | required | type of JSON object -- the value is always `content`. |
| `slice` | `str` | required | name of the slice. |
| `path` | `str` | required | location of the path. |


(chisel_manifest_example)=
## Example

This is an example manifest file (uncompressed), generated from installing the
`base-files_chisel` and `hello_bins` slices:

```json
{"jsonwall":"1.0","schema":"1.0","count":75}
{"kind":"content","slice":"base-files_chisel","path":"/var/lib/chisel/manifest.wall"}
{"kind":"content","slice":"base-files_copyright","path":"/usr/share/doc/base-files/copyright"}
{"kind":"content","slice":"base-files_var","path":"/run/"}
{"kind":"content","slice":"base-files_var","path":"/var/cache/"}
{"kind":"content","slice":"base-files_var","path":"/var/lib/"}
{"kind":"content","slice":"base-files_var","path":"/var/log/"}
{"kind":"content","slice":"base-files_var","path":"/var/run"}
{"kind":"content","slice":"base-files_var","path":"/var/tmp/"}
{"kind":"content","slice":"hello_bins","path":"/usr/bin/hello"}
{"kind":"content","slice":"hello_copyright","path":"/usr/share/doc/hello/copyright"}
{"kind":"content","slice":"libc6_copyright","path":"/usr/share/doc/libc6/copyright"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/ld-linux-x86-64.so.2"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/libBrokenLocale.so.1"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/libanl.so.1"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/libc.so.6"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/libc_malloc_debug.so.0"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/libdl.so.2"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/libm.so.6"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/libmemusage.so"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/libmvec.so.1"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/libnsl.so.1"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/libnss_compat.so.2"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/libnss_dns.so.2"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/libnss_files.so.2"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/libnss_hesiod.so.2"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/libpcprofile.so"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/libpthread.so.0"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/libresolv.so.2"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/librt.so.1"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/libthread_db.so.1"}
{"kind":"content","slice":"libc6_libs","path":"/lib/x86_64-linux-gnu/libutil.so.1"}
{"kind":"content","slice":"libc6_libs","path":"/lib64/ld-linux-x86-64.so.2"}
{"kind":"package","name":"base-files","version":"12ubuntu4.7","sha256":"543f50e12da693710e1d606af61029fa599cde7eb0eaee2c0e34fa47848f4533","arch":"amd64"}
{"kind":"package","name":"hello","version":"2.10-2ubuntu4","sha256":"750335248ccc68d07397e2b843d94fd1a164ddeca23892ca8398b5d528cd89eb","arch":"amd64"}
{"kind":"package","name":"libc6","version":"2.35-0ubuntu3.8","sha256":"76d582e6b5a7057acd8b239edf102329a5a966303d7d1b7a024b447e057b342e","arch":"amd64"}
{"kind":"path","path":"/lib/x86_64-linux-gnu/ld-linux-x86-64.so.2","mode":"0755","slices":["libc6_libs"],"sha256":"595bcdc306999711a5a9c787e5d01618f94eb92c4c09a178e875a324f1072d89","size":240936}
{"kind":"path","path":"/lib/x86_64-linux-gnu/libBrokenLocale.so.1","mode":"0644","slices":["libc6_libs"],"sha256":"b7c4634aba936e23121b6eb11f922cb2f2522908e7f6122969f15f5e6304baca","size":14664}
{"kind":"path","path":"/lib/x86_64-linux-gnu/libanl.so.1","mode":"0644","slices":["libc6_libs"],"sha256":"2828144cd00231b0b36317664d1a2edbdf4f24e5b3831a1daf08ef85974e527e","size":14432}
{"kind":"path","path":"/lib/x86_64-linux-gnu/libc.so.6","mode":"0755","slices":["libc6_libs"],"sha256":"5955dead1a55f545cf9cf34a40b2eb65deb84ea503ac467a266d061073315fa7","size":2220400}
{"kind":"path","path":"/lib/x86_64-linux-gnu/libc_malloc_debug.so.0","mode":"0644","slices":["libc6_libs"],"sha256":"bbe982205aa201813cc3ba15491180e8ea54e9380434e02676de47615de0b5de","size":56704}
{"kind":"path","path":"/lib/x86_64-linux-gnu/libdl.so.2","mode":"0644","slices":["libc6_libs"],"sha256":"15ba6ee5171eb72f8317c22e2c18ac1a1174e6e274fa458be17550764ef8e993","size":14432}
{"kind":"path","path":"/lib/x86_64-linux-gnu/libm.so.6","mode":"0644","slices":["libc6_libs"],"sha256":"3d448cb402aa7973d3a58f1fefbbe6ac856989702858c9b6ae4128193b0083bf","size":940560}
{"kind":"path","path":"/lib/x86_64-linux-gnu/libmemusage.so","mode":"0644","slices":["libc6_libs"],"sha256":"f983a04a2b2e8230b0bce38ea0965d171b88c5eb12b2a0a876b344337d078d62","size":18904}
{"kind":"path","path":"/lib/x86_64-linux-gnu/libmvec.so.1","mode":"0644","slices":["libc6_libs"],"sha256":"b090a959394fe51945659ef649a448f2f46dac5090453b62874f3fc97a9f0cc1","size":1031720}
{"kind":"path","path":"/lib/x86_64-linux-gnu/libnsl.so.1","mode":"0644","slices":["libc6_libs"],"sha256":"35dbf1a67b4f884f69eacd36596b383d8d4c42388315e57d93d098c1ea5dfb5a","size":101464}
{"kind":"path","path":"/lib/x86_64-linux-gnu/libnss_compat.so.2","mode":"0644","slices":["libc6_libs"],"sha256":"04f71867098cbf0a9fb2f75d53dd07c3f1325ae588302fb7917f5143c86595cc","size":44024}
{"kind":"path","path":"/lib/x86_64-linux-gnu/libnss_dns.so.2","mode":"0644","slices":["libc6_libs"],"sha256":"4df05c2d3c89b7848c8d74027810ed0a1b13aa0c12d7e767ebd82db16060b429","size":14352}
{"kind":"path","path":"/lib/x86_64-linux-gnu/libnss_files.so.2","mode":"0644","slices":["libc6_libs"],"sha256":"6ddfdf35421c722c8cac7476c44b50c90f0899a587c61a29ff9a2ea585f67e42","size":14352}
{"kind":"path","path":"/lib/x86_64-linux-gnu/libnss_hesiod.so.2","mode":"0644","slices":["libc6_libs"],"sha256":"88f2849b81c52194c89e88bbe49bcd5d516f431e2e42147a05409d7cd4e7b208","size":27160}
{"kind":"path","path":"/lib/x86_64-linux-gnu/libpcprofile.so","mode":"0644","slices":["libc6_libs"],"sha256":"bd442f5834e6bbf5db142205a0ed76c0cd508ee0a1a6eadb00237e1b5d8e5236","size":14616}
{"kind":"path","path":"/lib/x86_64-linux-gnu/libpthread.so.0","mode":"0644","slices":["libc6_libs"],"sha256":"fdfd7d83a335ba12bd73e857f95a798dae0355f37935231966fd088647978e23","size":21448}
{"kind":"path","path":"/lib/x86_64-linux-gnu/libresolv.so.2","mode":"0644","slices":["libc6_libs"],"sha256":"a8f03fb7400a9d34d8dc5bcc4d337004bc777462c91a9e8318ada0ce4f2876d1","size":68552}
{"kind":"path","path":"/lib/x86_64-linux-gnu/librt.so.1","mode":"0644","slices":["libc6_libs"],"sha256":"fa2b83203da2d25fb690140e8bc2da0c034532a045eec9fb8264849bad04f219","size":14664}
{"kind":"path","path":"/lib/x86_64-linux-gnu/libthread_db.so.1","mode":"0644","slices":["libc6_libs"],"sha256":"62b2cb55181ac47e05c27018b404e7ce79f437e4300d49b9c0db633fdd5467f9","size":39952}
{"kind":"path","path":"/lib/x86_64-linux-gnu/libutil.so.1","mode":"0644","slices":["libc6_libs"],"sha256":"684e765a68e8a7bacd7af7fdc7c45d31f4553361e511311f14a729b5623109d5","size":14432}
{"kind":"path","path":"/lib64/ld-linux-x86-64.so.2","mode":"0777","slices":["libc6_libs"],"link":"/lib/x86_64-linux-gnu/ld-linux-x86-64.so.2"}
{"kind":"path","path":"/run/","mode":"0755","slices":["base-files_var"]}
{"kind":"path","path":"/usr/bin/hello","mode":"0755","slices":["hello_bins"],"sha256":"1d636479116bca47dc34806a14f943f67b44a9bb948cf925f1814db6e316310f","size":26856}
{"kind":"path","path":"/usr/share/doc/base-files/copyright","mode":"0644","slices":["base-files_copyright"],"sha256":"cdb5461d8515002d0fe3babb764eec3877458b20f4e4bb16219f62ea953afeea","size":1228}
{"kind":"path","path":"/usr/share/doc/hello/copyright","mode":"0644","slices":["hello_copyright"],"sha256":"c3d6d02b6210ec90f78926b2da9509ad4372c22450599a0015f26ee05c07a9c6","size":2264}
{"kind":"path","path":"/usr/share/doc/libc6/copyright","mode":"0644","slices":["libc6_copyright"],"sha256":"d3c95b56fa33e28b57860580f0baf4e4f4de2a268a2b80f1d031a5191bade265","size":26462}
{"kind":"path","path":"/var/cache/","mode":"0755","slices":["base-files_var"]}
{"kind":"path","path":"/var/lib/","mode":"0755","slices":["base-files_var"]}
{"kind":"path","path":"/var/lib/chisel/manifest.wall","mode":"0644","slices":["base-files_chisel"]}
{"kind":"path","path":"/var/log/","mode":"0755","slices":["base-files_var"]}
{"kind":"path","path":"/var/run","mode":"0777","slices":["base-files_var"],"link":"/run"}
{"kind":"path","path":"/var/tmp/","mode":"01777","slices":["base-files_var"]}
{"kind":"slice","name":"base-files_chisel"}
{"kind":"slice","name":"base-files_copyright"}
{"kind":"slice","name":"base-files_var"}
{"kind":"slice","name":"hello_bins"}
{"kind":"slice","name":"hello_copyright"}
{"kind":"slice","name":"libc6_copyright"}
{"kind":"slice","name":"libc6_libs"}
```
