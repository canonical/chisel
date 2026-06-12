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
	Segment  segment
	Slices   []segmentSlice
	Children map[string]*node
}

// pathConflictTree uses a custom trie to find conflicts that might arise from
// extracting different paths into the same root directory.
//
// It optimizes conflict resolution by calling strdist.GlobPath only when
// strictly necessary and by passing it less data to compare. It relies on the
// fact that real chisel releases most paths often share a very long prefix
// that does not need to be compared each time. Additionally, our grammar is
// very restrictive (only "*", "?" and "**") meaning that unless "**" is used,
// any symbol can only match until a "/" is found.
//
// Because of the above, this algorithms splits paths into segments that are
// delimited by "/". When inserting a path, each segment is compared at most
// once with the path independently of how many paths there are in the release.
// Lastly, when looking for conflicts, if the segments do not contain "**" then
// instead of comparing the whole path we can compare only the segment.
type pathConflictTree struct {
	Root         *node
	PathToSlices map[string][]*Slice
}

func newConflictTree(pathToSlices map[string][]*Slice) pathConflictTree {
	root := &node{
		Segment:  segment{"/", false, false},
		Children: map[string]*node{},
	}
	return pathConflictTree{Root: root, PathToSlices: pathToSlices}
}

func (g *pathConflictTree) HasConflict() error {
	for path, slices := range g.PathToSlices {
		var oldInfos []segmentSlice
		for _, oldSlice := range slices {
			oldInfos = append(oldInfos, segmentSlice{oldSlice, oldSlice.Contents[path], path})
		}
		segments, err := pathToSegments(path)
		if err != nil {
			return err
		}
		err = g.pathHasConflict(path, segments, oldInfos)
		if err != nil {
			return err
		}
		g.insertSegments(segments, oldInfos)
	}
	return nil
}

func (g *pathConflictTree) pathHasConflict(oldPath string, oldSegments []segment, oldInfos []segmentSlice) error {
	conflictErrMsg := func(oldInfo, newInfo *segmentSlice) error {
		oldSlice, oldPath := oldInfo.Slice, oldInfo.WholePath
		newSlice, newPath := newInfo.Slice, newInfo.WholePath
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
	oldSegments = oldSegments[1:]

	for len(currentQueue) > 0 {
		oldSegment := oldSegments[0]
		for _, newNode := range currentQueue {
		newNodeLoop:
			for _, oldSegmentInfo := range oldInfos {
				oldSlice := oldSegmentInfo.Slice
				oldPathInfo := oldSegmentInfo.PathInfo
				for _, newSegmentInfo := range newNode.Slices {
					newSlice := newSegmentInfo.Slice
					newPathInfo := newSegmentInfo.PathInfo
					newSegment := newNode.Segment

					// If slices cannot conflict then skip the more expensive
					// checks.
					if (oldPathInfo.Kind == GlobPath || oldPathInfo.Kind == CopyPath) && (newPathInfo.Kind == GlobPath || newPathInfo.Kind == CopyPath) {
						if newSlice.Package == oldSlice.Package {
							// If content is **extracted** from the same
							// package, it will necessarily be the same.
							continue
						}
					}

					if newSegment.HasDoubleGlob || oldSegment.HasDoubleGlob {
						// Case 1: One of the strings has a double glob, we
						// need to check the whole remaining path against
						// each other.
						if strdist.GlobPath(oldSegmentInfo.WholePath, newSegmentInfo.WholePath) {
							return conflictErrMsg(&oldSegmentInfo, &newSegmentInfo)
						}
					} else if newSegment.HasGlob || oldSegment.HasGlob {
						// Case 2: Either segment has a single glob (* or ?).
						// We only need to check the segment.
						if strdist.GlobPath(oldSegment.Text, newSegment.Text) {
							// Only when we get to leaf (i.e. no children, can
							// we have a conflict).
							if len(newNode.Children) == 0 {
								if len(oldSegments) == 1 {
									// If we are at the terminal node of both paths we found a conflict.
									return conflictErrMsg(&oldSegmentInfo, &newSegmentInfo)
								} else {
									// If oldPath is not yet finished we will keep comparing it against
									// this segment. Example: ["/", "a/", "*", ""] and ["/", "a/", ""];
									// the segments ["*", ""] match [""].
									nextQueue = append(nextQueue, newNode)
								}
							}
							for _, child := range newNode.Children {
								nextQueue = append(nextQueue, child)
							}
							break newNodeLoop
						} else {
							// Once GlobPath returns false there cannot be a
							// conflict  between oldPath and newPath, we can
							// break here.
							break newNodeLoop
						}
					} else {
						// Case 3: No globs, we can compare the strings directly.
						if oldSegment.Text == newSegment.Text {
							if len(newNode.Children) == 0 && len(oldSegments) == 1 {
								// If these are both terminal nodes, conflict found.
								return conflictErrMsg(&oldSegmentInfo, &newSegmentInfo)
							}
							for _, child := range newNode.Children {
								nextQueue = append(nextQueue, child)
							}
							break newNodeLoop
						}
					}
				}
			}
		}
		currentQueue, nextQueue = nextQueue, currentQueue
		nextQueue = nextQueue[0:0]

		if len(oldSegments) > 1 {
			// If the segment is a termination node keep it. See example in case 2.
			oldSegments = oldSegments[1:]
		}
	}

	return nil
}

// insertSegments inserts the path's segments blindly in the graph without
// looking at conflicts.
func (g *pathConflictTree) insertSegments(segments []segment, infos []segmentSlice) {
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
		current.Slices = append(current.Slices, infos...)
		parent.Children[segment.Text] = current
		parent = current
	}
}

// pathToSegments returns the list of segments that compose the path plus the
// empty segment "" for explicit termination in the trie.
func pathToSegments(path string) ([]segment, error) {
	if path[0] != '/' {
		return nil, errors.New("internal error: path does not start with '/'")
	}
	segments := []segment{segment{"/", false, false}}
	path = path[1:]
	for {
		end, singleGlob, doubleGlob := segmentEnd(path)
		segment := segment{
			Text:          path[:end+1],
			HasGlob:       singleGlob,
			HasDoubleGlob: doubleGlob,
		}
		segments = append(segments, segment)
		path = path[end+1:]
		if segment.Text == "" {
			break
		}
	}
	return segments, nil
}

// segmentEnd finds the end of a segment according to the following rules:
//   - If s contains "/" then segment will finish at the first "/" found unless
//     there is a "**" before that, in that case segment = s.
//   - Else segment = s.
//
// hasGlob is set to true if "*", "?" or "**" is found in the segment.
// hasDoubleGlob is set to true if "**" is found in the segment.
func segmentEnd(s string) (end int, hasGlob bool, hasDoubleGlob bool) {
	end = strings.IndexAny(s, "*?/")
	if end == -1 {
		end = len(s) - 1
	} else if s[end] == '*' || s[end] == '?' {
		hasGlob = true
		slash := strings.IndexRune(s[end:], '/')
		if slash != -1 {
			end = end + slash
		} else {
			end = len(s) - 1
		}
		hasDoubleGlob = strings.Contains(s[:end+1], "**")
		if hasDoubleGlob {
			end = len(s) - 1
		}
	}
	return end, hasGlob, hasDoubleGlob
}
