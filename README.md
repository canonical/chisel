[![Build](https://github.com/canonical/chisel/actions/workflows/build.yml/badge.svg)](https://github.com/canonical/chisel/actions/workflows/build.yml)
[![Tests](https://github.com/canonical/chisel/actions/workflows/tests.yaml/badge.svg)](https://github.com/canonical/chisel/actions/workflows/tests.yaml)

# Chisel

**Chisel** is a software tool for carving and cutting Debian packages!

It is built on the idea of **Package Slices** - minimal, complimentary and
loosely coupled sets of files, based on the package’s metadata and content.
Slices are basically subsets of the Debian packages, with their own content and
set of dependencies to other internal and external slices.

![pkg-slices](https://canonical-rockcraft.readthedocs-hosted.com/en/latest/_images/package-slices.png)

---
![a-slice-of-ubuntu](https://canonical-rockcraft.readthedocs-hosted.com/en/latest/_images/slice-of-ubuntu.png)

This image depicts a simple case, where both packages A and B are deconstructed
into multiple slices. At a package level, B depends on A, but in reality, there
might be files in A that B doesn’t actually need (eg. A_slice3 isn’t needed for
B to function properly). With this slice definition in place, Chisel is able to
extract a highly-customized and specialized Slice of the Ubuntu distribution,
which one could see as a block of stone from which we can carve and extract
small and relevant parts we need to run our applications. It is ideal to
support the creation of smaller but equally functional container images.

> “The sculpture is already complete within the marble block, before I start my
> work. It is already there, I just have to chisel away the superfluous
> material.”
>
> \- Michelangelo

In the end, it’s like having a slice of Ubuntu - get *just what you need*. You
*can have your cake and eat it too*!

## Using Chisel

To install the latest version of Chisel, run the following command:

```bash
go install github.com/canonical/chisel/cmd/chisel@latest
```

Chisel is invoked using `chisel <command>`. To get more information:

 - To see a help summary, type `chisel -h`.
 - To see a short description of all commands, type `chisel help --all`.
 - To see details for one command, type `chisel help <command>` or
`chisel <command> -h`.

### Example command

Chisel relies on a [database of slices](https://github.com/canonical/chisel-releases) that are indexed per Ubuntu release.

```bash
chisel cut --release ubuntu-22.04 --root myrootfs/ libgcc-s1_libs libssl3_libs
```

In this example, Chisel would look into the Ubuntu Jammy archives, fetch the
provided packages and install only the desired slices into the *myrootfs*
folder, according to the slice definitions available in the
["ubuntu-22.04" chisel-releases branch](<https://github.com/canonical/chisel-releases/tree/ubuntu-22.04>).

## Reference

### Chisel commands

#### help

Show basic help summary. Can also be used to display command details.

#### version

Displays the Chisel version.

#### cut

Uses the provided selection of package slices, for the given release, to create
a new filesystem tree in the root location.

### Chisel releases

As mentioned above, Chisel relies on **Package Slices**. These slices need to
be defined prior to the execution of the `chisel` command.

By default, Chisel will look into its central [chisel-releases](https://github.com/canonical/chisel-releases)
database, where package slices are defined and indexed by Ubuntu release.

One can, however, also point Chisel to a custom and local Chisel release (i.e.
`chisel cut --release ubuntu-22.10 ...` will fetch the package slices from
the upstream central database of slices, whilst
`chisel cut --release release/ ...` will make Chisel fetch the package slices
from the local path "./release/").

#### Chisel release configuration

Each Chisel release must have one "chisel.yaml" file.

*chisel.yaml*:

```yaml
format: <chiselReleaseFormat>

archives:
    ubuntu:
        # Ubuntu archive for Chisel to look into
        version: <ubuntuRelease>

        # categories/components of the Ubuntu archive to look into
        components: [<componentName>, ...]

        # pockets/suites of the Ubuntu archive to look into
        suites: [<pocket>, ...]
```

Example:

```yaml
format: chisel-v1

archives:
    ubuntu:
        version: 22.04
        components: [main, universe]
        suites: [jammy, jammy-security, jammy-updates]
```

#### Slice definitions

There can be only **one slice definitions file** for each Ubuntu package, per
Chisel release. All of the slice definitions files must be placed under a
"slices" folder, and follow the same structure:

*slices/\<pkgName\>.yaml*:

```yaml
# (req) Name of the package.
# The slice definition file should be named accordingly (eg. "openssl.yaml")

package: <package-name>

# (req) List of slices
slices:

    # (req) Name of the slice
    <slice-name>:

        # (opt) Optional list of slices that this slice depends on
        essential:
          - <pkgA_slice-name>
          - ...

        # (req) The list of files, from the package, that this slice will install
        contents:
            </path/to/content>:
            </path/to/another/multiple*/content/**>:
            </path/to/moved/content>: {copy: /bin/original}
            </path/to/link>: {symlink: /bin/mybin}
            </path/to/new/dir>: {make: true}
            </path/to/file/with/text>: {text: "Some text"}
            </path/to/mutable/file/with/default/text>: {text: FIXME, mutable: true}
            </path/to/temporary/content>: {until: mutate}

        # (opt) Mutation scripts, to allow for the reproduction of maintainer scripts,
        # based on Starlark (https://github.com/canonical/starlark)
        mutate: |
            ...
```

Example:

```yaml
package: mypkg

slices:
    bins:
        essential:
            - mypkg_config

        contents:
            /bin/mybin:
            /bin/moved:  {copy: /bin/original}
            /bin/linked: {symlink: /bin/mybin}

    config:
        contents:
            /etc/mypkg.conf: {text: "The configuration."}
            /etc/mypkg.d/:   {make: true}
```

To find more examples of real slice definitios files (and contribute your own),
please go to <https://github.com/canonical/chisel-releases>.

## TODO

- [ ] Preserve ownerships when possible
- [ ] GPG signature checking for archives
- [ ] Use a fake server for the archive tests
- [ ] Functional tests


## FAQ

#### May I use arbitrary package names?

No, package names must reflect the package names in the archive,
so that there's a single namespace to remember and respect.

#### I've tried to use a different Ubuntu version and it failed?

The mapping is manual for now. Let us know and we'll fix it.

#### Can I use multiple repositories in a Chisel release?

Not at the moment, but maybe eventually.

#### Can I use non-Ubuntu repositories?

Not at the moment, but eventually.

#### Can multiple slices refer to the same path?

Yes, but see below.

#### Can multiple slices _output_ the same path?

Yes, as long as either both slices are part of the same package,
or the path is not extracted from a package at all (not copied)
and the explicit inline definitions match exactly.

#### Is file ownership preserved?

Not right now, but it will be supported.
