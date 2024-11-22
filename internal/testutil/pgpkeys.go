package testutil

import (
	"log"

	"golang.org/x/crypto/openpgp/packet"

	"github.com/canonical/chisel/internal/pgputil"
)

type PGPKeyData struct {
	ID           string
	PubKeyArmor  string
	PrivKeyArmor string
	PubKey       *packet.PublicKey
	PrivKey      *packet.PrivateKey
}

var PGPKeys = map[string]*PGPKeyData{
	"key-ubuntu-2018": {
		ID:          "871920D1991BC93C",
		PubKeyArmor: pubKeyUbuntu2018Armor,
	},
	"key-ubuntu-fips-v1": {
		ID:          "C1997C40EDE22758",
		PubKeyArmor: pubKeyUbuntuFIPSv1Armor,
	},
	"key-ubuntu-apps": {
		ID:          "AB01A101DB53907B",
		PubKeyArmor: pubKeyUbuntuAppsArmor,
	},
	"key-ubuntu-esm-v2": {
		ID:          "4067E40313CB4B13",
		PubKeyArmor: pubKeyUbuntuESMv2Armor,
	},
	"key1": {
		ID:           "854BAF1AA9D76600",
		PubKeyArmor:  pubKey1Armor,
		PrivKeyArmor: privKey1Armor,
	},
	"key2": {
		ID:           "9568570379BF1F43",
		PubKeyArmor:  pubKey2Armor,
		PrivKeyArmor: privKey2Armor,
	},
}

func init() {
	for name, key := range PGPKeys {
		if key.PubKeyArmor != "" {
			pubKeys, privKeys, err := pgputil.DecodeKeys([]byte(key.PubKeyArmor))
			if err != nil || len(privKeys) > 0 || len(pubKeys) != 1 || pubKeys[0].KeyIdString() != key.ID {
				log.Panicf("invalid public key armored data: %s", name)
			}
			key.PubKey = pubKeys[0]
		}
		if key.PrivKeyArmor != "" {
			pubKeys, privKeys, err := pgputil.DecodeKeys([]byte(key.PrivKeyArmor))
			if err != nil || len(pubKeys) > 0 || len(privKeys) != 1 || privKeys[0].KeyIdString() != key.ID {
				log.Panicf("invalid private key armored data: %s", name)
			}
			key.PrivKey = privKeys[0]
		}
	}
}

// Ubuntu Archive Automatic Signing Key (2018) <ftpmaster@ubuntu.com>.
// ID: 871920D1991BC93C.
// Useful to validate InRelease files from live archive.
const pubKeyUbuntu2018Armor = `
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
`

