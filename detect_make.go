package repometa

type makeDetector struct{}

func (makeDetector) detect(dv dirVisit, cfg options) []finding {
	_ = cfg // detector inspects only directory entries; cfg unused.
	name, ok := firstFile(dv.files, "GNUmakefile", "Makefile", "makefile")
	if !ok {
		return nil
	}
	return []finding{{
		Kind:       KindMakeProject,
		Confidence: 1.0,
		Evidence:   []Evidence{{Path: relJoin(dv.rel, name), Reason: name + " at directory root"}},
	}}
}
