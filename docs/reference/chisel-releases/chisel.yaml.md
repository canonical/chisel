(chisel_yaml_ref)=
# chisel.yaml

The `chisel.yaml` file defines various configuration values for Chisel,
for a given _chisel-release_.


(chisel_yaml_location)=
## Location

The file must be placed in the root level of a {ref}`chisel-releases_ref`
directory.


(chisel_yaml_format_spec)=
## Format specification

(chisel_yaml_format_spec_format)=
### `format`

| Field    | Type  | Required | Supported values |
| -------- | ----- | -------- | ---------------- |
| `format` | `str` | Required | `v1`             |

Used to define the supported schemas for the {ref}`chisel-releases_ref`.
For example:

```yaml
format: v1
```

```{note}
New formats are typically introduced with new _chisel-releases_ and may
introduce disruptive changes to the previous formats.
```


(chisel_yaml_format_spec_archives)=
### `archives`

| Field      | Type              | Required |
| ---------- | ----------------- | -------- |
| `archives` | `dict(str, dict)` | Required |

Tells Chisel which Ubuntu archives to fetch packages from.

```{note}
Chisel only supports fetching packages from the official Ubuntu archives,
including ESM.
```

For example:

```yaml
archives:
  ubuntu-esm-apps:
    pro: esm-apps
    priority: 16
    version: 24.04
    components: [main]
    suites: [noble-apps-security, noble-apps-updates]
    public-keys: [ubuntu-apps-key]
```

If {ref}`chisel_yaml_format_spec_archives_pro` is not specified, the archives
point to:

- http://archive.ubuntu.com/ for `amd64` and `i386` package architecture, and
- http://ports.ubuntu.com/ubuntu-ports/ for other architectures.

otherwise, the archive point to the Ubuntu Pro archives listed
{ref}`below<chisel_yaml_format_spec_archives_pro>`.


(chisel_yaml_format_spec_archives_default)=
### `archives.<name>.default`

| Field     | Type   | Required                                                                                            | Supported values |
| --------- | ------ | --------------------------------------------------------------------------------------------------- | ---------------- |
| `default` | `bool` | Required with multiple archives, if no {ref}`priorities<chisel_yaml_format_spec_archives_priority>` | `true`, `false`  |

If `default` is `true`, Chisel fetches packages from this archive, unless
otherwise specified by the field {ref}`"archive"<slice_definitions_format_archive>`
in the slice definitions file.

In case there are multiple archives, one, **and only one**, must be the default,
**otherwise**, use {ref}`priorities<chisel_yaml_format_spec_archives_priority>`.

```{tip}
It is preferred to use {ref}`chisel_yaml_format_spec_archives_priority` instead
of {ref}`chisel_yaml_format_spec_archives_default`.
```


(chisel_yaml_format_spec_archives_version)=
### `archives.<name>.version`

| Field     | Type  | Required | Supported values                                        |
| --------- | ----- | -------- | ------------------------------------------------------- |
| `version` | `str` | Required | Ubuntu release in `xx.yy` format e.g. 22.04, 24.04 etc. |

Indicates the Ubuntu release this archive should fetch the
packages for. This value is currently only used for logging, and does not change
the archive behaviour.


(chisel_yaml_format_spec_archives_suites)=
### `archives.<name>.suites`

| Field    | Type        | Required | Supported values                                              |
| -------- | ----------- | -------- | ------------------------------------------------------------- |
| `suites` | `list(str)` | Required | Ubuntu archive suite names e.g. `jammy`, `noble-updates` etc. |

