# find command

The **find** command queries {{chisel_releases}} for matching slices.

## Usage

```{terminal}
:scroll:
:input: chisel find --help

Usage:
  chisel find [find-OPTIONS] [<query>...]

The find command queries the slice definitions for matching slices.
Globs (* and ?) are allowed in the query.

By default it fetches the slices for the same Ubuntu version as the
current host, unless the --release flag is used.

[find command options]
      --release=<branch|dir>      Chisel release name or directory (e.g. ubuntu-22.04)
```

### Options

The find command takes in the following options:

- `--release` is used to specify the {{chisel_releases}} branch to search slices
  in or the local release directory path.

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

(how_find_cmd_works)=
## How the find command works

The command takes in a list of query terms. For each term, it finds the matching
slices. However, it only outputs the slices which matches each query. For
example, if you run `chisel find query1 query2`, Chisel will list only the
slices who matches with `query1` and `query2`.

Additionally, for matching slices against a query term, the `find` command has
a conditional logic.

1. If the term does not have any underscore(`_`), it will match against package
   names only. As shown above, `chisel find foo` will search for packages whose
   name matches with `foo` and list all of their slices.

(how_find_works_pt_2)=
2. If the term starts with an underscore(`_`), then Chisel will list all the
   slices whose slice name matches with the rest of the query. Note here that,
   for a slice `python3.12_core`, `python3.12` is the package name and `core` is
   the slice name (short name). Thus, if you were to run `chisel find _foo`, you
   will get a list of slices whose slice names match with `foo`. Examples below
   in latter sections.

3. If the term does not start with an underscore(`_`) but contains one, then
   only those slices are listed whose full slice name (`pkg_slice`) match with
   the query term.

## Advanced usage

This section describes a few advanced use cases of the find command.

### Find slices across packages

To search how many `bins` slices are there, run the following:

```{terminal}
:input: chisel find _bins 2> logs

Slice                    Summary
base-files_bin           -
bash_bins                -
busybox_bins             -
coreutils_bins           -
curl_bins                -
dash_bins                -
dotnet-host_bins         -
findutils_bins           -
...
```

This illustrates the behaviour mentioned in the second point in
{ref}`how_find_cmd_works`.

### Wildcards!

The find command supports wildcards (`*`, `?`).

- `*` matches any zero or more character including `_`.
- `?` matches zero or one character including `_`.

The following searches for all the `dotnet` slices:

```{terminal}
:input: chisel find dotnet* 2> logs

Slice                         Summary
dotnet-host_bins              -
dotnet-host_copyright         -
dotnet-hostfxr-6.0_copyright  -
dotnet-hostfxr-6.0_libs       -
dotnet-runtime-6.0_copyright  -
dotnet-runtime-6.0_libs       -
```

The following only searches for the `bins` and `libs` slices from above:

```{terminal}
:input: chisel find dotnet* _?i?s 2> logs

Slice                    Summary
dotnet-host_bins         -
dotnet-hostfxr-6.0_libs  -
dotnet-runtime-6.0_libs  -
```

```{note}
Notice that there are two query terms used above.
```
