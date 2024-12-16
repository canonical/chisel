# find command

The **find** command queries {{chisel_releases_repo}} for matching slices.

Globs (`*` and `?`) are allowed in the query.
 
By default it fetches the slices for the same Ubuntu version as the
current host, unless the `--release` option is used.

## Options

- `--release` is a {{chisel_releases_repo}} branch or local directory (e.g. ubuntu-22.04).

## Example

Run the following command to search python3.10 slices in `ubuntu-22.04` release:

```{terminal}
:input: chisel find --release ubuntu-22.04 python3.10

2024/11/26 13:11:08 Consulting release repository...
2024/11/26 13:11:10 Fetching current ubuntu-22.04 release...
2024/11/26 13:11:10 Processing ubuntu-22.04 release...
Slice                 Summary
python3.10_copyright  -
python3.10_core       -
python3.10_standard   -
python3.10_utils      -
python3.11_copyright  -
python3.11_core       -
python3.11_standard   -
python3.11_utils      -
```

```{note}
Notice that there are some python3.11 slices in the output as well. This is
because the find command finds partially-matched slices with
{{Levenshtein_distance}} of up to 1.
```

The first three lines are logs, which you can ignore with:

```sh
chisel find --release ubuntu-22.04 python3.10 2>logs
```
