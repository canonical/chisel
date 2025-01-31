package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/crypto/openpgp/packet"

	"github.com/canonical/chisel/internal/apacheutil"
	"github.com/canonical/chisel/internal/strdist"
)

// Release is a collection of package slices targeting a particular
// distribution version.
type Release struct {
	Path     string
	Packages map[string]*Package
	Archives map[string]*Archive

	// pathOrdering stores the sorted packages if there is a 'prefer'
	// relationship. Otherwise, it will be nil.
	// Given a selection of packages, the path should be extracted from the one
	// that is found first on the list.
	pathOrdering map[string][]string
}

// Archive is the location from which binary packages are obtained.
type Archive struct {
	Name       string
	Version    string
	Suites     []string
	Components []string
	Priority   int
	Pro        string
	PubKeys    []*packet.PublicKey
}

// Package holds a collection of slices that represent parts of themselves.
type Package struct {
	Name    string
	Path    string
	Archive string
	Slices  map[string]*Slice
}

// Slice holds the details about a package slice.
type Slice struct {
	Package   string
	Name      string
	Essential []SliceKey
	Contents  map[string]PathInfo
	Scripts   SliceScripts
}

type SliceScripts struct {
	Mutate string
}

type PathKind string

const (
	DirPath      PathKind = "dir"
	CopyPath     PathKind = "copy"
	GlobPath     PathKind = "glob"
	TextPath     PathKind = "text"
	SymlinkPath  PathKind = "symlink"
	GeneratePath PathKind = "generate"

	// TODO Maybe in the future, for binary support.
	//Base64Path PathKind = "base64"
)

type PathUntil string

const (
	UntilNone   PathUntil = ""
	UntilMutate PathUntil = "mutate"
)

type GenerateKind string

const (
	GenerateNone     GenerateKind = ""
	GenerateManifest GenerateKind = "manifest"
)

type PathInfo struct {
	Kind PathKind
	Info string
	Mode uint

	Mutable  bool
	Until    PathUntil
	Arch     []string
	Generate GenerateKind
	Prefer   string
}

// SameContent returns whether the path has the same content properties as some
// other path. In other words, the resulting file/dir entry is the same. The
// Mutable flag must also match, as that's a common agreement that the actual
// content is not well defined upfront.
func (pi *PathInfo) SameContent(other *PathInfo) bool {
	return (pi.Kind == other.Kind &&
		pi.Info == other.Info &&
		pi.Mode == other.Mode &&
		pi.Mutable == other.Mutable &&
		pi.Generate == other.Generate)
}

type SliceKey = apacheutil.SliceKey

func ParseSliceKey(sliceKey string) (SliceKey, error) {
	return apacheutil.ParseSliceKey(sliceKey)
}

func (s *Slice) String() string { return s.Package + "_" + s.Name }

// Selection holds the required configuration to create a Build for a selection
// of slices from a Release. It's still an abstract proposal in the sense that
// the real information coming from packages is still unknown, so referenced
// paths could potentially be missing, for example.
type Selection struct {
	Release             *Release
	Slices              []*Slice
	cachedSelectPackage map[string]string
}

// SelectPackage returns true if path should be extracted from pkg.
func (s *Selection) SelectPackage(path, pkg string) bool {
	// If the path has no prefer relationships then it is always selected.
	ordering, ok := s.Release.pathOrdering[path]
	if !ok {
		return true
	}

	if cached, ok := s.cachedSelectPackage[path]; ok {
		return cached == pkg
	}

	var selected string
	for _, pkg := range ordering {
		i := slices.IndexFunc(s.Slices, func(s *Slice) bool {
			return s.Package == pkg
		})
		if i != -1 {
			selected = s.Slices[i].Package
			break
		}
	}
	s.cachedSelectPackage[path] = selected
	return selected == pkg
}

func ReadRelease(dir string) (*Release, error) {
	logDir := dir
	if strings.Contains(dir, "/.cache/") {
		logDir = filepath.Base(dir)
	}
	logf("Processing %s release...", logDir)

	release, err := readRelease(dir)
	if err != nil {
		return nil, err
	}

	err = release.validate()
	if err != nil {
		return nil, err
	}
	return release, nil
}

func checkConflict(path string, old, new *Slice) error {
	oldInfo := old.Contents[path]
	newInfo := new.Contents[path]
	if oldInfo.Prefer != newInfo.Prefer || !newInfo.SameContent(&oldInfo) ||
		((newInfo.Kind == CopyPath || newInfo.Kind == GlobPath) && new.Package != old.Package) {
		if old.Package > new.Package || old.Package == new.Package && old.Name > new.Name {
			old, new = new, old
		}
		return fmt.Errorf("slices %s and %s conflict on %s", old, new, path)
	}
	return nil
}

