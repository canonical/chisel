## Example command

```
$ chisel cut --release release/ --root output/ mypkg.bins mypkg.config
```

## Example release configuration

**release/chisel.yaml**
```yaml
format: chisel-v1

archives:
    ubuntu:
        version: 22.04
        components: [main, universe]
```

**release/slices/mypkg.yaml**
```yaml
package: mypkg

slices:
    bins:
        essential:
            - mypkg.config

        contents:
            /bin/mybin:
            /bin/moved:  {copy: /bin/original}
            /bin/linked: {symlink: /bin/mybin}

    config:
        contents:
            /etc/mypkg.conf: {text: "The configuration."}
            /etc/mypkg.d/:   {make: true}
```

## TODO

- [ ] Globbing support with no-conflict enforcement
- [ ] Preserve ownerships when possible
- [ ] GPG signature checking for archives
- [ ] Use a fake server for the archive tests
- [ ] More functional tests


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
