package repometa

type cmakeDetector struct{}

func (cmakeDetector) detect(dv dirVisit, cfg options) []finding {
	_ = cfg // detector inspects only directory entries; cfg unused.
	if !hasFile(dv.files, "CMakeLists.txt") {
		return nil
	}
	return []finding{{
		Kind:       KindCMakeProject,
		Confidence: 1.0,
		Evidence:   []Evidence{{Path: relJoin(dv.rel, "CMakeLists.txt"), Reason: "CMakeLists.txt at directory root"}},
	}}
}
