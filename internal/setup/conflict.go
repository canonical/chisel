package setup

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/canonical/chisel/internal/strdist"
)

type segmentSlice struct {
	Slice *Slice
	// PathInfo is kept here as an optimization to avoid lookups on
	// Slice.Contents for every slice.
	PathInfo PathInfo
	// WholePath is used to simplify both error reporting and matching against
	// paths with "**"; both of which require reconstructing the whole path.
	WholePath string
}

type segment struct {
	Text string
	// HasGlob is set when the path contains "*" or "?" or "**".
	HasGlob bool
	// HasDoubleGlob is set when the path contains "**".
	HasDoubleGlob bool
}

type node struct {
	Segment       segment
	SegmentSlices []*segmentSlice
	Children      map[string]*node
}

// pathConflictTree uses a custom trie to find conflicts that might arise from
// extracting different paths into the same root directory.
//
// It optimizes finding conflicts by calling strdist.GlobPath only when
// strictly necessary and by passing it less data to compare. It relies on the
// fact that in real chisel releases most paths often share a very long prefix
// that does not need to be compared each time. Additionally, our grammar is
// very restrictive (only "*", "?" and "**") meaning that unless "**" is used,
// any symbol can only match until a "/" is found.
//
// Because of the above, this algorithm splits paths into segments that are
// delimited by "/". When inserting a path, each segment is compared at most
// once with the path independently of how many paths there are in the release.
// Lastly, when looking for conflicts, if the segments do not contain "**" then
// instead of comparing the whole path we can compare only the segment.
type pathConflictTree struct {
	Root         *node
	PathToSlices map[string][]*Slice
}

var rootSegment = segment{"/", false, false}

func newConflictTree(pathToSlices map[string][]*Slice) pathConflictTree {
	root := &node{
		Segment:  rootSegment,
		Children: map[string]*node{},
	}
	return pathConflictTree{Root: root, PathToSlices: pathToSlices}
}

func (g *pathConflictTree) HasConflict() error {
	// Make errors deterministic.
	paths := slices.Collect(maps.Keys(g.PathToSlices))
	slices.Sort(paths)

	for _, path := range paths {
		slices := g.PathToSlices[path]
		var segmentSlices []*segmentSlice
		for _, slice := range slices {
			segmentSlices = append(segmentSlices, &segmentSlice{slice, slice.Contents[path], path})
		}
		segments, err := pathToSegments(path)
		if err != nil {
			return err
		}
		err = g.pathHasConflict(segments, segmentSlices)
		if err != nil {
			return err
		}
		g.insertSegments(segments, segmentSlices)
	}
	return nil
}

