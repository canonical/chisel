# jsonwall

The "jsonwall" format is a custom database file format where there is one JSON
object per line in the file, and both the individual JSON fields and the lines
themselves (except the {ref}`header<jsonwall_header>`) are sorted to optimize
for searching and iteration.

For example, the following content represents a valid jsonwall database:

```json
{"jsonwall":"1.0","count":3}
{"kind":"app","name":"chisel","version":"1.0"}
{"kind":"app","name":"pebble","version":"1.2"}
```


(jsonwall_contents)=
## Contents of a jsonwall file

A jsonwall file consists of a header and the actual data.


(jsonwall_header)=
### Header

A jsonwall file starts with a header JSON object on the first line. The header
has the following attributes:

| Field | Type | Required | Description |
| - | - | - | - |
| `jsonwall` | `str` | required | the version of jsonwall. |
| `schema` | `str` | optional | the schema version that the particular application uses, if it does. |
| `count` | `int` | required | the number of JSON entries (or, lines) in this file, including the header itself. |

```{important}
The jsonwall header line always stays on top of the file. The following lines
are sorted in a lexicographic order.
```

(jsonwall_data)=
### Data

The following lines after the header consists of actual data. These must be JSON
objects, defined one per line. These lines are sorted in a lexicographic order
and the fields within these objects are also sorted in a lexicographic order.


(jsonwall_usage)=
## Usage

[Chisel manifest](/reference/manifest) uses the jsonwall format to list the
metadata about installed files, slices and packages.


(jsonwall_source)=
## Source

The jsonwall format is introduced by the internal [jsonwall
package](https://github.com/canonical/chisel/blob/main/internal/jsonwall/jsonwall.go).
