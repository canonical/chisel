# Getting started with Chisel

This tutorial will walk you through the creation of your first
chiseled Ubuntu root filesystem, from installation to the slicing of
Ubuntu packages. 

The goal is to give you a basic understanding of what Chisel is and
how to use it to install {{package_slices}}. By the end of it,
you will have a minimal slice of Ubuntu comprising only the necessary
files to run our application.

## Prerequisites

- A Linux machine.

## Download and install Chisel

Let's install the Chisel from its GitHub releases page. If you prefer a
different installation method, see [](/how-to/install-chisel).

To install the latest Chisel binary:

```{include} /how-to/install-chisel.md
  :start-after: "<!-- Start: Install Chisel binary -->"
  :end-before:  "<!-- End: Install Chisel binary -->"
```

### Verify the Chisel installation

```{include} /how-to/install-chisel.md
  :start-after: "<!-- Start: Verify Chisel installation -->"
  :end-before:  "<!-- End: Verify Chisel installation -->"
```

## Slice the "{{hello_pkg}}" package

The {{hello_pkg}} package has already been sliced and its slice definitions are
[available in the chisel-releases
repository](https://github.com/canonical/chisel-releases/blob/ubuntu-24.04/slices/hello.yaml). 

Before running Chisel, creating the empty directory where the root file system should be
located:
```
mkdir rootfs
```

Finally, use the {{cut_cmd}} to install the `hello_bins` slice from the `ubuntu-24.04`
_chisel-release_:

```{include} /reference/cmd/cut.md
  :start-after: "<!-- Start: hello_bins installation -->"
  :end-before:  "<!-- End: hello_bins installation -->"
```

### Inspect the chiseled root file system

A quick look at the `rootfs/` directory will show that the {{hello_pkg}} binary
has been installed at `rootfs/usr/bin/hello`.

```{terminal}
:input: find rootfs

rootfs
rootfs/lib64
rootfs/usr
rootfs/usr/lib64
rootfs/usr/lib64/ld-linux-x86-64.so.2
rootfs/usr/share
rootfs/usr/share/doc
rootfs/usr/share/doc/base-files
rootfs/usr/share/doc/base-files/copyright
rootfs/usr/share/doc/hello
rootfs/usr/share/doc/hello/copyright
rootfs/usr/share/doc/libc6
rootfs/usr/share/doc/libc6/copyright
rootfs/usr/bin
rootfs/usr/bin/hello
rootfs/usr/lib
rootfs/usr/lib/x86_64-linux-gnu
rootfs/usr/lib/x86_64-linux-gnu/libpthread.so.0
rootfs/usr/lib/x86_64-linux-gnu/libthread_db.so.1
rootfs/usr/lib/x86_64-linux-gnu/libm.so.6
rootfs/usr/lib/x86_64-linux-gnu/libc.so.6
rootfs/usr/lib/x86_64-linux-gnu/libmemusage.so
rootfs/usr/lib/x86_64-linux-gnu/libdl.so.2
rootfs/usr/lib/x86_64-linux-gnu/librt.so.1
rootfs/usr/lib/x86_64-linux-gnu/libnss_files.so.2
rootfs/usr/lib/x86_64-linux-gnu/libanl.so.1
rootfs/usr/lib/x86_64-linux-gnu/libnss_hesiod.so.2
rootfs/usr/lib/x86_64-linux-gnu/libc_malloc_debug.so.0
rootfs/usr/lib/x86_64-linux-gnu/libmvec.so.1
rootfs/usr/lib/x86_64-linux-gnu/libutil.so.1
rootfs/usr/lib/x86_64-linux-gnu/libresolv.so.2
rootfs/usr/lib/x86_64-linux-gnu/ld-linux-x86-64.so.2
rootfs/usr/lib/x86_64-linux-gnu/libBrokenLocale.so.1
rootfs/usr/lib/x86_64-linux-gnu/libpcprofile.so
rootfs/usr/lib/x86_64-linux-gnu/libnss_dns.so.2
rootfs/usr/lib/x86_64-linux-gnu/libnsl.so.1
rootfs/usr/lib/x86_64-linux-gnu/libnss_compat.so.2
rootfs/lib
```

````{note}
Notice, however, that there are a few other files besides the `hello` binary.
This is because the `hello_bins` slice depends on other slices, such as `libc6_libs`,
which provides necessary runtime libraries:

```{terminal}
:input: chisel info --release ubuntu-24.04 hello_bins 2>/dev/null
package: hello
archive: ubuntu
slices:
    bins:
        essential:
            - hello_copyright
            - libc6_libs
        contents:
            /usr/bin/hello: {}
```

When installing a slice, Chisel installs its dependencies as well.
````

## Test the application

To run `hello` from the chiseled root file system, do the following:

```{terminal}
:input: sudo chroot rootfs/ hello

Hello, world!
```

## Next steps


See what other [](/reference/cmd/index) are there, and have a look at
the {ref}`how_to_guides` for learning about other typical
Chisel operations. 
