# How to install Chisel

To install the latest version of Chisel, you can choose any of the following
methods:

- {ref}`install_chisel_binary`
- {ref}`install_chisel_source`
- {ref}`install_chisel_snap`


(install_chisel_binary)=
## Install the binary

We publish pre-built binaries for every version of Chisel on GitHub. To install
the latest Chisel binary:

<!-- Start: Install Chisel binary -->

1. Visit the {{latest_release_page}} to determine the latest tag, for example,
   `v1.0.0`.

2. Run the following command to download the file. Make sure to replace
   `v1.0.0` with the latest tag and `amd64` with your architecture.
   ```sh
   wget https://github.com/canonical/chisel/releases/download/v1.0.0/chisel_v1.0.0_linux_amd64.tar.gz
   ```

3. We publish checksum files for the release tarballs. Download the appropriate
   checksum file with the following command.
   ```sh
   wget https://github.com/canonical/chisel/releases/download/v1.0.0/chisel_v1.0.0_linux_amd64.tar.gz.sha384
   ```

4. Verify the checksum with the following command:
   ```sh
   sha384sum -c chisel_v1.0.0_linux_amd64.tar.gz.sha384
   ```

5. Extract the contents of the downloaded tarball by running:
   ```sh
   tar zxvf chisel_v1.0.0_linux_amd64.tar.gz
   ```

6. Install the Chisel binary. Make sure the installation directory is included
   in your system’s `PATH` environment variable.
   ```sh
   sudo mv chisel /usr/local/bin/
   ```
<!-- End: Install Chisel binary -->

(install_chisel_source)=
## Install from source

Alternatively, you can install the latest version of Chisel from source:

1. Follow the [official Go documentation](https://go.dev/doc/install) to
   download and install Go.

2. After installing, you will want to add the `$GOBIN` directory to your
   `$PATH` so you can use the installed tools. For more information, refer to
   the [official documentation](https://go.dev/doc/install/source#environment).

3. Run the following command to build and install the latest version of Chisel:
   ```sh
   go install github.com/canonical/chisel/cmd/chisel@latest
   ```


(install_chisel_snap)=
## Install Snap

You can also install the latest version of Chisel from the Snap store. Run the
following command to install from the `latest/stable` track:

```sh
sudo snap install chisel
```

```{note}
This snap can only install the slices in a location inside the user `$HOME`
directory i.e. the `--root` option in the `cut` command should have a location
inside the user `$HOME` directory.
```


## Verify installation

<!-- Start: Verify Chisel installation -->

Once the installation is complete, verify that Chisel has been installed
correctly by running:

```sh
chisel
```

This should produce output similar to the following:

```{terminal}
:input: chisel

Chisel can slice a Linux distribution using a release database
and construct a new filesystem using the finely defined parts.

Usage: chisel <command> [<options>...]

...
```

<!-- End: Verify Chisel installation -->
