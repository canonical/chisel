# info command

The **info** command shows detailed information about package slices.

It complements the {{find_cmd}}. Whereas the `find` command searches for existing
slices, the `info` command takes in slice or package name(s) and displays the
slice definition(s).

It accepts a whitespace-separated list of strings. The list can be
composed of package names, slice names, or a combination of both. The
default output format is YAML. When multiple arguments are provided,
the output is a list of YAML documents separated by a `---` line.

Slice definitions are shown according to their definition in
the selected release. For example, globs are not expanded.


## Options

- `--release` is a {{chisel_releases_repo}} branch or local directory (e.g. ubuntu-22.04).

## Example

The following illustrates using the `info` command to inspect multiple slices:

```{terminal}
:input: chisel info hello libgcc-s1_libs 2>logs

package: hello
archive: ubuntu
slices:
    bins:
        essential:
            - hello_copyright
            - libc6_libs
        contents:
            /usr/bin/hello: {}
    copyright:
        contents:
            /usr/share/doc/hello/copyright: {}
---
package: libgcc-s1
archive: ubuntu
slices:
    libs:
        essential:
            - libgcc-s1_copyright
            - libc6_libs
        contents:
            /usr/lib/*-linux-*/libgcc_s.so.*: {}
```
