package testutil

var testKey = PGPKeys["key1"]

var DefaultChiselYaml = `
	format: v1
	maintenance:
		standard: 2025-01-01
		end-of-life: 2100-01-01
	archives:
		ubuntu:
			version: 22.04
			components: [main, universe]
			suites: [jammy]
			public-keys: [test-key]
	public-keys:
		test-key:
			id: ` + testKey.ID + `
			armor: |` + "\n" + PrefixEachLine(testKey.PubKeyArmor, "\t\t\t\t\t\t")
