package setup_test

import (
	"golang.org/x/crypto/openpgp/packet"
	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/setup"
)

type archiveKeyTest struct {
	summary  string
	armored  string
	relerror string
	pubKey   *packet.PublicKey
}

var archiveKeyTests = []archiveKeyTest{{
	summary: "Armored data with one public key",
	armored: testKey.ArmoredPublicKey,
	pubKey:  testKey.PublicKey,
}, {
	summary:  "Armored data with two public keys",
	armored:  twoPubKeysArmored,
	relerror: "armored data contains more than one public key",
}, {
	summary:  "Armored data with no public key",
	armored:  armoredDataWithNoKeys,
	relerror: "armored data contains no public key",
}, {
	summary:  "Armored data with private key",
	armored:  testKey.ArmoredPrivateKey,
	relerror: "armored data contains private key",
}, {
	summary: "Invalid armored data",
	armored: `
		Roses are red
		Violets are blue
	`,
	relerror: "cannot decode armored data",
}, {
	summary:  "Empty armored data",
	relerror: "cannot decode armored data",
}, {
	summary:  "Armored data: bad packets",
	armored:  invalidArmoredKey,
	relerror: "openpgp: .*",
}}

func (s *S) TestDecodeArchivePubKey(c *C) {
	for _, test := range archiveKeyTests {
		c.Logf("Summary: %s", test.summary)

		pubKey, err := setup.DecodePublicKey([]byte(test.armored))
		if test.relerror != "" {
			c.Assert(err, ErrorMatches, test.relerror)
			continue
		}
		c.Assert(err, IsNil)

		c.Assert(pubKey, DeepEquals, test.pubKey)
	}
}

type verifyClearSignTest struct {
	summary   string
	clearData string
	pubKeys   []*packet.PublicKey
	relerror  string
}

var verifyClearSignTests = []verifyClearSignTest{{
	summary:   "Good data with proper sign",
	clearData: clearSignedData,
	pubKeys:   []*packet.PublicKey{testKey.PublicKey},
}, {
	summary:   "Good data with multiple signatures",
	clearData: clearSignedWithMultipleSigns,
	pubKeys:   []*packet.PublicKey{testKey.PublicKey, extraTestKey.PublicKey},
}, {
	summary:   "Multiple signatures: verify at least one signature",
	clearData: clearSignedWithMultipleSigns,
	pubKeys:   []*packet.PublicKey{testKey.PublicKey},
}, {
	summary:   "Multiple signatures: no valid public keys",
	clearData: clearSignedWithMultipleSigns,
	relerror:  "cannot verify any signatures",
}, {
	summary:   "Invalid data: improper hash",
	clearData: invalidSignedData,
	pubKeys:   []*packet.PublicKey{testKey.PublicKey},
	relerror:  "openpgp: .*invalid signature: hash tag doesn't match.*",
}, {
	summary:   "Invalid data: bad packets",
	clearData: invalidSignedDataBadPackets,
	pubKeys:   []*packet.PublicKey{testKey.PublicKey},
	relerror:  "error parsing armored data:.*",
}, {
	summary:   "Invalid data: malformed clearsign text",
	clearData: "foo\n",
	pubKeys:   []*packet.PublicKey{testKey.PublicKey},
	relerror:  "invalid clearsign text.*",
}, {
	summary:   "Wrong public key to verify with",
	clearData: clearSignedData,
	pubKeys:   []*packet.PublicKey{extraTestKey.PublicKey},
	relerror:  "openpgp: .*invalid signature:.*verification failure",
}}

func (s *S) TestVerifySignature(c *C) {
	for _, test := range verifyClearSignTests {
		c.Logf("Summary: %s", test.summary)

		sigs, body, _, err := setup.DecodeClearSigned([]byte(test.clearData))
		if err == nil {
			err = setup.VerifyAnySignature(test.pubKeys, sigs, body)
		}
		if test.relerror != "" {
			c.Assert(err, ErrorMatches, test.relerror)
			continue
		} else {
			c.Assert(err, IsNil)
		}
	}
}

