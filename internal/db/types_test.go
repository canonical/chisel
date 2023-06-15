package db_test

import (
	"encoding/json"
	"reflect"

	"github.com/canonical/chisel/internal/db"
	. "gopkg.in/check.v1"
)

type roundTripTestCase struct {
	value any
	json  string
}

var roundTripTestCases = []roundTripTestCase{{
	db.Package{"foo", "bar", "", ""},
	`{"kind":"package","name":"foo","version":"bar","sha256":"","arch":""}`,
}, {
	db.Package{"coolutils", "1.2-beta", "bbbaa816dedc5c5d58e30e7ed600fe62ea0208ef2d6fac4da8b312d113401958", "all"},
	`{"kind":"package","name":"coolutils","version":"1.2-beta","sha256":"bbbaa816dedc5c5d58e30e7ed600fe62ea0208ef2d6fac4da8b312d113401958","arch":"all"}`,
}, {
	db.Package{"badcowboys", "7", "d0aba6d028cd4a3fd153eb5e0bfb35c33f4d5674b80a7a827917df40e1192424", "amd64"},
	`{"kind":"package","name":"badcowboys","version":"7","sha256":"d0aba6d028cd4a3fd153eb5e0bfb35c33f4d5674b80a7a827917df40e1192424","arch":"amd64"}`,
}, {
	db.Slice{"elitestrike_bins"},
	`{"kind":"slice","name":"elitestrike_bins"}`,
}, {
	db.Slice{"invalid but unmarshals"},
	`{"kind":"slice","name":"invalid but unmarshals"}`,
}, {
	db.Path{
		Path:   "/bin/snake",
		Mode:   0755,
		Slices: []string{"snake_bins"},
		SHA256: &[...]byte{
			0xa0, 0x1b, 0xab, 0x26, 0xf0, 0x8b, 0xa8, 0x7b, 0x86,
			0x73, 0x63, 0x68, 0xb0, 0x68, 0x4f, 0x08, 0x49, 0xa3,
			0x65, 0xab, 0x4c, 0x5e, 0xc5, 0x46, 0xd9, 0x73, 0xca,
			0x87, 0xc8, 0x15, 0xf6, 0x82,
		},
		Size: 13,
	},
	`{"kind":"path","path":"/bin/snake","mode":"0755","slices":["snake_bins"],"sha256":"a01bab26f08ba87b86736368b0684f0849a365ab4c5ec546d973ca87c815f682","size":13}`,
}, {
	db.Path{
		Path:   "/etc/default/",
		Mode:   0750,
		Slices: []string{"someconfig_data", "mytoo_data"},
	},
	`{"kind":"path","path":"/etc/default/","mode":"0750","slices":["someconfig_data","mytoo_data"]}`,
}, {
	db.Path{
		Path:   "/var/lib/matt/index.data",
		Mode:   0600,
		Slices: []string{"daemon_data"},
		SHA256: &[...]byte{
			0x06, 0x82, 0xc5, 0xf2, 0x07, 0x6f, 0x09, 0x9c, 0x34,
			0xcf, 0xdd, 0x15, 0xa9, 0xe0, 0x63, 0x84, 0x9e, 0xd4,
			0x37, 0xa4, 0x96, 0x77, 0xe6, 0xfc, 0xc5, 0xb4, 0x19,
			0x8c, 0x76, 0x57, 0x5b, 0xe5,
		},
		FinalSHA256: &[...]byte{
			0xd7, 0xd5, 0xdc, 0xc3, 0x69, 0x42, 0x6e, 0x2e, 0x5f,
			0x8d, 0xcb, 0x89, 0xaf, 0x43, 0x08, 0xb0, 0xda, 0xed,
			0x6e, 0x55, 0x91, 0x0d, 0x53, 0x39, 0x5c, 0xe3, 0x8b,
			0xd6, 0xdd, 0x1a, 0x94, 0x56,
		},
		Size: 999,
	},
	`{"kind":"path","path":"/var/lib/matt/index.data","mode":"0600","slices":["daemon_data"],"sha256":"0682c5f2076f099c34cfdd15a9e063849ed437a49677e6fcc5b4198c76575be5","final_sha256":"d7d5dcc369426e2e5f8dcb89af4308b0daed6e55910d53395ce38bd6dd1a9456","size":999}`,
}, {
	db.Path{
		Path:   "/etc/config",
		Mode:   0644,
		SHA256: &[32]byte{},
	},
	`{"kind":"path","path":"/etc/config","mode":"0644","slices":[],"sha256":"0000000000000000000000000000000000000000000000000000000000000000","size":0}`,
}, {
	db.Path{
		Path:   "/lib",
		Mode:   0777,
		Slices: []string{"libc6_libs", "zlib1g_libs"},
		Link:   "/usr/lib/",
	},
	`{"kind":"path","path":"/lib","mode":"0777","slices":["libc6_libs","zlib1g_libs"],"link":"/usr/lib/"}`,
}, {
	db.Path{},
	`{"kind":"path","path":"","mode":"0","slices":[]}`,
}, {
	db.Path{Mode: 077777},
	`{"kind":"path","path":"","mode":"077777","slices":[]}`,
}, {
	db.Content{"foo_sl", "/a/b/c"},
	`{"kind":"content","slice":"foo_sl","path":"/a/b/c"}`,
}}

