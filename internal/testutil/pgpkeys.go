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
