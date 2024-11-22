package setup

import (
	"bytes"
	"fmt"
	"path"
	"slices"
	"strings"

	"golang.org/x/crypto/openpgp/packet"
	"gopkg.in/yaml.v3"

	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/deb"
	"github.com/canonical/chisel/internal/pgputil"
)

func (p *Package) MarshalYAML() (interface{}, error) {
	return packageToYAML(p)
}

var _ yaml.Marshaler = (*Package)(nil)

type yamlRelease struct {
	Format   string                 `yaml:"format"`
	Archives map[string]yamlArchive `yaml:"archives"`
	PubKeys  map[string]yamlPubKey  `yaml:"public-keys"`
}

const (
	MaxArchivePriority = 1000
	MinArchivePriority = -1000
)

type yamlArchive struct {
	Version    string   `yaml:"version"`
	Suites     []string `yaml:"suites"`
	Components []string `yaml:"components"`
	Priority   *int     `yaml:"priority"`
	Pro        string   `yaml:"pro"`
	Default    bool     `yaml:"default"`
	PubKeys    []string `yaml:"public-keys"`
}

type yamlPackage struct {
	Name      string               `yaml:"package"`
	Archive   string               `yaml:"archive,omitempty"`
	Essential []string             `yaml:"essential,omitempty"`
	Slices    map[string]yamlSlice `yaml:"slices,omitempty"`
}

type yamlPath struct {
	Dir      bool         `yaml:"make,omitempty"`
	Mode     yamlMode     `yaml:"mode,omitempty"`
	Copy     string       `yaml:"copy,omitempty"`
	Text     *string      `yaml:"text,omitempty"`
	Symlink  string       `yaml:"symlink,omitempty"`
	Mutable  bool         `yaml:"mutable,omitempty"`
	Until    PathUntil    `yaml:"until,omitempty"`
	Arch     yamlArch     `yaml:"arch,omitempty"`
	Generate GenerateKind `yaml:"generate,omitempty"`
}

func (yp *yamlPath) MarshalYAML() (interface{}, error) {
	type flowPath *yamlPath
	node := &yaml.Node{}
	err := node.Encode(flowPath(yp))
	if err != nil {
		return nil, err
	}
	node.Style |= yaml.FlowStyle
	return node, nil
}

var _ yaml.Marshaler = (*yamlPath)(nil)

// SameContent returns whether the path has the same content properties as some
// other path. In other words, the resulting file/dir entry is the same. The
// Mutable flag must also match, as that's a common agreement that the actual
// content is not well defined upfront.
func (yp *yamlPath) SameContent(other *yamlPath) bool {
	return (yp.Dir == other.Dir &&
		yp.Mode == other.Mode &&
		yp.Copy == other.Copy &&
		yp.Text == other.Text &&
		yp.Symlink == other.Symlink &&
		yp.Mutable == other.Mutable)
}

type yamlArch struct {
	List []string
}

func (ya *yamlArch) UnmarshalYAML(value *yaml.Node) error {
	var s string
	var l []string
	if value.Decode(&s) == nil {
		ya.List = []string{s}
	} else if value.Decode(&l) == nil {
		ya.List = l
	} else {
		return fmt.Errorf("cannot decode arch")
	}
	// Validate arch correctness later for a better error message.
	return nil
}

func (ya yamlArch) MarshalYAML() (interface{}, error) {
	if len(ya.List) == 1 {
		return ya.List[0], nil
	}
	return ya.List, nil
}

var _ yaml.Marshaler = yamlArch{}

type yamlMode uint

func (ym yamlMode) MarshalYAML() (interface{}, error) {
	// Workaround for marshalling integers in octal format.
	// Ref: https://github.com/go-yaml/yaml/issues/420.
	node := &yaml.Node{}
	err := node.Encode(uint(ym))
	if err != nil {
		return nil, err
	}
	node.Value = fmt.Sprintf("0%o", ym)
	return node, nil
}

var _ yaml.Marshaler = yamlMode(0)

type yamlSlice struct {
	Essential []string             `yaml:"essential,omitempty"`
	Contents  map[string]*yamlPath `yaml:"contents,omitempty"`
	Mutate    string               `yaml:"mutate,omitempty"`
}