func (r *Release) validate() error {
	keys := []SliceKey(nil)

	// Check for info conflicts and prepare for following checks. A conflict
	// means that two slices attempt to extract different files or directories
	// to the same location.
	// Conflict validation is done without downloading packages which means that
	// if we are extracting content from different packages to the same location
	// we cannot be sure that it will be the same. On the contrary, content
	// extracted from the same package will never conflict because it is
	// guaranteed to be the same.
	// The above also means that generated content (e.g. text files, directories
	// with make:true) will always conflict with extracted content, because we
	// cannot validate that they are the same without downloading the package.
	globs := make(map[string]*Slice)
	paths := make(map[string]*Slice)
	// nodes is used for bookkeeping to find the slice for a path without
	// having to do further processing.
	nodes := make(map[string]*Slice)
	successors := make(map[string][]string)
	const (
		// no means none of the nodes have prefers.
		no int = 0
		// yes means all the nodes but one (the tail) have prefers. We have
		// already found the tail.
		yes int = 1
		// maybeTail means we have found a single node with no prefers, it
		// could either be the tail or belong to a graph with no prefers.
		maybeTail int = 2
		// yesMissingTail means all the existing nodes have prefers and we
		// are yet to find a single node without prefers (the tail).
		yesMissingTail int = 3
	)
	hasPrefers := make(map[string]int)

	// Iterate on a stable package order.
	var pkgNames []string
	for _, pkg := range r.Packages {
		pkgNames = append(pkgNames, pkg.Name)
	}
	slices.Sort(pkgNames)
	for _, pkgName := range pkgNames {
		pkg := r.Packages[pkgName]
		for _, new := range pkg.Slices {
			keys = append(keys, SliceKey{pkg.Name, new.Name})
			for newPath, newInfo := range new.Contents {
				old, ok := paths[newPath]
				if !ok {
					paths[newPath] = new
					if newInfo.Kind == GeneratePath || newInfo.Kind == GlobPath {
						globs[newPath] = new
					}
					if newInfo.Prefer != "" {
						if _, ok := r.Packages[newInfo.Prefer]; !ok {
							return fmt.Errorf("slice %s path %s 'prefer' refers to undefined package %q",
								new, newPath, newInfo.Prefer)
						}
						key := preferKey(newPath, new.Package)
						successors[key] = append(successors[key], preferKey(newPath, newInfo.Prefer))
						nodes[key] = new
						hasPrefers[newPath] = yesMissingTail
					} else {
						nodes[preferKey(newPath, new.Package)] = new
						hasPrefers[newPath] = maybeTail
					}
					continue
				}

				prevSamePkg, ok := nodes[preferKey(newPath, new.Package)]
				if ok {
					// If the package was already visited we only need to check
					// that the new path provides the same content as the
					// recorded one and they have the same prefer relationship.
					err := checkConflict(newPath, prevSamePkg, new)
					if err != nil {
						return err
					}
					continue
				}

				if newInfo.Prefer != "" {
					if _, ok := r.Packages[newInfo.Prefer]; !ok {
						return fmt.Errorf("slice %s path %s 'prefer' refers to undefined package %q",
							new, newPath, newInfo.Prefer)
					}
					switch hasPrefers[newPath] {
					case no:
						return fmt.Errorf("slice %s creates an invalid prefer ordering for path %s",
							new, newPath)
					case yesMissingTail:
						// No action needed.
					default:
						hasPrefers[newPath] = yes
					}
					key := preferKey(newPath, new.Package)
					successors[key] = append(successors[key], preferKey(newPath, newInfo.Prefer))
					nodes[key] = new
				} else {
					switch hasPrefers[newPath] {
					case yes:
						return fmt.Errorf("slice %s creates an invalid prefer ordering for path %s",
							new, newPath)
					case yesMissingTail:
						nodes[preferKey(newPath, new.Package)] = new
						hasPrefers[newPath] = yes
						continue
					default:
						// Since there are already two or more **packages** for
						// this path with no 'prefer' specified, the graph
						// must be disconnected.
						hasPrefers[newPath] = no
					}
					err := checkConflict(newPath, old, new)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	// Note: Disconnected graphs have already been filtered out before
	// tarjanSort. If a graph is disconnected it has at least two nodes
	// with no prefers which results in an error above.
	ts := tarjanSort(successors)
	for i, names := range ts {
		if len(names) > 1 {
			slices.Sort(names)
			var cyclePkgs []string
			var path string
			for _, name := range names {
				p, pkg := parsePreferKey(name)
				// All paths are the same.
				path = p
				cyclePkgs = append(cyclePkgs, pkg)
			}
			// Lookup can never fail by construction.
			s, _ := nodes[preferKey(path, cyclePkgs[0])]
			return fmt.Errorf("slice %s path %s has a 'prefer' cycle: %s",
				s, path, strings.Join(cyclePkgs, ", "))
		}
		current := names[0]
		path, pkg := parsePreferKey(current)
		if hasPrefers[path] == no {
			continue
		}
		if i < len(ts)-1 {
			predecessor := ts[i+1][0]
			predPath, _ := parsePreferKey(predecessor)
			if predPath == path && slices.Index(successors[predecessor], current) == -1 {
				// If the previous node does not point to the current one it means
				// we have a Y-shaped graph.
				new, old := nodes[current], nodes[predecessor]
				if old.Package > new.Package || old.Package == new.Package && old.Name > new.Name {
					old, new = new, old
				}
				return fmt.Errorf("slices %s and %s create an invalid prefer ordering for path %s",
					new, old, path)
			}
		}
		s, hasPath := nodes[current]
		if hasPath {
			_, ok := s.Contents[path]
			hasPath = ok
		}
		if !hasPath {
			// If the current node is in the list and it doesn't have the path,
			// it has a predecessor.
			predSlice := nodes[ts[i+1][0]]
			return fmt.Errorf(`slice %s path %s has invalid 'prefer': "%s": package does not have path %s`,
				predSlice, path, pkg, path)
		}
		r.pathOrdering[path] = append(r.pathOrdering[path], pkg)
	}

	// Check for glob and generate conflicts.
	for oldPath, old := range globs {
		oldInfo := old.Contents[oldPath]
		for newPath, new := range paths {
			if oldPath == newPath {
				// Identical globs have been filtered earlier. This must be the
				// exact same entry.
				continue
			}
			if !strdist.GlobPath(newPath, oldPath) {
				continue
			}
			toCheck := []*Slice{new}
			if hasPrefers[newPath] == yes {
				toCheck = []*Slice{}
				for _, pkg := range r.pathOrdering[newPath] {
					// We have already checked above that the node exists
					// when verifying the ordering.
					s, _ := nodes[preferKey(newPath, pkg)]
					toCheck = append(toCheck, s)
				}
			}
			for _, new := range toCheck {
				// It is okay to check only one slice per packages because the
				// content has been validated to be the same earlier.
				newInfo := new.Contents[newPath]
				if oldInfo.Kind == GlobPath && (newInfo.Kind == GlobPath || newInfo.Kind == CopyPath) {
					if new.Package == old.Package {
						continue
					}
				}
				if (old.Package > new.Package) || (old.Package == new.Package && old.Name > new.Name) ||
					(old.Package == new.Package && old.Name == new.Name && oldPath > newPath) {
					old, new = new, old
					oldPath, newPath = newPath, oldPath
				}
				return fmt.Errorf("slices %s and %s conflict on %s and %s", old, new, oldPath, newPath)
			}
		}
	}

	// Check for cycles.
	_, err := order(r.Packages, keys)
	if err != nil {
		return err
	}

	// Check for archive priority conflicts.
	priorities := make(map[int]*Archive)
	for _, archive := range r.Archives {
		if old, ok := priorities[archive.Priority]; ok {
			if old.Name > archive.Name {
				archive, old = old, archive
			}
			return fmt.Errorf("chisel.yaml: archives %q and %q have the same priority value of %d", old.Name, archive.Name, archive.Priority)
		}
		priorities[archive.Priority] = archive
	}

	// Check that archives pinned in packages are defined.
	for _, pkg := range r.Packages {
		if pkg.Archive == "" {
			continue
		}
		if _, ok := r.Archives[pkg.Archive]; !ok {
			return fmt.Errorf("%s: package refers to undefined archive %q", pkg.Path, pkg.Archive)
		}
	}

	return nil
}

func order(pkgs map[string]*Package, keys []SliceKey) ([]SliceKey, error) {

	// Preprocess the list to improve error messages.
	for _, key := range keys {
		if pkg, ok := pkgs[key.Package]; !ok {
			return nil, fmt.Errorf("slices of package %q not found", key.Package)
		} else if _, ok := pkg.Slices[key.Slice]; !ok {
			return nil, fmt.Errorf("slice %s not found", key)
		}
	}

	// Collect all relevant package slices.
	successors := map[string][]string{}
	pending := append([]SliceKey(nil), keys...)

	seen := make(map[SliceKey]bool)
	for i := 0; i < len(pending); i++ {
		key := pending[i]
		if seen[key] {
			continue
		}
		seen[key] = true
		pkg := pkgs[key.Package]
		slice := pkg.Slices[key.Slice]
		fqslice := slice.String()
		predecessors := successors[fqslice]
		for _, req := range slice.Essential {
			fqreq := req.String()
			if reqpkg, ok := pkgs[req.Package]; !ok || reqpkg.Slices[req.Slice] == nil {
				return nil, fmt.Errorf("%s requires %s, but slice is missing", fqslice, fqreq)
			}
			predecessors = append(predecessors, fqreq)
		}
		successors[fqslice] = predecessors
		pending = append(pending, slice.Essential...)
	}

	// Sort them up.
	var order []SliceKey
	for _, names := range tarjanSort(successors) {
		if len(names) > 1 {
			return nil, fmt.Errorf("essential loop detected: %s", strings.Join(names, ", "))
		}
		name := names[0]
		dot := strings.IndexByte(name, '_')
		order = append(order, SliceKey{name[:dot], name[dot+1:]})
	}

	return order, nil
}

func readRelease(baseDir string) (*Release, error) {
	baseDir = filepath.Clean(baseDir)
	filePath := filepath.Join(baseDir, "chisel.yaml")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read release definition: %s", err)
	}
	release, err := parseRelease(baseDir, filePath, data)
	if err != nil {
		return nil, err
	}
	err = readSlices(release, baseDir, filepath.Join(baseDir, "slices"))
	if err != nil {
		return nil, err
	}
	return release, err
}

func readSlices(release *Release, baseDir, dirName string) error {
	entries, err := os.ReadDir(dirName)
	if err != nil {
		return fmt.Errorf("cannot read %s%c directory", stripBase(baseDir, dirName), filepath.Separator)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			err := readSlices(release, baseDir, filepath.Join(dirName, entry.Name()))
			if err != nil {
				return err
			}
			continue
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		match := apacheutil.FnameExp.FindStringSubmatch(entry.Name())
		if match == nil {
			return fmt.Errorf("invalid slice definition filename: %q", entry.Name())
		}

		pkgName := match[1]
		pkgPath := filepath.Join(dirName, entry.Name())
		if pkg, ok := release.Packages[pkgName]; ok {
			return fmt.Errorf("package %q slices defined more than once: %s and %s\")", pkgName, pkg.Path, pkgPath)
		}
		data, err := os.ReadFile(pkgPath)
		if err != nil {
			// Errors from package os generally include the path.
			return fmt.Errorf("cannot read slice definition file: %v", err)
		}

		pkg, err := parsePackage(baseDir, pkgName, stripBase(baseDir, pkgPath), data)
		if err != nil {
			return err
		}

		release.Packages[pkg.Name] = pkg
	}
	return nil
}

func stripBase(baseDir, path string) string {
	// Paths must be clean for this to work correctly.
	return strings.TrimPrefix(path, baseDir+string(filepath.Separator))
}

func Select(release *Release, slices []SliceKey) (*Selection, error) {
	logf("Selecting slices...")

	selection := &Selection{
		Release:             release,
		cachedSelectPackage: make(map[string]string),
	}

	sorted, err := order(release.Packages, slices)
	if err != nil {
		return nil, err
	}
	selection.Slices = make([]*Slice, len(sorted))
	for i, key := range sorted {
		selection.Slices[i] = release.Packages[key.Package].Slices[key.Slice]
	}

	for _, new := range selection.Slices {
		for newPath, newInfo := range new.Contents {
			// An invalid "generate" value should only throw an error if that
			// particular slice is selected. Hence, the check is here.
			switch newInfo.Generate {
			case GenerateNone, GenerateManifest:
			default:
				return nil, fmt.Errorf("slice %s has invalid 'generate' for path %s: %q",
					new, newPath, newInfo.Generate)
			}
		}
	}

	return selection, nil
}

func preferKey(path, pkg string) string {
	return path + "|" + pkg
}

func parsePreferKey(key string) (path string, pkg string) {
	i := strings.LastIndex(key, "|")
	return key[:i], key[i+1:]
}