// Ubuntu Federal Information Processing Standards Automatic Signing Key V1 <esm@canonical.com>.
// ID: C1997C40EDE22758.
// Useful to validate InRelease files from live archive.
const pubKeyUbuntuFIPSv1Armor = `
-----BEGIN PGP PUBLIC KEY BLOCK-----

mQINBFzZxGABEADSWmX0+K//0cosKPyr5m1ewmwWKjRo/KBPTyR8icHhbBWfFd8T
DtYggvQHPU0YnKRcWits0et8JqSgZttNa28s7SaSUTBzfgzFJZgULAi/4i8u8TUj
+KH2zSoUX55NKC9aozba1cR66jM6O/BHXK5YoZzTpmiY1AHlIWAJ9s6cCClhnYMR
zwxSZVbefcYFbVPX/dQw/FMvJVeZ2aQ18NDgMQciu786aYklMFowxWNs/eLLTqum
cDHaw9UpKyhgfL/mkaIXuhYy6YRByYq1oOnJ5XffAOtovvCti1MvsPc0NDhPiGLf
9Fd/GtnqHxzVDqZmtUXX50mGu4LnJoHgWRjml3mapDPStzFr7Xgbb0NnyflmxnfN
kQcu2lFyXFfndWwg/RAOFdBPxBQhRK52uZiCfydKD7zCXz9YGm9xEK541EG0FrwA
6Vk1xaFol/jI8MQdP1o3JySX0Pqva3IHF7FHWHmxrIPaJLIHi0IrFG6Fgmk4sQ2w
XSc8kbxR+wYYKqIhBUZP0eb1jkFfvRVS6YvAy18xtw5pFD+VURdA0Uu5cotESfyz
oHsQ5R7wzg76oV/mYukHGC0x8peqxiPwbyhGFAhG8eUR66iYZgGbzmNI+OJz2EUi
UZJJXt4rnI1RVJLbhK9RjeobkOjf58Cm8RExlqJU16gy9saCMSiAqHx8swARAQAB
tFxVYnVudHUgRmVkZXJhbCBJbmZvcm1hdGlvbiBQcm9jZXNzaW5nIFN0YW5kYXJk
cyBBdXRvbWF0aWMgU2lnbmluZyBLZXkgVjEgPGVzbUBjYW5vbmljYWwuY29tPokC
OAQTAQIAIgUCXNnEYAIbAwYLCQgHAwIGFQgCCQoLBBYCAwECHgECF4AACgkQwZl8
QO3iJ1j4Vw//SawfmZi1GW+EUnuPqSz+zcmIKdx6AWZTe9/vSj6jgq4SYt//LAiD
NQz3dn2m0m5AaCucza2BCixUBrNhMh66m+lXfTqymUtTIpWpu4L1WLUbPjQ+s3Ad
xuF7S5wJtQrYmPvmZduZgg1wcb8eaqVltRJREpOP6sxcuqtvcfv4v4QYZ+iYd7eJ
8fxPOiyJEOTQPTdPZahYTaUOIloN5pT6uVg03u59Kh4aHCYxlRorvuRBabdctCfA
EBgomk4Us20Tv31dqlvMAiGKJqf1wdjhzlUmk4g/fOiRSNETKSC/VeUGH0fSbizl
Gs7Mg60jChPKpwzB6Rb5Nv2/Aw/FlSkfFhMdCdfKjl8IWOMPmElTVJFyVx1mmURi
3LgsloDFmJfebXefSFA7S8KLyBGlZJ/APaym64Ls12PUOjfh1Glie3E8KO66AGLo
ID1dQnzRizuHxW80ET03dSjzTXHLSi+iFycmNAxo6gB3GyOQ8tlIHjo1FfDfNYDf
qKic3Q0B9TvF6hqVRIcyePK4lN5YtRpVRdVj/jv8AqbzaIaVCP4k4nNrbaVx5zQf
BWq2E9IH+vLZfPyiP+hwxswfrlU3mrXBpPStIxq41yXFwQiDnqgkhEVAcrYPjBnS
T6s3+b+4HbAW6mbp4jEHUd/F1+iXz90T2WArrNIkMbmpChMuSyRN8Hc=
=DWhM
-----END PGP PUBLIC KEY BLOCK-----
`