// twoPubKeysArmored contains two public keys:
//   - 854BAF1AA9D76600 ("foo-bar <foo@bar>")
//   - 871920D1991BC93C ("Ubuntu Archive Automatic Signing Key (2018) <ftpmaster@ubuntu.com>")
const twoPubKeysArmored = `
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
F4nA55kCDQRbn8HaARAA7/xscrcfy3El2LjNDMCqI2wcnvNbNBtZxMfpc+lQFKSF
GZ25KnVwRwvncKxkvwnni7gIz0S1PAKMRP4472VafMRRhFh2HZJalxmf4CXz+Xd3
yFAbWR2RCZfAfJvaTB3/wEEHbAvmM4s0hubeTIZ6LcNOOC17XRBJMdreic9Dhq4f
uSKMal+6WYqugr9fQaIWlIqCjHaexEukWHze6Jeh0ixZazF7VX4f4o6TfY92YVRl
XkQvJCh0LCeT5CG5r8QYlIe0iZn2VMdCEITTGgx133WQBjbZ4c8zUXm9RajS0lZK
0vz57AEMzIRtQQ5tlTkheuI3myl33xajOS10UE3qky7I1G266kerPxgjvFBe431I
+iO7Wi8oJrBzvyQ+I6SkQtIG6VAX2oici77nqcd5FqKi97DdC4ZTCPNPnwOxk76D
seLaalZc5ROk2o2Lvo31t0KThUuXsBDHS9uoc8bGYP4Hmb02wK3D/jrCSkZob+JD
aOgMnch0P92Vf391/Zk9/0jy2yWrppIKd2M3ereT3gbvmUJP5jeVjTbmooTRFe5Z
W9WYb2NBcbvQVXfwTZdK87sad6yIpwdk19kgoO8BOcV5MF7kP9nkwxNL9B5Rp7ZL
mYxqMA2ZMR2UEsWVTs3WQkVWl/1hBS6SmtgEKcOUSa0OKGfzn4n18icz9u6NN8EA
EQEAAbRCVWJ1bnR1IEFyY2hpdmUgQXV0b21hdGljIFNpZ25pbmcgS2V5ICgyMDE4
KSA8ZnRwbWFzdGVyQHVidW50dS5jb20+iQI4BBMBCgAiBQJbn8HaAhsDBgsJCAcD
AgYVCAIJCgsEFgIDAQIeAQIXgAAKCRCHGSDRmRvJPCxzEACktnJ8c/+VmqAjlgK3
+YOlB23jgoHOQwZtIQrhQ2Vlr+Nu2hnotwj7i8NAxiwl2XcnOXahPJr4zJTppgCi
pY9bhoN02Am0Fo1j3jJwT2W5BYJGaFye/+gge21kYbdbB86bdS02fkmA8DsCevEE
aew0WmZfWOkIlG3roatg1HE6H1WwcW4a3JDeGbXi75vv5xvZv3IqKXOui8EXZMan
yd9gsqvtU0uVWiCQxuw1s4hvim7uqggzOEDZYNyx+6deAq0cQG3OJb6IUYLFeHkK
rCHHRZLlWORzz49ivE6qWOkk3vBodGqaxtUVfGSmstykjKZ8ldXwCp+HzPW8oi80
AKLwtC2fTDDLKwEv+OQLwtyBCkkoYyxZ9V9XUQojuv+45mRKGbQKed4ZH/EjAbIu
/IVTawbpmcHyHQQNb9tvi2OMUCvKuFwqEXAPRvqb81PWFVu3EZw2WRpdLsDsO8/T
5EAReShSo1g8+HwpPiuvmLRqaLxinpBgW/COxAOlKbz4KgP0HSNLdSAT9DdOkUHL
NX1GgEBLc+gxsuc5EYUeKRkmZ/nRRE+z3QIxCvOMuwXWOLflNY3EiLwY9Bdgey8E
S+8RqUqSCov3pAFy7Nde27xR2gr5lGDeyVadRjJlRcYSHceghZt38RvEIzW+bXq3
v2KivrjoHF58tVJcLQlM5a0mj4kCMwQQAQoAHRYhBBU/HJ7xOV+/ADUujQv7hH8/
Jy9bBQJbn8RDAAoJEAv7hH8/Jy9bbhcP/RoGnoILwp9eUKZQAWvOjkXiQEcZwMaW
i9tt6S5IAGwWADk+z5k48MBwqhniWRi8wELBi3OlpEA3oHsEAjFi6ftczh5lAR22
T7M9xO+gHN/NRQF4WQY/DC23MjkTrCmCmfTP8hnqzKVceAfFjW+T/rfbbQMMAEf5
TbOTkt5aVeJ5MCM78QOlp6tIFigS//a3O7C/qlniQ50BJKtWf3TQW4CFpLQ7aniF
xZXYI2Dl/sdUTfNW3i1Q7US6DlNCJELBRmjjm9KNsfP3ZmDNnF7nITRmJnWNmeY3
iyNRdHcwkfgkVBAxXa9HBfeFEoFRlsgqGh3QAU0Q+Xv7iBMki9E/cpvd0TQbaHPY
DxDRQdgEjCYJDDSDYlfNmpT42GK27PmVR7i0CIHfqsPzes8C7VQ4KNj3OhV2aapk
o0UZrQUSbr/lZwwXgDrLZdEJaEZuYEQaf8ILfdxNQIfkCUVbjEBas9Jz2Vk8H3Bm
oJkhLq1oil/J9hRWJIi38lFtN9+UzfPGfffoL0PgMkgBbvEXk/5UMwD0TzUS46QJ
OXtRbjM0GKASXGMD9LIwCDtQFpoLjyNSi351Z157E+SWDGxtgwixyPziL56UavL/
eeYJWeS/WqvGzZzsAtgSujFVLKWyUaRi0NvYW3h/I50Tzj0Pkm8GtgvP2UqAWvy+
iRpeUQ2ji0Nc
=3HS8
-----END PGP PUBLIC KEY BLOCK-----
`

