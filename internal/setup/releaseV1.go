package setup

import (
	"bytes"
	"fmt"

	"golang.org/x/crypto/openpgp/packet"
	"gopkg.in/yaml.v3"

	"github.com/canonical/chisel/internal/pgputil"
)

func parseReleaseV1(baseDir, filePath string, data []byte) (*Release, error) {
	type yamlArchive struct {
		Version    string   `yaml:"version"`
		Suites     []string `yaml:"suites"`
		Components []string `yaml:"components"`
		Default    bool     `yaml:"default"`
		PubKeys    []string `yaml:"public-keys"`
	}
	type yamlPubKey struct {
		ID    string `yaml:"id"`
		Armor string `yaml:"armor"`
	}
	type yamlRelease struct {
		Format   string                 `yaml:"format"`
		Archives map[string]yamlArchive `yaml:"archives"`
		PubKeys  map[string]yamlPubKey  `yaml:"public-keys"`
	}
	const yamlReleaseFormat = "v1"

	release := &Release{
		Path:     baseDir,
		Packages: make(map[string]*Package),
		Archives: make(map[string]*Archive),
	}

	fileName := stripBase(baseDir, filePath)

	yamlVar := yamlRelease{}
	dec := yaml.NewDecoder(bytes.NewBuffer(data))
	dec.KnownFields(false)
	err := dec.Decode(&yamlVar)
	if err != nil {
		return nil, fmt.Errorf("%s: cannot parse release definition: %v", fileName, err)
	}
	if yamlVar.Format != yamlReleaseFormat {
		return nil, fmt.Errorf("%s: expected format %q, got %q", fileName, yamlReleaseFormat, yamlVar.Format)
	}
	if len(yamlVar.Archives) == 0 {
		return nil, fmt.Errorf("%s: no archives defined", fileName)
	}

	// Decode the public keys and match against provided IDs.
	pubKeys := make(map[string]*packet.PublicKey, len(yamlVar.PubKeys))
	for keyName, yamlPubKey := range yamlVar.PubKeys {
		key, err := pgputil.DecodePubKey([]byte(yamlPubKey.Armor))
		if err != nil {
			return nil, fmt.Errorf("%s: cannot decode public key %q: %w", fileName, keyName, err)
		}
		if yamlPubKey.ID != key.KeyIdString() {
			return nil, fmt.Errorf("%s: public key %q armor has incorrect ID: expected %q, got %q", fileName, keyName, yamlPubKey.ID, key.KeyIdString())
		}
		pubKeys[keyName] = key
	}

	for archiveName, details := range yamlVar.Archives {
		if details.Version == "" {
			return nil, fmt.Errorf("%s: archive %q missing version field", fileName, archiveName)
		}
		if len(details.Suites) == 0 {
			adjective := ubuntuAdjectives[details.Version]
			if adjective == "" {
				return nil, fmt.Errorf("%s: archive %q missing suites field", fileName, archiveName)
			}
			details.Suites = []string{adjective}
		}
		if len(details.Components) == 0 {
			return nil, fmt.Errorf("%s: archive %q missing components field", fileName, archiveName)
		}
		if len(yamlVar.Archives) == 1 {
			details.Default = true
		} else if details.Default && release.DefaultArchive != "" {
			return nil, fmt.Errorf("%s: more than one default archive: %s, %s", fileName, release.DefaultArchive, archiveName)
		}
		if details.Default {
			release.DefaultArchive = archiveName
		}
		if len(details.PubKeys) == 0 {
			return nil, fmt.Errorf("%s: archive %q missing public-keys field", fileName, archiveName)
		}
		var archiveKeys []*packet.PublicKey
		for _, keyName := range details.PubKeys {
			key, ok := pubKeys[keyName]
			if !ok {
				return nil, fmt.Errorf("%s: archive %q refers to undefined public key %q", fileName, archiveName, keyName)
			}
			archiveKeys = append(archiveKeys, key)
		}
		release.Archives[archiveName] = &Archive{
			Name:       archiveName,
			Version:    details.Version,
			Suites:     details.Suites,
			Components: details.Components,
			PubKeys:    archiveKeys,
		}
	}

	return release, err
}