// Ubuntu Apps Automatic Signing Key <esm@canonical.com>.
// ID: AB01A101DB53907B.
// Useful to validate InRelease files from live archive.
const pubKeyUbuntuAppsArmor = `
-----BEGIN PGP PUBLIC KEY BLOCK-----

mQINBF3WVA4BEAC7MDr8HClfKptSd4VeB12Vy+Ao/4NpY2ITdkRed4vfh/4eBWWn
3+in6So2ekweifACSxScB/M9zVObsI1cab7QPMkIiATNUfIyOEP7iNWLX4+AytM1
LP3bZo8OpghnLZNstCGbiRUO4CDNmCI04DOPCu9EVEO4WWNuWIMRwCLShDSf7Cid
J2fn2TT/7vsmA4eI3YnAne+u8g4X2zMHQFkHANhylB0lPyThXo5jaxHImzm4wf/2
LF8f1Y1nRQObS2jcvYc3fm9B7iOGpyNAw3h6hrPKH5T9tY/ZoMtFHqn66J1CBSHb
hDkEvA46X50su4yAHeSiEG/hMYG7SoHzmAsjEXnvkTIE41WhmxlidQnRs2uWy34U
7VmOpaidWn3R99fNHYOtSOB6bpIvls8snWSQ63jcFXnt05nVZsp/Ixzl0Oqitynx
DFwoxEwt3ZuCHwxbx2vZ+FiZXVFN7I0IyBDOEL6XS27FNaMCZ7Q/6z/ckdWto55E
264OWf9lnw31bXFXHWSusRXWzD6FK8dqWgjtrWwRxlvF4jm688lqpjac6fFES3UK
BhjyHXFGL/+HHZ9CNxlLYF5QnXq1mGR0Ykw975u8KoOFSLBqsx+1a21m6dfzujY7
2Gq6Sju+9Yo1aOF+CNvTMYdRBoDL4sFj6VAmUsszMA5aAb+82pOCaDvGJQARAQAB
tDVVYnVudHUgQXBwcyBBdXRvbWF0aWMgU2lnbmluZyBLZXkgPGVzbUBjYW5vbmlj
YWwuY29tPokCOAQTAQIAIgUCXdZUDgIbAwYLCQgHAwIGFQgCCQoLBBYCAwECHgEC
F4AACgkQqwGhAdtTkHuTOw/8Czv42TSpwHz+eNtl3ZFyxta9rR/qWC3h+vMu0R/l
5KU3aQQOygWOoUcr1QTPSSg3v/H+v/8vqVq2UuUxSIfpMxBj2kIX2vqskv6Roez7
xR8lVDa0a47z/NYMfKpxrEJxOLh/c7I6aAsa597bTqDHtucHL/22BvfUJJqw6jq1
7SswP5lqKPBFz7x+E2hgfJE7Vn7h0ICm29FkWnOeTKfj8VwTAeKXKUI9Hw6+aqr9
29Y2NdLsYZ57mpivRLNM9sBZoF3avP1pUC2k0IwP3dwh4AxUMXjRRPh173iXBfR2
yAf1lWET/5+8dSBrfFIZSo+FF/EEBmqIVtJpHkq8+YxUbCLbkoikRi2kwrgyXLEn
FqxSU2Ab0xurFHiHcJoCGVD38xjznO5cQl7H4K9+B/rFpTTowOHbOcFpKAzpYqB5
8rnR1yRSsB33zac8xesUIfzYWRtLc5/VIb5mOkWlb62d8emILx2XuRFVjKq6mKki
oGckhDUOuEFrjW1cQq+PWBBxyJoXcy6wGSoPJ/ELeaf9zg8SF0jwuN6BPHVBeJ/E
W53zR5iV0N9fRT+M2JN5tc5HenO92xLgPAh+GPWLYmPdTmHu+kFozqsHx/NUw2iP
PBL6Q1VZytt2Uf6qLPUx7GpYMKf42Vldb0feFo/YA/lzOgPlY29pDLKXbse6o+Sr
kmk=
=AEEr
-----END PGP PUBLIC KEY BLOCK-----
`