// clearSignedData is signed with key 854BAF1AA9D76600 (foo@bar).
const clearSignedData = `
-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA512

foo
-----BEGIN PGP SIGNATURE-----

iQE8BAEBCgAmFiEEDp0LAdsRnT9gfhU5hUuvGqnXZgAFAmVuylQIHGZvb0BiYXIA
CgkQhUuvGqnXZgDB5wf/UaxTLwO22BQdpjtkRWoI9EooNr02K5jW7x4Y73akuBFt
EJi1bUPrNKFqL7VDTMiaRv+1RSytY9U3+AKgMKVq1p7Iwr2t6CLs3D7bqw9Vy2Z4
SpjS8zZQ5H+7t0O2zqNSu4UqBTCXWIsW9EiL1EHr92F2O3HhOn1ER7KgTl+GDUZ/
4szrBZsfltvX51UMvFD1TO9EYcJ4tzB6mvftTBZZ6KeoyUC5u4a1ZljYkujWAlFW
VvD4PlSNTcSmpZTICEmLmb3DLlXezQ0Rgfwy6Q6X0kt9xztIJsNo5sgRxQUlpVl3
5VFsefx4LxtZvdSFK0SNh7UAhdOzD5Tc/7aG0NFfjw==
=BAhz
-----END PGP SIGNATURE-----
`

// This data contains two signatures, from these keys:
//   - 854BAF1AA9D76600	(foo@bar)
//   - 9568570379BF1F43	(test@key)
const clearSignedWithMultipleSigns = `
-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA512

foo
-----BEGIN PGP SIGNATURE-----

iQEzBAEBCgAdFiEEDp0LAdsRnT9gfhU5hUuvGqnXZgAFAmVxmacACgkQhUuvGqnX
ZgBarggAp4YWGia+9yUJG3ojieaSFnue9Ov/YnhV3PLKqxo1+DJZZDekxuxk7sbU
x+ZQmM/3xus1MhEVmySvwEiuGktr9fk+/eEZOZj6d4ZTTloUeDZNaJ7LSUEUKdMM
HA5Adphtv+vBZwmkH6u7jyJSGC+P/U7DFmIPODeDcqLzh5hjWWK1dkNqkwEF75Ot
9AXI5Y0e4WWJj/UQ1zuUwtw9Rf4JB8MUFOVUPJe4UFZw+XUYHq5DFBNYLn2SDLMQ
BQ3hzmDE9FazILBIFfutKTpA3gmPu9wZ+WroNXkKkleV0Wjo0kA4bnz5hLy2D4Bf
DBATaX5qzUwC9LxpzNJoScsW/2U+KYizBAEBCgAdFiEEVCsygCgPC1R8H4YGlWhX
A3m/H0MFAmVxmacACgkQlWhXA3m/H0PVCgQArXUt7hQO3bATZBsbTgQ2INhs1aiR
GAWkroW5Dp5mOmTtAtfFuysEMdH+v42Z6g1BqwypWtCVNYF+v8aQYwUwUulN/Pna
qtWNWLmXMFLmNVILL9X+o/sRCtra1qCu6Vn59H+yPhye9CXiV+U/V8dB60YLs812
cgcXWByCFx3J1hM=
=1GLl
-----END PGP SIGNATURE-----
`

