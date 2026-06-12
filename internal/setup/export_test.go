package setup

type YAMLPath = yamlPath

type PathSegment = segment
type PathSegmentSlice = segmentSlice
type PathNode = node
type PathConflictTree = pathConflictTree

var PathToSegments func(string) ([]PathSegment, error) = pathToSegments
var NewConflictTree func(map[string][]*Slice) PathConflictTree = newConflictTree
