# Getting Started with Chisel

In this tutorial, we will download and install Chisel, install slices to create
a minimal root file system containing an application and execute that
application from the file system. At the end of the tutorial, we should have a
file system which is minimal and yet contains all the necessary files to run our
application.

After this tutorial, you will have a basic understanding of what Chisel is and
how to use it to install {{package_slices}}, and you can continue exploring more
advance features and use cases (see {ref}`tutorial_next_steps`).

## Prerequisites

- A Linux machine.

## Download and install Chisel

The easiest way to install the latest Chisel release is by downloading the
binary. If you prefer a different installation method, see
[](/how-to/install-chisel). To install the latest Chisel binary:

```{include} /how-to/install-chisel.md
  :start-after: "<!-- Start: Install Chisel binary -->"
  :end-before:  "<!-- End: Install Chisel binary -->"
```

### Verify the Chisel installation

```{include} /how-to/install-chisel.md
  :start-after: "<!-- Start: Verify Chisel installation -->"
  :end-before:  "<!-- End: Verify Chisel installation -->"
```

## Install a slice

In this tutorial, we will install the {{hello_pkg}} application in our root file
system. We will use the Ubuntu 24.04 (noble) package as the source of the
necessary files.

Chisel uses slice definition files to define and install slices. These files are
stored in the {{chisel_releases_repo}}. Upon a quick look there, you can find
that there is a `hello_bins` slice that contains the `hello` binary and looks
like the following:

{{hello_bins}}

```{tip}
We can also search for slices with the {{find_cmd}} easily.
```

Now, create an empty directory where the root file system should be located:
```
mkdir rootfs
```

Finally, use the {{cut_cmd}} to install the `hello_bins` slice in the root file
system:

```
chisel cut --release ubuntu-24.04 --root rootfs/ hello_bins
```

````{note}

```{include} /reference/cmd/cut.md
  :start-after: "<!-- Start: cut command options -->"
  :end-before:  "<!-- End: cut command options -->"
```

Learn more in the {{cut_cmd}} reference page.
````

Successful execution of the above command should look like the following:

```{include} /reference/cmd/cut.md
  :start-after: "<!-- Start: hello_bins installation -->"
  :end-before:  "<!-- End: hello_bins installation -->"
```

### Check root file system

The `hello_bins` slice has been successfully installed. But what do we have
exactly in our root file system? The following illustrates:

```{terminal}
:input: cd rootfs/ && find .

.
./lib64
./usr
./usr/lib64
./usr/lib64/ld-linux-x86-64.so.2
./usr/share
./usr/share/doc
./usr/share/doc/base-files
./usr/share/doc/base-files/copyright
./usr/share/doc/hello
./usr/share/doc/hello/copyright
./usr/share/doc/libc6
./usr/share/doc/libc6/copyright
./usr/bin
./usr/bin/hello
./usr/lib
./usr/lib/x86_64-linux-gnu
./usr/lib/x86_64-linux-gnu/libpthread.so.0
./usr/lib/x86_64-linux-gnu/libthread_db.so.1
./usr/lib/x86_64-linux-gnu/libm.so.6
./usr/lib/x86_64-linux-gnu/libc.so.6
./usr/lib/x86_64-linux-gnu/libmemusage.so
./usr/lib/x86_64-linux-gnu/libdl.so.2
./usr/lib/x86_64-linux-gnu/librt.so.1
./usr/lib/x86_64-linux-gnu/libnss_files.so.2
./usr/lib/x86_64-linux-gnu/libanl.so.1
./usr/lib/x86_64-linux-gnu/libnss_hesiod.so.2
./usr/lib/x86_64-linux-gnu/libc_malloc_debug.so.0
./usr/lib/x86_64-linux-gnu/libmvec.so.1
./usr/lib/x86_64-linux-gnu/libutil.so.1
./usr/lib/x86_64-linux-gnu/libresolv.so.2
./usr/lib/x86_64-linux-gnu/ld-linux-x86-64.so.2
./usr/lib/x86_64-linux-gnu/libBrokenLocale.so.1
./usr/lib/x86_64-linux-gnu/libpcprofile.so
./usr/lib/x86_64-linux-gnu/libnss_dns.so.2
./usr/lib/x86_64-linux-gnu/libnsl.so.1
./usr/lib/x86_64-linux-gnu/libnss_compat.so.2
./lib
```

Notice that the `hello` binary has been installed at `./usr/bin/hello`.

```{note}
Notice how we have a few other files than the `hello` binary. This is because
the `hello_bins` slice depends on a few other slices such as `libc6_libs` which
provides libraries. When installing a slice, Chisel installs its dependencies as
well.
```

## Run application from root file system

To run the `hello` application from the root file system, do the following:

```{terminal}
:input: sudo chroot rootfs/ hello

Hello, world!
```

(tutorial_next_steps)=
## Next steps

- To learn more about the Chisel subcommands e.g. `cut`, `find`, see
  [](/reference/cmd/index).
- To learn more about what slices are, see [](/explanation/slices).