// Ubuntu Extended Security Maintenance Automatic Signing Key v2 <esm@canonical.com>.
// ID: 4067E40313CB4B13.
// Useful to validate InRelease files from live archive.
const pubKeyUbuntuESMv2Armor = `
-----BEGIN PGP PUBLIC KEY BLOCK-----

mQINBFy2kH0BEADl/2e2pULZaSRovd3E1i1cVk3zebzndHZm/hK8/Srx69ivw3pY
680gFE/N3s3R/C5Jh9ThdD1zpGmxVdqcABSPmW1FczdFZY2E37HMH7Uijs4CsnFs
8nrNGQaqX/T1g2fQqjia3zkabMeehUEZC5GPYjpeeFW6Wy1O1A1Tzu7/Wjc+uF/t
YYe/ZPXea74QZphu/N+8dy/ts/IzL2VtXuxiegGLfBFqzgZuBmlxXHVhftKvcis9
t2ko65uVyDcLtItMhSJokKBsIYJliqOXjUbQf5dz8vLXkku94arBMgsxDWT4K/xI
OTsaI/GMlSIKQ6Ucd/GKrBEsy5O8RDtD9A2klV7YeEwPEgqL+RhpdxAs/xUeTOZG
JKwuvlBjzIhJF9bIfbyzx7DdcGFqRE+a8eBIUMQjVkt9Yk7jj0eV3oVTE7XNhb53
rHuPL+zJVkiharxiTgYvkow3Nlbg3oURx9Ln67ni9pUtI1HbortGZsAkyOcpep58
K9cYvUePJWzjkY+bjcGKR19CWPl7KaUalIf2Tao5OwtqjrblTsXdtV7eG45ys0MT
Kl/DeqTJ0w6+i4eq4ZUfOCL/DIwS5zUB9j1KMUgEfocjYIdHWI8TSrA8jLYNPbVE
6+WjekHMB9liNrEQoESWBddS+bglPxuVwy2paGTUYJW1GnRZOTD+CG4ETQARAQAB
tFFVYnVudHUgRXh0ZW5kZWQgU2VjdXJpdHkgTWFpbnRlbmFuY2UgQXV0b21hdGlj
IFNpZ25pbmcgS2V5IHYyIDxlc21AY2Fub25pY2FsLmNvbT6JAjgEEwECACIFAly2
kH0CGwMGCwkIBwMCBhUIAgkKCwQWAgMBAh4BAheAAAoJEEBn5AMTy0sTo/8QAJ1C
NhAkZ+Xq/BZ8UzAFCQn6GlIYg/ueY216xcQdDX1uN8hNOlPTNmftroIvohFAfFtB
m5galzY3DBPU8eZr8Y8XgiGD97wkR4zfhfh1EK/6diMG/HG00kdcWquFXMRB7E7S
nDTpyuPfkAzm9n6l69UB3UA53CaEUuVJ7qFfZsWgiQeUJpvqD0MIVsWr+T/paSx7
1JE9BVatFefq0egErv1sa2uYgcH9TRZMLw6gYxWtXeGA08Cpp0+OEvIzmJOHo5/F
EpJ3hGk87Of77BC7FbqSDpeYkcjnlI2i0QAxxFygKhPOMLuA4XVn3TDuqCgTFIFC
puupzIX/Up51FJmo64V9GZ/uF0jZy4tDxsCRJnEV+4Kv2sU5uMlmNchZMBjXYGiG
tpH9CqJkSZjFvB6bk+Ot98KI6+CuNWn1N0sXFKpEUGdJLuOKfJ9+xI5plo8Bct5C
DM9s4l0IuAPCsyayXrSmlyOAHzxDUeRMCEUnXWfycCUyqdyYIcCMPLV44Ccg9NyS
89dEauSCPuyCSxm5UYEHQdsSI/+rxRdS9IzoKs4za2L7fhY8PfdPlmghmXc/chz1
RtgjPfAsUHUPRr0h//TzxRm5dbYdUyqMPzZcDO8wYBT/4xrwnFkSHZhnVxpw7PDi
JYK4SVVc4ZO20PE1+RZc5oSbt4hRbFTCSb31Pydc
=KWLs
-----END PGP PUBLIC KEY BLOCK-----
`