func (g *pathConflictTree) pathHasConflict(newSegments []segment, newSegmentSlices []*segmentSlice) error {
	conflictErrMsg := func(oldSegmentSlice, newSegmentSlice *segmentSlice) error {
		oldSlice, oldPath := oldSegmentSlice.Slice, oldSegmentSlice.WholePath
		newSlice, newPath := newSegmentSlice.Slice, newSegmentSlice.WholePath
		if (oldSlice.Package > newSlice.Package) || (oldSlice.Package == newSlice.Package && oldSlice.Name > newSlice.Name) ||
			(oldSlice.Package == newSlice.Package && oldSlice.Name == newSlice.Name && oldPath > newPath) {
			oldSlice, newSlice = newSlice, oldSlice
			oldPath, newPath = newPath, oldPath
		}
		return fmt.Errorf("slices %s and %s conflict on %s and %s", oldSlice, newSlice, oldPath, newPath)
	}

	var currentQueue []*node
	var nextQueue []*node

	// Skip "/".
	currentQueue = slices.Collect(maps.Values(g.Root.Children))
	newSegments = newSegments[1:]

	// If we run out of segments from the graph or the path there cannot be a
	// conflict.
	for len(currentQueue) > 0 && len(newSegments) > 0 {
		newSegment := newSegments[0]
		for _, oldNode := range currentQueue {
		oldNodeLoop:
			for _, newSegmentSlice := range newSegmentSlices {
				newSlice := newSegmentSlice.Slice
				newPathInfo := newSegmentSlice.PathInfo
				for _, oldSegmentSlice := range oldNode.SegmentSlices {
					oldSlice := oldSegmentSlice.Slice
					oldPathInfo := oldSegmentSlice.PathInfo
					oldSegment := oldNode.Segment

					// If slices cannot conflict then skip the more expensive
					// checks.
					if (newPathInfo.Kind == GlobPath || newPathInfo.Kind == CopyPath) && (oldPathInfo.Kind == GlobPath || oldPathInfo.Kind == CopyPath) {
						if oldSlice.Package == newSlice.Package {
							// If content is **extracted** from the same
							// package, it will necessarily be the same.
							continue
						}
					}

					if oldSegment.HasDoubleGlob || newSegment.HasDoubleGlob {
						// Case 1: Either segment has a double glob, we need to
						// check the whole remaining path against each other.
						if strdist.GlobPath(newSegmentSlice.WholePath, oldSegmentSlice.WholePath) {
							return conflictErrMsg(oldSegmentSlice, newSegmentSlice)
						}
					} else {
						var matched bool
						if oldSegment.HasGlob || newSegment.HasGlob {
							// Case 2: Either segment has a single glob (* or ?).
							// We only need to check the segment.
							matched = strdist.GlobPath(newSegment.Text, oldSegment.Text)
						} else {
							// Case 3: No globs, we can compare the segments directly.
							matched = newSegment.Text == oldSegment.Text
						}
						if matched {
							if len(oldNode.Children) == 0 && len(newSegments) == 1 {
								// If we are at the terminal node of both paths we found a conflict.
								return conflictErrMsg(oldSegmentSlice, newSegmentSlice)
							}
							for _, child := range oldNode.Children {
								nextQueue = append(nextQueue, child)
							}
							break oldNodeLoop
						} else {
							// Once GlobPath returns false there cannot be a
							// conflict between both paths, we can break here.
							break oldNodeLoop
						}
					}
				}
			}
		}
		currentQueue, nextQueue = nextQueue, currentQueue
		nextQueue = nextQueue[0:0]

		newSegments = newSegments[1:]
	}

	return nil
}

// insertSegments inserts the path's segments blindly in the graph without
// looking at conflicts.
func (g *pathConflictTree) insertSegments(segments []segment, segmentSlices []*segmentSlice) {
	parent := g.Root
	// Skip "/".
	segments = segments[1:]

	for _, segment := range segments {
		current, ok := parent.Children[segment.Text]
		if !ok {
			current = &node{
				Segment:  segment,
				Children: map[string]*node{},
			}
		}
		current.SegmentSlices = append(current.SegmentSlices, segmentSlices...)
		parent.Children[segment.Text] = current
		parent = current
	}
}

// pathToSegments returns the list of segments that compose the path.
// Directories, i.e. paths that end with "/", contain the empty segment "" for
// explicit termination in the trie to distinguish them from parent directories
// of other paths.
func pathToSegments(path string) ([]segment, error) {
	if path[0] != '/' {
		return nil, errors.New("internal error: path does not start with '/'")
	}
	segments := []segment{rootSegment}
	path = path[1:]
	for {
		end, singleGlob, doubleGlob := segmentEnd(path)
		segment := segment{
			Text:          path[:end],
			HasGlob:       singleGlob,
			HasDoubleGlob: doubleGlob,
		}
		segments = append(segments, segment)
		path = path[end:]
		if path == "" && !strings.HasSuffix(segment.Text, "/") {
			// Non-directories: last segment is also termination node.
			break
		}
		if segment.Text == "" {
			// Directories: add the termination node.
			break
		}
	}
	return segments, nil
}

// segmentEnd finds the end of a segment according to the following rules:
//   - If s contains "/" then segment will finish at the first "/" found unless
//     there is a "**" before that, in that case segment = s.
//   - Else segment = s.
func segmentEnd(s string) (end int, hasGlob bool, hasDoubleGlob bool) {
	end = strings.IndexAny(s, "*?/")
	if end == -1 {
		end = len(s) - 1
	} else if s[end] == '*' || s[end] == '?' {
		hasGlob = true
		slash := strings.IndexRune(s[end:], '/')
		if slash == -1 {
			end = len(s) - 1
		} else {
			end = end + slash
		}
		hasDoubleGlob = strings.Contains(s[:end+1], "**")
		if hasDoubleGlob {
			end = len(s) - 1
		}
	}
	return end + 1, hasGlob, hasDoubleGlob
}
