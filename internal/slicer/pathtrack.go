package slicer

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/canonical/chisel/internal/db"
	"github.com/canonical/chisel/internal/deb"
	"github.com/canonical/chisel/internal/fsutil"
	"github.com/canonical/chisel/internal/strdist"
)

type pathTracker interface {
	// addSlicePath records that the path belongs to the slice.
	addSlicePath(slice, path string)
	// addSliceGlob records that paths matched by the glob belong to the
	// slice.
	addSliceGlob(slice, glob string)
	// onCreate is a callback passed to deb.ExtractOptions.onData. It
	// records checksums of paths extracted from deb packages.
	onData(source string, size int64) (deb.ConsumeData, error)
	// onCreate is a callback passed to deb.ExtractOptions.OnData. It
	// records metadata of paths extracted from deb packages.
	onCreate(source, target, link string, mode fs.FileMode) error
	// addTarget records the target path as being non-extracted content,
	// i.e. not originating from a deb package.
	addTarget(target, link string, mode fs.FileMode, data []byte)
	// markMutated marks the target path as being changed by mutation
	// scripts.
	markMutated(target string)
	// removeTarget removes the target path. It should be called for paths
	// that have "until" attribute set.
	removeTarget(target string)
	// updateTargets reconciles changes made since the tracking started. It
	// must be called before writing the database with updateDB.
	updateTargets(root string) error
	// upadteDB calls addToDB for each recorded entry.
	updateDB(addToDB AddToDB) error
}

type contentInfo struct {
	size   int64
	digest *[sha256.Size]byte
}

func computeDigest(data []byte) *[sha256.Size]byte {
	digest := sha256.Sum256(data)
	return &digest
}

type stringSet []string

func (s stringSet) AddOne(x string) (stringSet, bool) {
	if s == nil {
		return []string{x}, true
	}
	i := sort.SearchStrings(s, x)
	if i == len(s) {
		s = append(s, x)
	} else if s[i] != x {
		s = append(s[:i], append([]string{x}, s[i:]...)...)
	} else {
		return s, false
	}
	return s, true
}

func (s stringSet) AddMany(xs ...string) stringSet {
	for _, x := range xs {
		s, _ = s.AddOne(x)
	}
	return s
}

type pathTrackCtx struct {
	pathSlices     map[string]stringSet
	globSlices     map[string]stringSet
	targetToSource map[string]string
	sourceContent  map[string]contentInfo
	targets        map[string]*db.Path
	mutatedTargets map[string]bool
}

var _ pathTracker = (*pathTrackCtx)(nil)

func newPathTracker() pathTracker {
	return &pathTrackCtx{
		pathSlices:     make(map[string]stringSet),
		globSlices:     make(map[string]stringSet),
		targetToSource: make(map[string]string),
		sourceContent:  make(map[string]contentInfo),
		targets:        make(map[string]*db.Path),
		mutatedTargets: make(map[string]bool),
	}
}

func (ctx *pathTrackCtx) addSlicePath(slice, path string) {
	ctx.pathSlices[path] = ctx.pathSlices[path].AddMany(slice)
}

func (ctx *pathTrackCtx) addSliceGlob(slice, glob string) {
	ctx.globSlices[glob] = ctx.pathSlices[glob].AddMany(slice)
}

func (ctx *pathTrackCtx) onData(source string, size int64) (deb.ConsumeData, error) {
	// XXX: We should return nil if the source matches one of the
	// until-paths. But that would require some additional expensive
	// tracking. Until-paths are now untracked by removeTarget() called
	// during their removal from the output directory.
	consume := func(reader io.Reader) error {
		data, err := io.ReadAll(reader)
		if err != nil {
			return err
		}
		digest := computeDigest(data)
		ctx.sourceContent[source] = contentInfo{size, digest}
		return nil
	}
	return consume, nil
}

func (ctx *pathTrackCtx) onCreate(source, target, link string, mode fs.FileMode) error {
	info := db.Path{
		Path: target,
		Mode: mode,
		Link: link,
	}
	ctx.targets[target] = &info
	ctx.targetToSource[target] = source
	return nil
}

func (ctx *pathTrackCtx) addTarget(target, link string, mode fs.FileMode, data []byte) {
	info := db.Path{
		Path: target,
		Mode: mode,
		Link: link,
	}
	if data != nil {
		info.Size = int64(len(data))
		info.SHA256 = computeDigest(data)
	}
	ctx.targets[target] = &info
	// add parents
	for parent := fsutil.SlashedPathDir(target); parent != "/"; parent = fsutil.SlashedPathDir(parent) {
		if _, ok := ctx.targets[parent]; ok {
			break
		}
		ctx.targets[parent] = &db.Path{
			Path: parent,
			Mode: fs.ModeDir | 0755,
		}
	}
}

func (ctx *pathTrackCtx) markMutated(target string) {
	ctx.mutatedTargets[target] = true
}

func (ctx *pathTrackCtx) removeTarget(target string) {
	delete(ctx.targets, target)
}

func (ctx *pathTrackCtx) completeTarget(info *db.Path) {
	// keep only permission bits
	info.Mode = info.Mode & 07777

	// copy content info from OnData callbacks
	source := ctx.targetToSource[info.Path]
	if content, ok := ctx.sourceContent[source]; ok {
		info.Size = content.size
		info.SHA256 = content.digest
	}

	// assign slices
	slices := ctx.pathSlices[info.Path]
	for glob, globSlices := range ctx.globSlices {
		if strdist.GlobPath(glob, info.Path) {
			slices = slices.AddMany(globSlices...)
		}
	}

	// assign slices to parents
	path := info.Path
	for len(slices) > 0 && path != "/" {
		newSlices := []string{}
		for _, sl := range slices {
			if tmp, ok := stringSet(info.Slices).AddOne(sl); ok {
				info.Slices = tmp
				newSlices = append(newSlices, sl)
			}
		}
		slices = newSlices
		path = fsutil.SlashedPathDir(path)
		info = ctx.targets[path]
	}
}

// set final digest on mutated files
func (ctx *pathTrackCtx) refreshTarget(info *db.Path, root string) error {
	if !ctx.mutatedTargets[info.Path] || info.SHA256 != nil {
		// not mutated or not a regular file
		return nil
	}
	local := filepath.Join(root, info.Path)
	data, err := os.ReadFile(local)
	if err != nil {
		return err
	}
	finalDigest := computeDigest(data)
	if *finalDigest != *info.SHA256 {
		info.FinalSHA256 = finalDigest
	}
	return nil
}

func (ctx *pathTrackCtx) updateTargets(root string) (err error) {
	for _, info := range ctx.targets {
		ctx.completeTarget(info)
		if err = ctx.refreshTarget(info, root); err != nil {
			break
		}
	}
	return
}

func (ctx *pathTrackCtx) updateDB(addToDB AddToDB) error {
	for _, info := range ctx.targets {
		if err := addToDB(*info); err != nil {
			return fmt.Errorf("cannot write path to db: %w", err)
		}
		for _, sl := range info.Slices {
			content := db.Content{sl, info.Path}
			if err := addToDB(content); err != nil {
				return fmt.Errorf("cannot write content to db: %w", err)
			}
		}
	}
	return nil
}