// Test-purpose RSA 2048 bits signing key-pairs without a passphrase.
// ID: 854BAF1AA9D76600. User: "foo-bar <foo@bar>".
const pubKey1Armor = `
-----BEGIN PGP PUBLIC KEY BLOCK-----

mQENBGVs8P4BCADPh/fNnw2AI1JCYf+3p4jkcFQPLVsUkoTZk8OXjCxy+UP9Jd2m
xnxat7a0JEJZa0aWCmtlSL1XR+kFKBrd7Ry5jOHYjuDKx4kTmDUbezPnjoZIGDNX
j5cdNuMLpOINZweNNWDKRdRvhj5QX89/DYwPrLkNFwwjXjlj5tjU6RUkROYJBGPe
G2ns2cZtVbYMh3FDU9YRfp/hUqGVf+UFRyUw+mo1TUlk5F7fnfwEQmsppDHvfTNJ
yjEMZD7nReTEeMy12GV2wysOwWMPEb2PSE/+Od7AKn5dFA7w3kyLCzAxYp6o7IE/
+RY8YzAJe6GmLwhTWtylMV1xteQhZkEe/QGXABEBAAG0EWZvby1iYXIgPGZvb0Bi
YXI+iQFOBBMBCgA4FiEEDp0LAdsRnT9gfhU5hUuvGqnXZgAFAmVs8P4CGwMFCwkI
BwIGFQoJCAsCBBYCAwECHgECF4AACgkQhUuvGqnXZgCHZAf/b/rkMz2UY42LhuvJ
xDW7KbdBI+UgFp2k2tg2SkLM27GdcztpcNn/RE9U1vc8uCI05MbMhKQ+oq4RmO6i
QbCPPGy1Mgf61Fku0JTZGEKg+4DKNmnVkSpiOc03z3G2Gyi2m9G2u+HdJhXHumej
7NXkQvVFxXzDnzntbnmkM0fMfO+wdP5/EFjJbHC47yAAds/yspfk5qIHu6PHrTVB
+wJGwOJdwJ1+2zis5ONE8NexfSrDzjGJoKAFtlMwNNDZ39JlkguMB0M5SxoGRXxQ
ZE4DhPntUIW0qsE6ChmmjssjSDeg75rwgc+hjNDunKQhKNpjVVFGF4uceV5EQ084
F4nA5w==
=ZXap
-----END PGP PUBLIC KEY BLOCK-----
`
const privKey1Armor = `
-----BEGIN PGP PRIVATE KEY BLOCK-----

lQOYBGVs8P4BCADPh/fNnw2AI1JCYf+3p4jkcFQPLVsUkoTZk8OXjCxy+UP9Jd2m
xnxat7a0JEJZa0aWCmtlSL1XR+kFKBrd7Ry5jOHYjuDKx4kTmDUbezPnjoZIGDNX
j5cdNuMLpOINZweNNWDKRdRvhj5QX89/DYwPrLkNFwwjXjlj5tjU6RUkROYJBGPe
G2ns2cZtVbYMh3FDU9YRfp/hUqGVf+UFRyUw+mo1TUlk5F7fnfwEQmsppDHvfTNJ
yjEMZD7nReTEeMy12GV2wysOwWMPEb2PSE/+Od7AKn5dFA7w3kyLCzAxYp6o7IE/
+RY8YzAJe6GmLwhTWtylMV1xteQhZkEe/QGXABEBAAEAB/4jvxdbdyiTqEHchlXO
NBDbzE9mV9km53/znESl/3KOkUn5OkL+HZVA6QES8WXuUhCT+pJ6HTfj51KHXVuX
W2bFvTMPorispQcC9YY8SBHuMjoGBAkf7W9JjHE6SbnYNiVyWL3lyXZoiVaFcKNk
jphQAN/VFeG029+FyjcSIV3PY7FWI4Q1dyqyf78iWa6I400cmyGFvZDSps/oo3sT
0xcjdLL5AaXyR0FtZoSrltioYzp4cnYDI2ES9PT7uR6MQ7AwUamUQ/7dUR6zSi1o
NbHVOYItsZEsY8N/1vUxW+Ps0bbgZd9ob6n+1beQIeSMhJiW0g2NiqlZXo8GELNp
LNOBBADl+tu0iX0DCTJ5fnDeiWgMv+sPA9pcACKhnxDuOXMJjV/gGY2XtKzP0o68
y8N5Nry0UG3wHMlgqp5qY8ZkXfH3zMmIezG5C6HZQ7A44wem3iBYj8Z1bjpT8AW7
rFi+1iBDmZ4whHzsxLp8XB/cugAh/g3bo6rJl2bCaQPnpsSygQQA5wLnFL8pnj4M
kNzefp/ZFGTstB7AC1Dfkja9QTfimZpJZj/5XXyewAgmqQt9uersmLHfXhS3sgrk
kko74ZEZY5PCInsbcvUkgRxgw/JnjWdHLVUOMMd12RVQU9BOVf2kN8sEWCQbqzsM
H9IEtFjXXyyubmb4euI25xs1ptxk+BcD/j1J5bu6RZfP2IfEeBPu4w8zK5WOioLY
dia8kvzScIRvREB6DbYCifirx0gSuZSCyo+zm/KfZCof89ihOZ4e3OAWQDqajfQH
AGoXJCN9LRJsGe/x79LHuOx71x1MbTTvOUlYJTD9+cHzWRzKHb2ecFL6jaJb4OhY
RP4t194OXMHdQ2q0EWZvby1iYXIgPGZvb0BiYXI+iQFOBBMBCgA4FiEEDp0LAdsR
nT9gfhU5hUuvGqnXZgAFAmVs8P4CGwMFCwkIBwIGFQoJCAsCBBYCAwECHgECF4AA
CgkQhUuvGqnXZgCHZAf/b/rkMz2UY42LhuvJxDW7KbdBI+UgFp2k2tg2SkLM27Gd
cztpcNn/RE9U1vc8uCI05MbMhKQ+oq4RmO6iQbCPPGy1Mgf61Fku0JTZGEKg+4DK
NmnVkSpiOc03z3G2Gyi2m9G2u+HdJhXHumej7NXkQvVFxXzDnzntbnmkM0fMfO+w
dP5/EFjJbHC47yAAds/yspfk5qIHu6PHrTVB+wJGwOJdwJ1+2zis5ONE8NexfSrD
zjGJoKAFtlMwNNDZ39JlkguMB0M5SxoGRXxQZE4DhPntUIW0qsE6ChmmjssjSDeg
75rwgc+hjNDunKQhKNpjVVFGF4uceV5EQ084F4nA5w==
=VBWI
-----END PGP PRIVATE KEY BLOCK-----
`