// This should be an invalid clearsign data. Obtained by changing
// "foo" to "bar" in clearSignedData defined above.
const invalidSignedData = `
-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA512

bar
-----BEGIN PGP SIGNATURE-----

iQE8BAEBCgAmFiEEDp0LAdsRnT9gfhU5hUuvGqnXZgAFAmVuylQIHGZvb0BiYXIA
CgkQhUuvGqnXZgDB5wf/UaxTLwO22BQdpjtkRWoI9EooNr02K5jW7x4Y73akuBFt
EJi1bUPrNKFqL7VDTMiaRv+1RSytY9U3+AKgMKVq1p7Iwr2t6CLs3D7bqw9Vy2Z4
SpjS8zZQ5H+7t0O2zqNSu4UqBTCXWIsW9EiL1EHr92F2O3HhOn1ER7KgTl+GDUZ/
4szrBZsfltvX51UMvFD1TO9EYcJ4tzB6mvftTBZZ6KeoyUC5u4a1ZljYkujWAlFW
VvD4PlSNTcSmpZTICEmLmb3DLlXezQ0Rgfwy6Q6X0kt9xztIJsNo5sgRxQUlpVl3
5VFsefx4LxtZvdSFK0SNh7UAhdOzD5Tc/7aG0NFfjw==
=BAhz
-----END PGP SIGNATURE-----
`

// This should be an invalid clearsign data with invalid packets.
// Obtained by removing some lines from clearSignedData above.
const invalidSignedDataBadPackets = `
-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA512

foo
-----BEGIN PGP SIGNATURE-----

qtWNWLmXMFLmNVILL9X+o/sRCtra1qCu6Vn59H+yPhye9CXiV+U/V8dB60YLs812
cgcXWByCFx3J1hM=
=1GLl
-----END PGP SIGNATURE-----
`

// armoredDataWithNoKeys contains only a signature packet, to be
// used for testing purposes. It does not contain any key packets.
const armoredDataWithNoKeys = `
-----BEGIN PGP ARMORED FILE-----
Comment: Use "gpg --dearmor" for unpacking

iQI4BBMBCgAiBQJbn8HaAhsDBgsJCAcDAgYVCAIJCgsEFgIDAQIeAQIXgAAKCRCH
GSDRmRvJPCxzEACktnJ8c/+VmqAjlgK3+YOlB23jgoHOQwZtIQrhQ2Vlr+Nu2hno
twj7i8NAxiwl2XcnOXahPJr4zJTppgCipY9bhoN02Am0Fo1j3jJwT2W5BYJGaFye
/+gge21kYbdbB86bdS02fkmA8DsCevEEaew0WmZfWOkIlG3roatg1HE6H1WwcW4a
3JDeGbXi75vv5xvZv3IqKXOui8EXZManyd9gsqvtU0uVWiCQxuw1s4hvim7uqggz
OEDZYNyx+6deAq0cQG3OJb6IUYLFeHkKrCHHRZLlWORzz49ivE6qWOkk3vBodGqa
xtUVfGSmstykjKZ8ldXwCp+HzPW8oi80AKLwtC2fTDDLKwEv+OQLwtyBCkkoYyxZ
9V9XUQojuv+45mRKGbQKed4ZH/EjAbIu/IVTawbpmcHyHQQNb9tvi2OMUCvKuFwq
EXAPRvqb81PWFVu3EZw2WRpdLsDsO8/T5EAReShSo1g8+HwpPiuvmLRqaLxinpBg
W/COxAOlKbz4KgP0HSNLdSAT9DdOkUHLNX1GgEBLc+gxsuc5EYUeKRkmZ/nRRE+z
3QIxCvOMuwXWOLflNY3EiLwY9Bdgey8ES+8RqUqSCov3pAFy7Nde27xR2gr5lGDe
yVadRjJlRcYSHceghZt38RvEIzW+bXq3v2KivrjoHF58tVJcLQlM5a0mjw==
=cp5f
-----END PGP ARMORED FILE-----
`

// invalidArmoredKey contains bad packets.
const invalidArmoredKey = `
-----BEGIN PGP PUBLIC KEY BLOCK-----

mI0EZXAwcgEEAMBQ4Qx6xam1k1hyjPrKQfCnGRBBm2+Lw9DHQcz0lreH51iZEVkS
=U79/
-----END PGP PUBLIC KEY BLOCK-----
`