type yamlPubKey struct {
	ID    string `yaml:"id"`
	Armor string `yaml:"armor"`
}

func parseRelease(baseDir, filePath string, data []byte) (*Release, error) {
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
	if yamlVar.Format != "v1" {
		return nil, fmt.Errorf("%s: unknown format %q", fileName, yamlVar.Format)
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

	// For compatibility if there is a default archive set and priorities are
	// not being used, we will revert back to the default archive behaviour.
	hasPriority := false
	var defaultArchive string
	var archiveNoPriority string
	for archiveName, details := range yamlVar.Archives {
		if details.Version == "" {
			return nil, fmt.Errorf("%s: archive %q missing version field", fileName, archiveName)
		}
		if len(details.Suites) == 0 {
			return nil, fmt.Errorf("%s: archive %q missing suites field", fileName, archiveName)
		}
		if len(details.Components) == 0 {
			return nil, fmt.Errorf("%s: archive %q missing components field", fileName, archiveName)
		}
		switch details.Pro {
		case "", archive.ProApps, archive.ProFIPS, archive.ProFIPSUpdates, archive.ProInfra:
		default:
			logf("Archive %q ignored: invalid pro value: %q", archiveName, details.Pro)
			continue
		}
		if details.Default && defaultArchive != "" {
			if archiveName < defaultArchive {
				archiveName, defaultArchive = defaultArchive, archiveName
			}
			return nil, fmt.Errorf("%s: more than one default archive: %s, %s", fileName, defaultArchive, archiveName)
		}
		if details.Default {
			defaultArchive = archiveName
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
		priority := 0
		if details.Priority != nil {
			hasPriority = true
			priority = *details.Priority
			if priority > MaxArchivePriority || priority < MinArchivePriority || priority == 0 {
				return nil, fmt.Errorf("%s: archive %q has invalid priority value of %d", fileName, archiveName, priority)
			}
		} else {
			if archiveNoPriority == "" || archiveName < archiveNoPriority {
				// Make it deterministic.
				archiveNoPriority = archiveName
			}
		}
		release.Archives[archiveName] = &Archive{
			Name:       archiveName,
			Version:    details.Version,
			Suites:     details.Suites,
			Components: details.Components,
			Pro:        details.Pro,
			Priority:   priority,
			PubKeys:    archiveKeys,
		}
	}
	if (hasPriority && archiveNoPriority != "") ||
		(!hasPriority && defaultArchive == "" && len(yamlVar.Archives) > 1) {
		return nil, fmt.Errorf("%s: archive %q is missing the priority setting", fileName, archiveNoPriority)
	}
	if defaultArchive != "" && !hasPriority {
		// For compatibility with the default archive behaviour we will set
		// negative priorities to all but the default one, which means all
		// others will be ignored unless pinned.
		var archiveNames []string
		for archiveName := range yamlVar.Archives {
			archiveNames = append(archiveNames, archiveName)
		}
		// Make it deterministic.
		slices.Sort(archiveNames)
		for i, archiveName := range archiveNames {
			release.Archives[archiveName].Priority = -i - 1
		}
		release.Archives[defaultArchive].Priority = 1
	}

	return release, err
}

func parsePackage(baseDir, pkgName, pkgPath string, data []byte) (*Package, error) {
	pkg := Package{
		Name:   pkgName,
		Path:   pkgPath,
		Slices: make(map[string]*Slice),
	}

	yamlPkg := yamlPackage{}
	dec := yaml.NewDecoder(bytes.NewBuffer(data))
	dec.KnownFields(false)
	err := dec.Decode(&yamlPkg)
	if err != nil {
		return nil, fmt.Errorf("cannot parse package %q slice definitions: %v", pkgName, err)
	}
	if yamlPkg.Name != pkg.Name {
		return nil, fmt.Errorf("%s: filename and 'package' field (%q) disagree", pkgPath, yamlPkg.Name)
	}
	pkg.Archive = yamlPkg.Archive

	zeroPath := yamlPath{}
	for sliceName, yamlSlice := range yamlPkg.Slices {
		match := snameExp.FindStringSubmatch(sliceName)
		if match == nil {
			return nil, fmt.Errorf("invalid slice name %q in %s", sliceName, pkgPath)
		}

		slice := &Slice{
			Package: pkgName,
			Name:    sliceName,
			Scripts: SliceScripts{
				Mutate: yamlSlice.Mutate,
			},
		}
		for _, refName := range yamlPkg.Essential {
			sliceKey, err := ParseSliceKey(refName)
			if err != nil {
				return nil, fmt.Errorf("package %q has invalid essential slice reference: %q", pkgName, refName)
			}
			if sliceKey.Package == slice.Package && sliceKey.Slice == slice.Name {
				// Do not add the slice to its own essentials list.
				continue
			}
			if slices.Contains(slice.Essential, sliceKey) {
				return nil, fmt.Errorf("package %s defined with redundant essential slice: %s", pkgName, refName)
			}
			slice.Essential = append(slice.Essential, sliceKey)
		}
		for _, refName := range yamlSlice.Essential {
			sliceKey, err := ParseSliceKey(refName)
			if err != nil {
				return nil, fmt.Errorf("package %q has invalid essential slice reference: %q", pkgName, refName)
			}
			if sliceKey.Package == slice.Package && sliceKey.Slice == slice.Name {
				return nil, fmt.Errorf("cannot add slice to itself as essential %q in %s", refName, pkgPath)
			}
			if slices.Contains(slice.Essential, sliceKey) {
				return nil, fmt.Errorf("slice %s defined with redundant essential slice: %s", slice, refName)
			}
			slice.Essential = append(slice.Essential, sliceKey)
		}

		if len(yamlSlice.Contents) > 0 {
			slice.Contents = make(map[string]PathInfo, len(yamlSlice.Contents))
		}
		for contPath, yamlPath := range yamlSlice.Contents {
			isDir := strings.HasSuffix(contPath, "/")
			comparePath := contPath
			if isDir {
				comparePath = comparePath[:len(comparePath)-1]
			}
			if !path.IsAbs(contPath) || path.Clean(contPath) != comparePath {
				return nil, fmt.Errorf("slice %s_%s has invalid content path: %s", pkgName, sliceName, contPath)
			}
			var kinds = make([]PathKind, 0, 3)
			var info string
			var mode uint
			var mutable bool
			var until PathUntil
			var arch []string
			var generate GenerateKind
			if yamlPath != nil && yamlPath.Generate != "" {
				zeroPathGenerate := zeroPath
				zeroPathGenerate.Generate = yamlPath.Generate
				if !yamlPath.SameContent(&zeroPathGenerate) || yamlPath.Until != UntilNone {
					return nil, fmt.Errorf("slice %s_%s path %s has invalid generate options",
						pkgName, sliceName, contPath)
				}
				if _, err := validateGeneratePath(contPath); err != nil {
					return nil, fmt.Errorf("slice %s_%s has invalid generate path: %s", pkgName, sliceName, err)
				}
				kinds = append(kinds, GeneratePath)
			} else if strings.ContainsAny(contPath, "*?") {
				if yamlPath != nil {
					if !yamlPath.SameContent(&zeroPath) {
						return nil, fmt.Errorf("slice %s_%s path %s has invalid wildcard options",
							pkgName, sliceName, contPath)
					}
				}
				kinds = append(kinds, GlobPath)
			}
			if yamlPath != nil {
				mode = uint(yamlPath.Mode)
				mutable = yamlPath.Mutable
				generate = yamlPath.Generate
				if yamlPath.Dir {
					if !strings.HasSuffix(contPath, "/") {
						return nil, fmt.Errorf("slice %s_%s path %s must end in / for 'make' to be valid",
							pkgName, sliceName, contPath)
					}
					kinds = append(kinds, DirPath)
				}
				if yamlPath.Text != nil {
					kinds = append(kinds, TextPath)
					info = *yamlPath.Text
				}
				if len(yamlPath.Symlink) > 0 {
					kinds = append(kinds, SymlinkPath)
					info = yamlPath.Symlink
				}
				if len(yamlPath.Copy) > 0 {
					kinds = append(kinds, CopyPath)
					info = yamlPath.Copy
					if info == contPath {
						info = ""
					}
				}
				until = yamlPath.Until
				switch until {
				case UntilNone, UntilMutate:
				default:
					return nil, fmt.Errorf("slice %s_%s has invalid 'until' for path %s: %q", pkgName, sliceName, contPath, until)
				}
				arch = yamlPath.Arch.List
				for _, s := range arch {
					if deb.ValidateArch(s) != nil {
						return nil, fmt.Errorf("slice %s_%s has invalid 'arch' for path %s: %q", pkgName, sliceName, contPath, s)
					}
				}
			}
			if len(kinds) == 0 {
				kinds = append(kinds, CopyPath)
			}
			if len(kinds) != 1 {
				list := make([]string, len(kinds))
				for i, s := range kinds {
					list[i] = string(s)
				}
				return nil, fmt.Errorf("conflict in slice %s_%s definition for path %s: %s", pkgName, sliceName, contPath, strings.Join(list, ", "))
			}
			if mutable && kinds[0] != TextPath && (kinds[0] != CopyPath || isDir) {
				return nil, fmt.Errorf("slice %s_%s mutable is not a regular file: %s", pkgName, sliceName, contPath)
			}
			slice.Contents[contPath] = PathInfo{
				Kind:     kinds[0],
				Info:     info,
				Mode:     mode,
				Mutable:  mutable,
				Until:    until,
				Arch:     arch,
				Generate: generate,
			}
		}

		pkg.Slices[sliceName] = slice
	}

	return &pkg, err
}

// validateGeneratePath validates that the path follows the following format:
//   - /slashed/path/to/dir/**
//
// Wildcard characters can only appear at the end as **, and the path before
// those wildcards must be a directory.
func validateGeneratePath(path string) (string, error) {
	if !strings.HasSuffix(path, "/**") {
		return "", fmt.Errorf("%s does not end with /**", path)
	}
	dirPath := strings.TrimSuffix(path, "**")
	if strings.ContainsAny(dirPath, "*?") {
		return "", fmt.Errorf("%s contains wildcard characters in addition to trailing **", path)
	}
	return dirPath, nil
}

// pathInfoToYAML converts a PathInfo object to a yamlPath object.
// The returned object takes pointers to the given PathInfo object.
func pathInfoToYAML(pi *PathInfo) (*yamlPath, error) {
	path := &yamlPath{
		Mode:     yamlMode(pi.Mode),
		Mutable:  pi.Mutable,
		Until:    pi.Until,
		Arch:     yamlArch{List: pi.Arch},
		Generate: pi.Generate,
	}
	switch pi.Kind {
	case DirPath:
		path.Dir = true
	case CopyPath:
		path.Copy = pi.Info
	case TextPath:
		path.Text = &pi.Info
	case SymlinkPath:
		path.Symlink = pi.Info
	case GlobPath, GeneratePath:
		// Nothing more needs to be done for these types.
	default:
		return nil, fmt.Errorf("internal error: unrecognised PathInfo type: %s", pi.Kind)
	}
	return path, nil
}

// sliceToYAML converts a Slice object to a yamlSlice object.
func sliceToYAML(s *Slice) (*yamlSlice, error) {
	slice := &yamlSlice{
		Essential: make([]string, 0, len(s.Essential)),
		Contents:  make(map[string]*yamlPath, len(s.Contents)),
		Mutate:    s.Scripts.Mutate,
	}
	for _, key := range s.Essential {
		slice.Essential = append(slice.Essential, key.String())
	}
	for path, info := range s.Contents {
		// TODO remove the following line after upgrading to Go 1.22 or higher.
		info := info
		yamlPath, err := pathInfoToYAML(&info)
		if err != nil {
			return nil, err
		}
		slice.Contents[path] = yamlPath
	}
	return slice, nil
}

// packageToYAML converts a Package object to a yamlPackage object.
func packageToYAML(p *Package) (*yamlPackage, error) {
	pkg := &yamlPackage{
		Name:    p.Name,
		Archive: p.Archive,
		Slices:  make(map[string]yamlSlice, len(p.Slices)),
	}
	for name, slice := range p.Slices {
		yamlSlice, err := sliceToYAML(slice)
		if err != nil {
			return nil, err
		}
		pkg.Slices[name] = *yamlSlice
	}
	return pkg, nil
}
