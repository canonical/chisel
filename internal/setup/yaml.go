package setup

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"golang.org/x/crypto/openpgp/packet"
	"gopkg.in/yaml.v3"

	"github.com/canonical/chisel/internal/apacheutil"
	"github.com/canonical/chisel/internal/archive"
	"github.com/canonical/chisel/internal/deb"
	"github.com/canonical/chisel/internal/pgputil"
)

func (p *Package) MarshalYAML() (any, error) {
	return packageToYAML(p)
}

var _ yaml.Marshaler = (*Package)(nil)

type yamlRelease struct {
	Format      string                 `yaml:"format"`
	Maintenance yamlMaintenance        `yaml:"maintenance"`
	Archives    map[string]yamlArchive `yaml:"archives"`
	PubKeys     map[string]yamlPubKey  `yaml:"public-keys"`
	// "v2-archives" is used for backwards compatibility with Chisel <= 1.0.0,
	// where it will be ignored. In new versions, it will be parsed with the new
	// fields that break said compatibility (e.g. "pro" archives) and merged
	// together with "archives".
	V2Archives map[string]yamlArchive `yaml:"v2-archives"`
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
	Name    string `yaml:"package"`
	Archive string `yaml:"archive,omitempty"`
	// For backwards-compatibility reasons with v1 and v2, essential needs
	// custom logic to be parsed. See [yamlEssentialListMap].
	Essential yamlEssentialListMap `yaml:"essential,omitempty"`
	Slices    map[string]yamlSlice `yaml:"slices,omitempty"`
	// "v3-essential" is used for backwards porting of arch-specific essential
	// to releases that use "v1" or "v2". When using older versions of Chisel
	// the field will be ignored and `essential` is used as a fallback.
	V3Essential map[string]*yamlEssential `yaml:"v3-essential,omitempty"`
}

type yamlEssentialListMap struct {
	Values map[string]*yamlEssential
	// isList is set to true when the marshaler found a list and false if it
	// found a map. The former is only valid in format "v1" and "v2" while the
	// latter is valid from "v3" onwards.
	isList bool
}

func (es *yamlEssentialListMap) UnmarshalYAML(value *yaml.Node) error {
	m := map[string]*yamlEssential{}
	switch value.Kind {
	case yaml.SequenceNode:
		es.isList = true
		l := []string{}
		err := value.Decode(&l)
		if err != nil {
			return err
		}
		for _, sliceName := range l {
			if _, ok := m[sliceName]; ok {
				return fmt.Errorf("repeats %s in essential fields", sliceName)
			}
			m[sliceName] = &yamlEssential{}
		}
	case yaml.MappingNode:
		es.isList = false
		err := value.Decode(&m)
		if err != nil {
			return err
		}
	}
	es.Values = m
	return nil
}

func (es yamlEssentialListMap) MarshalYAML() (any, error) {
	return es.Values, nil
}

var _ yaml.Marshaler = yamlEssentialListMap{}
var _ yaml.Unmarshaler = (*yamlEssentialListMap)(nil)

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
	Prefer   string       `yaml:"prefer,omitempty"`
}

func (yp *yamlPath) MarshalYAML() (any, error) {
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
		yp.Mutable == other.Mutable &&
		yp.Generate == other.Generate)
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

func (ya yamlArch) MarshalYAML() (any, error) {
	if len(ya.List) == 1 {
		return ya.List[0], nil
	}
	return ya.List, nil
}

var _ yaml.Marshaler = yamlArch{}

type yamlMode uint

func (ym yamlMode) MarshalYAML() (any, error) {
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
	// For backwards-compatibility reasons with v1 and v2, essential needs
	// custom logic to be parsed. See [yamlEssentialListMap].
	Essential yamlEssentialListMap `yaml:"essential,omitempty"`
	Contents  map[string]*yamlPath `yaml:"contents,omitempty"`
	Mutate    string               `yaml:"mutate,omitempty"`
	// "v3-essential" is used for backwards porting of arch-specific essential
	// to releases that use "v1" or "v2". When using older versions of Chisel
	// the field will be ignored and `essential` is used as a fallback.
	V3Essential map[string]*yamlEssential `yaml:"v3-essential,omitempty"`
}

