package repometa

import "fmt"

type cDetector struct{}

// C source directories are common inside larger projects, so this detector
// reports every directory that contains .c / .h files. The post-filter
// pass in Scan drops findings that live inside a component of a more
// structured kind (Make / CMake / etc), so what remains is only "loose"
// C source trees the caller may want to know about.
func (cDetector) detect(dv dirVisit, cfg options) []finding {
	_ = cfg // detector inspects only directory entries; cfg unused.
	sources := countByExt(dv.files, ".c")
	headers := countByExt(dv.files, ".h")
	if sources == 0 && headers == 0 {
		return nil
	}
	return []finding{{
		Kind:       KindCSource,
		Confidence: 0.6,
		Evidence: []Evidence{{
			Path:   dv.rel,
			Reason: fmt.Sprintf("directory contains %d .c and %d .h file(s)", sources, headers),
		}},
	}}
}