Lists the archive suites to fetch packages from. Read more
about suites in the [Ubuntu packaging
guide](https://canonical-ubuntu-packaging-guide.readthedocs-hosted.com/en/latest/explanation/archive/#suite).


(chisel_yaml_format_spec_archives_components)=
### `archives.<name>.components`

| Field        | Type        | Required | Supported values                                   |
| ------------ | ----------- | -------- | -------------------------------------------------- |
| `components` | `list(str)` | Required | Suite component names e.g. `main`, `universe` etc. |

Lists the components of the archive suites to fetch
packages from. Read more about components in the [Ubuntu packaging
guide](https://canonical-ubuntu-packaging-guide.readthedocs-hosted.com/en/latest/explanation/archive/#components).

Chisel reads the `InRelease` files from each `(suite, component)` combination to
locate packages.


(chisel_yaml_format_spec_archives_public_keys)=
### `archives.<name>.public-keys`

| Field         | Type        | Required | Supported values                                                            |
| ------------- | ----------- | -------- | --------------------------------------------------------------------------- |
| `public-keys` | `list(str)` | Required | List of key names, as defined in {ref}`chisel_yaml_format_spec_public_keys` |

Lists the names of the OpenPGP public keys needed to verify the archive's `InRelease`
file signatures. These key names must be defined in
{ref}`chisel_yaml_format_spec_public_keys`.


(chisel_yaml_format_spec_archives_priority)=
### `archives.<name>.priority`

| Field      | Type  | Required                                         | Supported values                   |
| ---------- | ----- | ------------------------------------------------ | ---------------------------------- |
| `priority` | `int` | Required with multiple archives, if no `default` | Any integer between -1000 and 1000 |

Describes the priority of an archive compared to other
archives. It is used to support multiple archives in Chisel. If a package is
available in two archives, it is fetched from the archive with higher priority,
unless:
 - the package's slice definitions file specifies {ref}`"archive"<slice_definitions_format_archive>`, or
 - the {ref}`chisel_yaml_format_spec_archives_default` field is `true` for any archive.
In this case, the priority values of **all** archives are ignored.

Note that:
- an unspecified `priority` field **does not** yield a 0 value, and
- two archives cannot have the same `priority` value.


(chisel_yaml_format_spec_archives_pro)=
### `archives.<name>.pro`

| Field | Type  | Required | Supported values                                 |
| ----- | ----- | -------- | ------------------------------------------------ |
| `pro` | `str` | Optional | `fips`, `fips-updates`, `esm-apps`, `esm-infra`. |

Specifies the [Ubuntu Pro](https://ubuntu.com/pro) archive to
fetch and install packages from.

```{important}
To chisel a Pro package you need to have a Pro-enabled host.
```

Chisel reads the Pro archives' credentials from the directory defined by the
environment variable `CHISEL_AUTH_DIR` (which defaults to `/etc/apt/auth.conf.d`).

The following `pro` values are supported, and if specified, the
archive points to their corresponding base URLs.

| `pro` value    | Base repository URL                        |
| -------------- | ------------------------------------------ |
| `fips`         | https://esm.ubuntu.com/fips/ubuntu         |
| `fips-updates` | https://esm.ubuntu.com/fips-updates/ubuntu |
| `esm-apps`     | https://esm.ubuntu.com/apps/ubuntu         |
| `esm-infra`    | https://esm.ubuntu.com/infra/ubuntu        |

```{tip}
Although not enforced, the following `priority` values are suggested when
`pro` is used:

| `pro` value                                                                        | Suggested `priority` |
| ---------------------------------------------------------------------------------- | -------------------- |
| `fips`                                                                             | 20                   |
| `fips-updates`                                                                     | 21                   |
| `esm-apps`                                                                         | 16                   |
| `esm-infra`                                                                        | 15                   |
| `""` (empty, indicates a {ref}`non-Pro archive<chisel_yaml_format_spec_archives>`) | 10                   |
```

(chisel_yaml_format_spec_public_keys)=
### `public-keys`

| Field         | Type              | Required |
| ------------- | ----------------- | -------- |
| `public-keys` | `dict(str, dict)` | Required |

The top-level `public-keys` field is used to define OpenPGP public keys that are
needed to verify the `InRelease` file signatures of the 
{ref}`chisel_yaml_format_spec_archives`.

For example:

```yaml
public-keys:
  ubuntu-archive-key-2018:
    id: 871920D1991BC93C
    armor: |  # Armored ASCII data
      -----BEGIN PGP PUBLIC KEY BLOCK-----

      mQINBFufwdoBEADv/Gxytx/LcSXYuM0MwKojbBye81s0G1nEx+lz6VAUpIUZnbkq
      ...
      -----END PGP PUBLIC KEY BLOCK-----
```

The key names are then referenced in
{ref}`chisel_yaml_format_spec_archives_public_keys` as needed.


(chisel_yaml_format_spec_public_keys_id)=
### `public-keys.<name>.id`

| Field | Type  | Required |
| ----- | ----- | -------- |
| `id`  | `str` | Required |

The `id` field specifies the OpenPGP public key fingerprint in capital hex e.g.
`871920D1991BC93C`. It must be 16 chars long and must match the decoded
fingerprint in {ref}`chisel_yaml_format_spec_public_keys_armor`.


(chisel_yaml_format_spec_public_keys_armor)=
### `public-keys.<name>.armor`

| Field   | Type  | Required |
| ------- | ----- | -------- |
| `armor` | `str` | Required |

The `armor` field contains the multi-line armored ASCII data of OpenPGP public
key.


(chisel_yaml_example)=
## Example

The following `chisel.yaml` is used in Ubuntu 24.04 (Noble) release:

```yaml
format: v1

archives:
  ubuntu:
    default: true
    version: 24.04
    components: [main, universe]
    suites: [noble, noble-security, noble-updates]
    public-keys: [ubuntu-archive-key-2018]

public-keys:
  # Ubuntu Archive Automatic Signing Key (2018) <ftpmaster@ubuntu.com>
  # rsa4096/f6ecb3762474eda9d21b7022871920d1991bc93c 2018-09-17T15:01:46Z
  ubuntu-archive-key-2018:
    id: "871920D1991BC93C"
    armor: |
      -----BEGIN PGP PUBLIC KEY BLOCK-----

      mQINBFufwdoBEADv/Gxytx/LcSXYuM0MwKojbBye81s0G1nEx+lz6VAUpIUZnbkq
      dXBHC+dwrGS/CeeLuAjPRLU8AoxE/jjvZVp8xFGEWHYdklqXGZ/gJfP5d3fIUBtZ
      HZEJl8B8m9pMHf/AQQdsC+YzizSG5t5Mhnotw044LXtdEEkx2t6Jz0OGrh+5Ioxq
      X7pZiq6Cv19BohaUioKMdp7ES6RYfN7ol6HSLFlrMXtVfh/ijpN9j3ZhVGVeRC8k
      KHQsJ5PkIbmvxBiUh7SJmfZUx0IQhNMaDHXfdZAGNtnhzzNReb1FqNLSVkrS/Pns
      AQzMhG1BDm2VOSF64jebKXffFqM5LXRQTeqTLsjUbbrqR6s/GCO8UF7jfUj6I7ta
      LygmsHO/JD4jpKRC0gbpUBfaiJyLvuepx3kWoqL3sN0LhlMI80+fA7GTvoOx4tpq
      VlzlE6TajYu+jfW3QpOFS5ewEMdL26hzxsZg/geZvTbArcP+OsJKRmhv4kNo6Ayd
      yHQ/3ZV/f3X9mT3/SPLbJaumkgp3Yzd6t5PeBu+ZQk/mN5WNNuaihNEV7llb1Zhv
      Y0Fxu9BVd/BNl0rzuxp3rIinB2TX2SCg7wE5xXkwXuQ/2eTDE0v0HlGntkuZjGow
      DZkxHZQSxZVOzdZCRVaX/WEFLpKa2AQpw5RJrQ4oZ/OfifXyJzP27o03wQARAQAB
      tEJVYnVudHUgQXJjaGl2ZSBBdXRvbWF0aWMgU2lnbmluZyBLZXkgKDIwMTgpIDxm
      dHBtYXN0ZXJAdWJ1bnR1LmNvbT6JAjgEEwEKACIFAlufwdoCGwMGCwkIBwMCBhUI
      AgkKCwQWAgMBAh4BAheAAAoJEIcZINGZG8k8LHMQAKS2cnxz/5WaoCOWArf5g6UH
      beOCgc5DBm0hCuFDZWWv427aGei3CPuLw0DGLCXZdyc5dqE8mvjMlOmmAKKlj1uG
      g3TYCbQWjWPeMnBPZbkFgkZoXJ7/6CB7bWRht1sHzpt1LTZ+SYDwOwJ68QRp7DRa
      Zl9Y6QiUbeuhq2DUcTofVbBxbhrckN4ZteLvm+/nG9m/ciopc66LwRdkxqfJ32Cy
      q+1TS5VaIJDG7DWziG+Kbu6qCDM4QNlg3LH7p14CrRxAbc4lvohRgsV4eQqsIcdF
      kuVY5HPPj2K8TqpY6STe8Gh0aprG1RV8ZKay3KSMpnyV1fAKn4fM9byiLzQAovC0
      LZ9MMMsrAS/45AvC3IEKSShjLFn1X1dRCiO6/7jmZEoZtAp53hkf8SMBsi78hVNr
      BumZwfIdBA1v22+LY4xQK8q4XCoRcA9G+pvzU9YVW7cRnDZZGl0uwOw7z9PkQBF5
      KFKjWDz4fCk+K6+YtGpovGKekGBb8I7EA6UpvPgqA/QdI0t1IBP0N06RQcs1fUaA
      QEtz6DGy5zkRhR4pGSZn+dFET7PdAjEK84y7BdY4t+U1jcSIvBj0F2B7LwRL7xGp
      SpIKi/ekAXLs117bvFHaCvmUYN7JVp1GMmVFxhIdx6CFm3fxG8QjNb5tere/YqK+
      uOgcXny1UlwtCUzlrSaP
      =9AdM
      -----END PGP PUBLIC KEY BLOCK-----
```