func (s *S) TestMarshalUnmarshalRoundTrip(c *C) {
	for i, test := range roundTripTestCases {
		c.Logf("Test #%d", i)
		data, err := json.Marshal(test.value)
		c.Assert(err, IsNil)
		c.Assert(string(data), DeepEquals, test.json)
		ptrOut := reflect.New(reflect.ValueOf(test.value).Type())
		err = json.Unmarshal(data, ptrOut.Interface())
		c.Assert(err, IsNil)
		c.Assert(ptrOut.Elem().Interface(), DeepEquals, test.value)
	}
}

type unmarshalTestCase struct {
	json  string
	value any
	error string
}

var unmarshalTestCases = []unmarshalTestCase{{
	json:  `{"kind":"package","name":"pkg","version":"1.1","sha256":"d0aba6d028cd4a3fd153eb5e0bfb35c33f4d5674b80a7a827917df40e1192424","arch":"all"}`,
	value: db.Package{"pkg", "1.1", "d0aba6d028cd4a3fd153eb5e0bfb35c33f4d5674b80a7a827917df40e1192424", "all"},
}, {
	json:  `{"kind":"slice","name":"a"}`,
	value: db.Slice{"a"},
}, {
	json: `{"kind":"path","path":"/x/y/z","mode":"0644","slices":["pkg1_data","pkg2_data"],"sha256":"f177b37f18f5bc6596878f074721d796c2333d95f26ce1e45c5a096c350a1c07","final_sha256":"61bd495076999a77f75288fcfcdd76073ec4aa114632a310b3b3263c498e12f7","size":123}`,
	value: db.Path{
		Path:   "/x/y/z",
		Mode:   0644,
		Slices: []string{"pkg1_data", "pkg2_data"},
		SHA256: &[...]byte{
			0xf1, 0x77, 0xb3, 0x7f, 0x18, 0xf5, 0xbc, 0x65, 0x96,
			0x87, 0x8f, 0x07, 0x47, 0x21, 0xd7, 0x96, 0xc2, 0x33,
			0x3d, 0x95, 0xf2, 0x6c, 0xe1, 0xe4, 0x5c, 0x5a, 0x09,
			0x6c, 0x35, 0x0a, 0x1c, 0x07,
		},
		FinalSHA256: &[...]byte{
			0x61, 0xbd, 0x49, 0x50, 0x76, 0x99, 0x9a, 0x77, 0xf7,
			0x52, 0x88, 0xfc, 0xfc, 0xdd, 0x76, 0x07, 0x3e, 0xc4,
			0xaa, 0x11, 0x46, 0x32, 0xa3, 0x10, 0xb3, 0xb3, 0x26,
			0x3c, 0x49, 0x8e, 0x12, 0xf7,
		},
		Size: 123,
	},
}, {
	json:  `{"kind":"path","path":"/x/y/z","mode":"0777","slices":[],"link":"/home"}`,
	value: db.Path{Path: "/x/y/z", Mode: 0777, Link: "/home"},
}, {
	json:  `{"kind":"path","path":"/x/y/z","mode":"0","slices":null}`,
	value: db.Path{Path: "/x/y/z"},
}, {
	json:  `{"kind":"path","path":"/x/y/z","mode":"0"}`,
	value: db.Path{Path: "/x/y/z"},
}, {
	json:  `{"kind":"content","slice":"pkg_sl","path":"/a/b/c"}`,
	value: db.Content{Slice: "pkg_sl", Path: "/a/b/c"},
}, {
	json:  `{"kind":"path","path":"","mode":"90909"}`,
	value: db.Path{},
	error: `invalid mode "90909": strconv.ParseUint: parsing "90909": invalid syntax`,
}, {
	json:  `{"kind":"path","path":"","mode":"077777777777"}`,
	value: db.Path{},
	error: `invalid mode "077777777777": strconv.ParseUint: parsing "077777777777": value out of range`,
}, {
	json:  `{"kind":"path","path":"/tmp/abc.txt","mode":"0644","sha256":"too short"}`,
	value: db.Path{},
	error: `invalid sha256 "too short": length 9 != 64`,
}, {
	json:  `{"kind":"path","path":"/tmp/abc.txt","mode":"0644","sha256":"gfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"}`,
	value: db.Path{},
	error: `invalid sha256 "gfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff": encoding/hex: invalid byte: U\+0067 'g'`,
}, {
	json:  `{"kind":"package","name":"foo_libs"}`,
	value: db.Slice{},
	error: `invalid kind "package": must be "slice"`,
}}

func (s *S) TestUnmarshal(c *C) {
	for i, test := range unmarshalTestCases {
		c.Logf("Test #%d", i)
		ptrOut := reflect.New(reflect.ValueOf(test.value).Type())
		err := json.Unmarshal([]byte(test.json), ptrOut.Interface())
		if test.error != "" {
			c.Assert(err, ErrorMatches, test.error)
		} else {
			c.Assert(ptrOut.Elem().Interface(), DeepEquals, test.value)
		}
	}
}
