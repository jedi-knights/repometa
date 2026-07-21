package repometa

import "fmt"

type asmDetector struct{}

func (asmDetector) detect(dv dirVisit, cfg options) []finding {
	_ = cfg // detector inspects only directory entries; cfg unused.
	n := countByExt(dv.files, ".s", ".S", ".asm")
	if n == 0 {
		return nil
	}
	return []finding{{
		Kind:       KindAsmSource,
		Confidence: 0.6,
		Evidence: []Evidence{{
			Path:   dv.rel,
			Reason: fmt.Sprintf("directory contains %d assembly source file(s) (.s/.S/.asm)", n),
		}},
	}}
}