type yamlPubKey struct {
	ID    string `yaml:"id"`
	Armor string `yaml:"armor"`
}

type yamlEssential struct {
	Arch yamlArch `yaml:"arch,omitempty"`
}

func (ye *yamlEssential) MarshalYAML() (any, error) {
	type flowEssential *yamlEssential
	node := &yaml.Node{}
	err := node.Encode(flowEssential(ye))
	if err != nil {
		return nil, err
	}
	node.Style |= yaml.FlowStyle
	return node, nil
}

var _ yaml.Marshaler = (*yamlEssential)(nil)

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
	if yamlVar.Format != "v1" && yamlVar.Format != "v2" && yamlVar.Format != "v3" {
		return nil, fmt.Errorf("%s: unknown format %q", fileName, yamlVar.Format)
	}
	release.Format = yamlVar.Format

	if yamlVar.Format != "v1" && len(yamlVar.V2Archives) > 0 {
		return nil, fmt.Errorf("%s: v2-archives is obsolete since format v2", fileName)
	}
	if len(yamlVar.Archives)+len(yamlVar.V2Archives) == 0 {
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

	// Merge all archive definitions.
	yamlArchives := make(map[string]yamlArchive, len(yamlVar.Archives)+len(yamlVar.V2Archives))
	for archiveName, details := range yamlVar.Archives {
		yamlArchives[archiveName] = details
	}
	for archiveName, details := range yamlVar.V2Archives {
		if _, ok := yamlArchives[archiveName]; ok {
			return nil, fmt.Errorf("%s: archive %q defined twice", fileName, archiveName)
		}
		yamlArchives[archiveName] = details
	}

	// For compatibility if there is a default archive set and priorities are
	// not being used, we will revert back to the default archive behaviour.
	hasPriority := false
	var defaultArchive string
	var archiveNoPriority string
	for archiveName, details := range yamlArchives {
		if yamlVar.Format != "v1" && details.Default {
			return nil, fmt.Errorf("%s: archive %q has 'default' field which is obsolete since format v2", fileName, archiveName)
		}
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
		(!hasPriority && defaultArchive == "" && len(yamlArchives) > 1) {
		return nil, fmt.Errorf("%s: archive %q is missing the priority setting", fileName, archiveNoPriority)
	}
	if defaultArchive != "" && !hasPriority {
		// For compatibility with the default archive behaviour we will set
		// negative priorities to all but the default one, which means all
		// others will be ignored unless pinned.
		var archiveNames []string
		for archiveName := range yamlArchives {
			archiveNames = append(archiveNames, archiveName)
		}
		// Make it deterministic.
		slices.Sort(archiveNames)
		for i, archiveName := range archiveNames {
			release.Archives[archiveName].Priority = -i - 1
		}
		release.Archives[defaultArchive].Priority = 1
	}

	var maintenance Maintenance
	if yamlVar.Maintenance == (yamlMaintenance{}) {
		// Use default if key not present in yaml, best effort if "ubuntu"
		// archive is present.
		// TODO remove the defaults some time after chisel-releases is updated.
		ubuntuArchive, ok := release.Archives["ubuntu"]
		if ok {
			maintenance = defaultMaintenance[ubuntuArchive.Version]
		}
	}
	if maintenance == (Maintenance{}) {
		maintenance, err = parseYamlMaintenance(&yamlVar.Maintenance)
		if err != nil {
			return nil, fmt.Errorf("%s: cannot parse maintenance: %s", fileName, err)
		}
	}
	release.Maintenance = &maintenance
	for archiveName, details := range release.Archives {
		oldRelease := false
		maintained := true
		switch details.Pro {
		case "":
			// The standard archive is no longer maintained during Expanded
			// Security Maintenance for LTS, or after End of Life for interim
			// releases.
			if release.Maintenance.Expanded != (time.Time{}) {
				maintained = time.Now().Before(release.Maintenance.Expanded)
			} else {
				maintained = time.Now().Before(release.Maintenance.EndOfLife)
			}
			oldRelease = time.Now().After(release.Maintenance.EndOfLife)
		case archive.ProInfra, archive.ProApps:
			// Legacy support requires a different subscription and a different
			// archive.
			if release.Maintenance.Legacy != (time.Time{}) {
				maintained = time.Now().Before(release.Maintenance.Legacy)
			} else {
				maintained = time.Now().Before(release.Maintenance.EndOfLife)
			}
		default:
			// FIPS archives are not included in the support window, they need
			// a different subscription and have a different lifetime.
			maintained = time.Now().Before(release.Maintenance.EndOfLife)
		}
		details.Maintained = maintained
		details.OldRelease = oldRelease
		release.Archives[archiveName] = details
	}

	return release, err
}

func parsePackage(format, pkgName, pkgPath string, data []byte) (*Package, error) {
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

	if format == "v1" || format == "v2" {
		if len(yamlPkg.Essential.Values) > 0 && !yamlPkg.Essential.isList {
			return nil, fmt.Errorf("cannot parse package %q: essential expects a list", pkgName)
		}
		for sliceName, yamlSlice := range yamlPkg.Slices {
			if len(yamlSlice.Essential.Values) > 0 && !yamlSlice.Essential.isList {
				return nil, fmt.Errorf("cannot parse slice %s: essential expects a list", SliceKey{pkgName, sliceName})
			}
		}
	} else {
		if yamlPkg.V3Essential != nil {
			return nil, fmt.Errorf("cannot parse package %q: v3-essential is obsolete since format v3", pkgName)
		}
		if len(yamlPkg.Essential.Values) > 0 && yamlPkg.Essential.isList {
			return nil, fmt.Errorf("cannot parse package %q: essential expects a map", pkgName)
		}
		for sliceName, yamlSlice := range yamlPkg.Slices {
			if yamlSlice.V3Essential != nil {
				return nil, fmt.Errorf("cannot parse slice %s: v3-essential is obsolete since format v3", SliceKey{pkgName, sliceName})
			}
			if len(yamlSlice.Essential.Values) > 0 && yamlSlice.Essential.isList {
				return nil, fmt.Errorf("cannot parse slice %s: essential expects a map", SliceKey{pkgName, sliceName})
			}
		}
	}

	pkg.Archive = yamlPkg.Archive
	zeroPath := yamlPath{}
	for sliceName, yamlSlice := range yamlPkg.Slices {
		match := apacheutil.SnameExp.FindStringSubmatch(sliceName)
		if match == nil {
			return nil, fmt.Errorf("invalid slice name %q in %s (start with a-z, len >= 3, only a-z / 0-9 / -)", sliceName, pkgPath)
		}
		slice := &Slice{
			Package: pkgName,
			Name:    sliceName,
			Scripts: SliceScripts{
				Mutate: yamlSlice.Mutate,
			},
		}
		err := parseEssentials(&yamlPkg, &yamlSlice, pkgPath, slice)
		if err != nil {
			return nil, err
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
			var prefer string
			if yamlPath != nil && yamlPath.Generate != "" {
				zeroPathGenerate := zeroPath
				zeroPathGenerate.Generate = yamlPath.Generate
				if !yamlPath.SameContent(&zeroPathGenerate) || yamlPath.Prefer != "" || yamlPath.Until != UntilNone {
					return nil, fmt.Errorf("slice %s_%s path %s has invalid generate options",
						pkgName, sliceName, contPath)
				}
				if _, err := validateGeneratePath(contPath); err != nil {
					return nil, fmt.Errorf("slice %s_%s has invalid generate path: %s", pkgName, sliceName, err)
				}
				kinds = append(kinds, GeneratePath)
			} else if strings.ContainsAny(contPath, "*?") {
				if yamlPath != nil {
					if !yamlPath.SameContent(&zeroPath) || yamlPath.Prefer != "" {
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
				prefer = yamlPath.Prefer
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
			if prefer == pkgName {
				return nil, fmt.Errorf("slice %s_%s cannot 'prefer' its own package for path %s", pkgName, sliceName, contPath)
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
				Prefer:   prefer,
			}
		}

		pkg.Slices[sliceName] = slice
	}

	return &pkg, nil
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
		Prefer:   pi.Prefer,
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
		Contents: make(map[string]*yamlPath, len(s.Contents)),
		Mutate:   s.Scripts.Mutate,
		Essential: yamlEssentialListMap{
			Values: make(map[string]*yamlEssential, len(s.Essential)),
		},
	}
	for key, info := range s.Essential {
		slice.Essential.Values[key.String()] = &yamlEssential{Arch: yamlArch{info.Arch}}
	}
	for path, info := range s.Contents {
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

type yamlMaintenance struct {
	Standard  string `yaml:"standard"`
	Expanded  string `yaml:"expanded"`
	Legacy    string `yaml:"legacy"`
	EndOfLife string `yaml:"end-of-life"`
}

func parseYamlMaintenance(yamlVar *yamlMaintenance) (Maintenance, error) {
	maintenance := Maintenance{}

	if yamlVar.Standard == "" {
		return Maintenance{}, errors.New(`"standard" is unset`)
	}
	date, err := time.Parse(time.DateOnly, yamlVar.Standard)
	if err != nil {
		return Maintenance{}, errors.New(`expected format for "standard" is YYYY-MM-DD`)
	}
	maintenance.Standard = date

	if yamlVar.EndOfLife == "" {
		return Maintenance{}, errors.New(`"end-of-life" is unset`)
	}
	date, err = time.Parse(time.DateOnly, yamlVar.EndOfLife)
	if err != nil {
		return Maintenance{}, errors.New(`expected format for "end-of-life" is YYYY-MM-DD`)
	}
	maintenance.EndOfLife = date

	if yamlVar.Expanded != "" {
		date, err = time.Parse(time.DateOnly, yamlVar.Expanded)
		if err != nil {
			return Maintenance{}, errors.New(`expected format for "expanded" is YYYY-MM-DD`)
		}
		maintenance.Expanded = date
	}

	if yamlVar.Legacy != "" {
		date, err = time.Parse(time.DateOnly, yamlVar.Legacy)
		if err != nil {
			return Maintenance{}, errors.New(`expected format for "legacy" is YYYY-MM-DD`)
		}
		maintenance.Legacy = date
	}

	return maintenance, nil
}

var defaultMaintenance = map[string]Maintenance{
	"20.04": {
		Standard:  time.Date(2020, time.April, 23, 0, 0, 0, 0, time.UTC),
		Expanded:  time.Date(2025, time.May, 29, 0, 0, 0, 0, time.UTC),
		Legacy:    time.Date(2030, time.April, 23, 0, 0, 0, 0, time.UTC),
		EndOfLife: time.Date(2032, time.April, 27, 0, 0, 0, 0, time.UTC),
	},
	"22.04": {
		Standard:  time.Date(2022, time.April, 21, 0, 0, 0, 0, time.UTC),
		Expanded:  time.Date(2027, time.June, 1, 0, 0, 0, 0, time.UTC),
		Legacy:    time.Date(2032, time.April, 21, 0, 0, 0, 0, time.UTC),
		EndOfLife: time.Date(2034, time.April, 25, 0, 0, 0, 0, time.UTC),
	},
	"22.10": {
		Standard:  time.Date(2022, time.October, 20, 0, 0, 0, 0, time.UTC),
		EndOfLife: time.Date(2023, time.July, 20, 0, 0, 0, 0, time.UTC),
	},
	"23.04": {
		Standard:  time.Date(2023, time.April, 20, 0, 0, 0, 0, time.UTC),
		EndOfLife: time.Date(2024, time.January, 25, 0, 0, 0, 0, time.UTC),
	},
	"23.10": {
		Standard:  time.Date(2023, time.October, 12, 0, 0, 0, 0, time.UTC),
		EndOfLife: time.Date(2024, time.July, 11, 0, 0, 0, 0, time.UTC),
	},
	"24.04": {
		Standard:  time.Date(2024, time.April, 25, 0, 0, 0, 0, time.UTC),
		Expanded:  time.Date(2029, time.May, 31, 0, 0, 0, 0, time.UTC),
		Legacy:    time.Date(2034, time.April, 25, 0, 0, 0, 0, time.UTC),
		EndOfLife: time.Date(2036, time.April, 29, 0, 0, 0, 0, time.UTC),
	},
	"24.10": {
		Standard:  time.Date(2024, time.October, 10, 0, 0, 0, 0, time.UTC),
		EndOfLife: time.Date(2025, time.July, 10, 0, 0, 0, 0, time.UTC),
	},
	"25.04": {
		Standard:  time.Date(2025, time.April, 17, 0, 0, 0, 0, time.UTC),
		EndOfLife: time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC),
	},
}

// parseEssentials takes into account package-level and slice-level essentials,
// processes them to check they are valid and not duplicated and, if
// successful, adds them to slice.
func parseEssentials(yamlPkg *yamlPackage, yamlSlice *yamlSlice, pkgPath string, slice *Slice) error {
	addPackageEssential := func(refName string, essentialInfo *yamlEssential) error {
		sliceKey, err := ParseSliceKey(refName)
		if err != nil {
			return fmt.Errorf("package %q has invalid essential slice reference: %q", yamlPkg.Name, refName)
		}
		if sliceKey.Package == slice.Package && sliceKey.Slice == slice.Name {
			// Do not add the slice to its own essentials list.
			return nil
		}
		if _, ok := slice.Essential[sliceKey]; ok {
			return fmt.Errorf("package %q repeats %s in essential fields", yamlPkg.Name, refName)
		}
		if slice.Essential == nil {
			slice.Essential = map[SliceKey]EssentialInfo{}
		}
		var archList []string
		if essentialInfo != nil {
			archList = essentialInfo.Arch.List
		}
		slice.Essential[sliceKey] = EssentialInfo{Arch: archList}
		return nil
	}
	addSliceEssential := func(refName string, essentialInfo *yamlEssential) error {
		sliceKey, err := ParseSliceKey(refName)
		if err != nil {
			return fmt.Errorf("package %q has invalid essential slice reference: %q", yamlPkg.Name, refName)
		}
		if sliceKey.Package == slice.Package && sliceKey.Slice == slice.Name {
			return fmt.Errorf("cannot add slice to itself as essential %s in %s", refName, pkgPath)
		}
		if _, ok := slice.Essential[sliceKey]; ok {
			return fmt.Errorf("slice %s repeats %s in essential fields", slice, refName)
		}
		if slice.Essential == nil {
			slice.Essential = map[SliceKey]EssentialInfo{}
		}
		var archList []string
		if essentialInfo != nil {
			archList = essentialInfo.Arch.List
		}
		slice.Essential[sliceKey] = EssentialInfo{Arch: archList}
		return nil
	}

	for refName, essentialInfo := range yamlPkg.Essential.Values {
		err := addPackageEssential(refName, essentialInfo)
		if err != nil {
			return err
		}
	}
	for refName, essentialInfo := range yamlPkg.V3Essential {
		err := addPackageEssential(refName, essentialInfo)
		if err != nil {
			return err
		}
	}
	for refName, essentialInfo := range yamlSlice.Essential.Values {
		err := addSliceEssential(refName, essentialInfo)
		if err != nil {
			return err
		}
	}
	for refName, essentialInfo := range yamlSlice.V3Essential {
		err := addSliceEssential(refName, essentialInfo)
		if err != nil {
			return err
		}
	}
	return nil
}
