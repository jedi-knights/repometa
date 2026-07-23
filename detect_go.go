package repometa

import (
	"path/filepath"
	"strings"
)

type goDetector struct{}

func (goDetector) detect(dv dirVisit, cfg options) []finding {
	var out []finding

	// go.work — the whole directory is a Go workspace. Members are the
	// module paths listed in the `use` directive.
	if hasFile(dv.files, "go.work") {
		members := parseGoWorkUse(filepath.Join(dv.abs, "go.work"), cfg)
		expanded := make([]string, 0, len(members))
		for _, m := range members {
			expanded = append(expanded, joinRel(dv.rel, m))
		}
		out = append(out, finding{
			Kind:       KindGoModule,
			Confidence: 1.0,
			Evidence: []Evidence{
				{Path: relJoin(dv.rel, "go.work"), Reason: "go.work at directory root"},
			},
			Workspaces: []Workspace{{Kind: WorkspaceGo, Members: expanded}},
		})
		return out
	}

	if hasFile(dv.files, "go.mod") {
		out = append(out, finding{
			Kind:       KindGoModule,
			Confidence: 1.0,
			Evidence: []Evidence{
				{Path: relJoin(dv.rel, "go.mod"), Reason: "go.mod at directory root"},
			},
		})
	}
	return out
}

// parseGoWorkUse returns the module directories listed in a go.work file.
// It handles both `use ./mod` and `use ( ... )` block forms. On any read
// error it returns nil.
func parseGoWorkUse(path string, cfg options) []string {
	data := readManifestOrNil(path, cfg)
	if data == nil {
		return nil
	}
	var members []string
	inBlock := false
	for _, line := range strings.Split(string(data), "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "//") {
			continue
		}
		if idx := strings.Index(trim, "//"); idx >= 0 {
			trim = strings.TrimSpace(trim[:idx])
		}
		switch {
		case strings.HasPrefix(trim, "use ("):
			inBlock = true
		case inBlock && trim == ")":
			inBlock = false
		case inBlock:
			m := strings.Trim(trim, "\"'")
			m = strings.TrimPrefix(m, "./")
			if m != "" {
				members = append(members, m)
			}
		case strings.HasPrefix(trim, "use "):
			m := strings.TrimSpace(strings.TrimPrefix(trim, "use "))
			m = strings.Trim(m, "\"'")
			m = strings.TrimPrefix(m, "./")
			if m != "" {
				members = append(members, m)
			}
		}
	}
	return members
}