// Test-purpose RSA 1024 bits signing key-pairs without a passphrase.
// ID: 9568570379BF1F43. User: "Extra Test Key <test@key>".
const pubKey2Armor = `
-----BEGIN PGP PUBLIC KEY BLOCK-----

mI0EZXAwcgEEAMBQ4Qx6xam1k1hyjPrKQfCnGRBBm2+Lw9DHQcz0lreH51iZEVkS
fACbPHI9A7NX8xdX1cMLpaTQCT3h30WwuLuNAo1IdYcdGpfzFzd6rqS5OCItj+3u
XZrTlS8QxVVShSPYFfxYaIXKCZF9G+RTKD0rWQwkMwNHZ4vJGBm7qKytABEBAAG0
GUV4dHJhIFRlc3QgS2V5IDx0ZXN0QGtleT6IzgQTAQoAOBYhBFQrMoAoDwtUfB+G
BpVoVwN5vx9DBQJlcDByAhsDBQsJCAcCBhUKCQgLAgQWAgMBAh4BAheAAAoJEJVo
VwN5vx9Dy80D/iUzJkfT8lsH0vZ2jcpgcyjtZqrIfOMLYk8DqoYD/1wDGx4TIzg/
bpqDHxBCDmBaxY6+ps9IaBcsD1whjyX4AZK6FykV8d9GAc+3b9t2EPe92LV3XKaT
rwF9bjDSJZUUz1I31YTnHpBiRU+hWuf7OVjnLcEAB8mMa7Y6YN37qT44
=U79/
-----END PGP PUBLIC KEY BLOCK-----
`
const privKey2Armor = `
-----BEGIN PGP PRIVATE KEY BLOCK-----

lQHYBGVwMHIBBADAUOEMesWptZNYcoz6ykHwpxkQQZtvi8PQx0HM9Ja3h+dYmRFZ
EnwAmzxyPQOzV/MXV9XDC6Wk0Ak94d9FsLi7jQKNSHWHHRqX8xc3eq6kuTgiLY/t
7l2a05UvEMVVUoUj2BX8WGiFygmRfRvkUyg9K1kMJDMDR2eLyRgZu6isrQARAQAB
AAP+LXyDuiSor0rt0o/ndeLURVP0auKlnbS4SB902gHoyvh3OL6deoyTbT5KRffV
8fuFmNoSymrtDwYQhYUwvqY9jt+lVSKDseqLkF5C92VZFWpjiYDOqZzoBfVUDZo5
NffyIxuG5X33o9yBmUk29PWcLqzSanxg/TmXy63pp4sBYfECAN3GgiWxwrQTtv0X
OUuSKbvnDVyM86R7Hdo08hmwB/6qhGibw5KBko+h+kBsIo1naEzzGsXWUjLk8BbZ
qPTRGrECAN3+ijctJPm+JprWjJlJ5KrdXlIMG5x87vtdp5ZzctsmY97GMBaW+SvW
uuBHfiY7xFUru8304gWd/YAwTdxVeL0CALjGKCTWPhZaRJ+ew9iryVgFEznaNAgO
pzVXr3yllNdinGWjvbyEkn1y7OlzH0REg9jOsc82Bbz4aiDm19Qr/1KtR7QZRXh0
cmEgVGVzdCBLZXkgPHRlc3RAa2V5PojOBBMBCgA4FiEEVCsygCgPC1R8H4YGlWhX
A3m/H0MFAmVwMHICGwMFCwkIBwIGFQoJCAsCBBYCAwECHgECF4AACgkQlWhXA3m/
H0PLzQP+JTMmR9PyWwfS9naNymBzKO1mqsh84wtiTwOqhgP/XAMbHhMjOD9umoMf
EEIOYFrFjr6mz0hoFywPXCGPJfgBkroXKRXx30YBz7dv23YQ973YtXdcppOvAX1u
MNIllRTPUjfVhOcekGJFT6Fa5/s5WOctwQAHyYxrtjpg3fupPjg=
=JbF+
-----END PGP PRIVATE KEY BLOCK-----
`
