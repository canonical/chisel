package pgputil_test

import (
	"golang.org/x/crypto/openpgp/packet"
	. "gopkg.in/check.v1"

	"github.com/canonical/chisel/internal/pgputil"
	"github.com/canonical/chisel/internal/testutil"
)

var (
	key1 = testutil.PGPKeys["key1"]
	key2 = testutil.PGPKeys["key2"]
)

type archiveKeyTest struct {
	summary  string
	armor    string
	relerror string
	pubKey   *packet.PublicKey
}

var archiveKeyTests = []archiveKeyTest{{
	summary: "Armored data with one public key",
	armor:   key1.PubKeyArmor,
	pubKey:  key1.PubKey,
}, {
	summary:  "Armored data with two public keys",
	armor:    twoPubKeysArmor,
	relerror: "armored data contains more than one public key",
}, {
	summary:  "Armored data with no public key",
	armor:    armoredDataWithNoKeys,
	relerror: "armored data contains no public key",
}, {
	summary:  "Armored data with private key",
	armor:    key1.PrivKeyArmor,
	relerror: "armored data contains private key",
}, {
	summary: "Invalid armored data",
	armor: `
		Roses are red
		Violets are blue
	`,
	relerror: "cannot decode armored data",
}, {
	summary:  "Empty armored data",
	relerror: "cannot decode armored data",
}, {
	summary:  "Armored data: bad packets",
	armor:    invalidPubKeyArmor,
	relerror: "openpgp: .*",
}}

func (s *S) TestDecodeArchivePubKey(c *C) {
	for _, test := range archiveKeyTests {
		c.Logf("Summary: %s", test.summary)

		pubKey, err := pgputil.DecodePubKey([]byte(test.armor))
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
	pubKeys:   []*packet.PublicKey{key1.PubKey},
}, {
	summary:   "Good data with multiple signatures",
	clearData: clearSignedWithMultipleSigns,
	pubKeys:   []*packet.PublicKey{key1.PubKey, key2.PubKey},
}, {
	summary:   "Multiple signatures: verify at least one signature",
	clearData: clearSignedWithMultipleSigns,
	pubKeys:   []*packet.PublicKey{key1.PubKey},
}, {
	summary:   "Multiple signatures: no valid public keys",
	clearData: clearSignedWithMultipleSigns,
	relerror:  "cannot verify any signatures",
}, {
	summary:   "Invalid data: improper hash",
	clearData: invalidClearSignedData,
	pubKeys:   []*packet.PublicKey{key1.PubKey},
	relerror:  "openpgp: .*invalid signature: hash tag doesn't match.*",
}, {
	summary:   "Invalid data: bad packets",
	clearData: invalidClearSignedDataBadPackets,
	pubKeys:   []*packet.PublicKey{key1.PubKey},
	relerror:  "cannot parse armored data: openpgp: .*",
}, {
	summary:   "Invalid data: malformed clearsign text",
	clearData: "foo\n",
	pubKeys:   []*packet.PublicKey{key1.PubKey},
	relerror:  "cannot decode clearsign text",
}, {
	summary:   "Wrong public key to verify with",
	clearData: clearSignedData,
	pubKeys:   []*packet.PublicKey{key2.PubKey},
	relerror:  "openpgp: .*invalid signature:.*verification failure",
}}

func (s *S) TestVerifySignature(c *C) {
	for _, test := range verifyClearSignTests {
		c.Logf("Summary: %s", test.summary)

		sigs, body, err := pgputil.DecodeClearSigned([]byte(test.clearData))
		if err == nil {
			err = pgputil.VerifyAnySignature(test.pubKeys, sigs, body)
		}
		if test.relerror != "" {
			c.Assert(err, ErrorMatches, test.relerror)
			continue
		} else {
			c.Assert(err, IsNil)
		}
	}
}

// twoPubKeysArmor contains two public keys:
//   - 854BAF1AA9D76600 ("foo-bar <foo@bar>")
//   - 871920D1991BC93C ("Ubuntu Archive Automatic Signing Key (2018) <ftpmaster@ubuntu.com>")
const twoPubKeysArmor = `
-----BEGIN PGP ARMORED FILE-----

mQENBGVs8P4BCADPh/fNnw2AI1JCYf+3p4jkcFQPLVsUkoTZk8OXjCxy+UP9Jd2m
xnxat7a0JEJZa0aWCmtlSL1XR+kFKBrd7Ry5jOHYjuDKx4kTmDUbezPnjoZIGDNX
j5cdNuMLpOINZweNNWDKRdRvhj5QX89/DYwPrLkNFwwjXjlj5tjU6RUkROYJBGPe
G2ns2cZtVbYMh3FDU9YRfp/hUqGVf+UFRyUw+mo1TUlk5F7fnfwEQmsppDHvfTNJ
yjEMZD7nReTEeMy12GV2wysOwWMPEb2PSE/+Od7AKn5dFA7w3kyLCzAxYp6o7IE/
+RY8YzAJe6GmLwhTWtylMV1xteQhZkEe/QGXABEBAAGZAg0EW5/B2gEQAO/8bHK3
H8txJdi4zQzAqiNsHJ7zWzQbWcTH6XPpUBSkhRmduSp1cEcL53CsZL8J54u4CM9E
tTwCjET+OO9lWnzEUYRYdh2SWpcZn+Al8/l3d8hQG1kdkQmXwHyb2kwd/8BBB2wL
5jOLNIbm3kyGei3DTjgte10QSTHa3onPQ4auH7kijGpfulmKroK/X0GiFpSKgox2
nsRLpFh83uiXodIsWWsxe1V+H+KOk32PdmFUZV5ELyQodCwnk+Qhua/EGJSHtImZ
9lTHQhCE0xoMdd91kAY22eHPM1F5vUWo0tJWStL8+ewBDMyEbUEObZU5IXriN5sp
d98WozktdFBN6pMuyNRtuupHqz8YI7xQXuN9SPoju1ovKCawc78kPiOkpELSBulQ
F9qInIu+56nHeRaiovew3QuGUwjzT58DsZO+g7Hi2mpWXOUTpNqNi76N9bdCk4VL
l7AQx0vbqHPGxmD+B5m9NsCtw/46wkpGaG/iQ2joDJ3IdD/dlX9/df2ZPf9I8tsl
q6aSCndjN3q3k94G75lCT+Y3lY025qKE0RXuWVvVmG9jQXG70FV38E2XSvO7Gnes
iKcHZNfZIKDvATnFeTBe5D/Z5MMTS/QeUae2S5mMajANmTEdlBLFlU7N1kJFVpf9
YQUukprYBCnDlEmtDihn85+J9fInM/bujTfBABEBAAE=
=Znwy
-----END PGP ARMORED FILE-----
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
const invalidClearSignedData = `
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
const invalidClearSignedDataBadPackets = `
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

// invalidPubKeyArmor contains bad packets.
const invalidPubKeyArmor = `
-----BEGIN PGP PUBLIC KEY BLOCK-----

mI0EZXAwcgEEAMBQ4Qx6xam1k1hyjPrKQfCnGRBBm2+Lw9DHQcz0lreH51iZEVkS
=U79/
-----END PGP PUBLIC KEY BLOCK-----
`
