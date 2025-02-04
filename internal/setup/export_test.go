package setup

type YAMLPath = yamlPath

func (r *Release) SetPathOrdering(ordering map[string][]string) {
	r.pathOrdering = ordering
}

func (s *Selection) ClearCache() {
	s.cachedSelectPackage = nil
}
