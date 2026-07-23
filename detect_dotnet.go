package repometa

import (
	"path/filepath"
	"regexp"
	"strings"
)

type dotnetDetector struct{}

// slnProjectLineRE matches the Project(...) header lines that .sln files
// use to declare each member project. Format:
//
//	Project("{TYPE-GUID}") = "Name", "relative/path/to/Name.csproj", "{PROJECT-GUID}"
//
// Only the second field (the path) is captured; the type GUID (which
// distinguishes projects from solution folders) is inspected downstream
// by the extension of the path itself.
var slnProjectLineRE = regexp.MustCompile(`(?i)^Project\([^)]+\)\s*=\s*"[^"]*",\s*"([^"]+)"`)

func (dotnetDetector) detect(dv dirVisit, cfg options) []finding {
	var out []finding

	for _, f := range dv.files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		ext := strings.ToLower(filepath.Ext(name))

		switch ext {
		case ".sln":
			members := parseSlnMembers(filepath.Join(dv.abs, name), cfg)
			out = append(out, finding{
				Kind:       KindDotNetSolution,
				Confidence: 1.0,
				Evidence: []Evidence{{
					Path:   relJoin(dv.rel, name),
					Reason: ".sln at directory root",
				}},
				Workspaces: []Workspace{{
					Kind:    WorkspaceDotNetSolution,
					Members: expandMembers(dv.abs, members, dv.rel),
				}},
			})

		case ".csproj", ".fsproj", ".vbproj":
			out = append(out, finding{
				Kind:       KindDotNetProject,
				Confidence: 1.0,
				Evidence: []Evidence{{
					Path:   relJoin(dv.rel, name),
					Reason: ext + " project file",
				}},
				Attributes: map[string]string{
					"dotnet.language": dotnetLanguageFor(ext),
				},
			})
		}
	}
	return out
}

// dotnetLanguageFor maps a project-file extension to the language label
// exposed on the dotnet.language attribute.
func dotnetLanguageFor(ext string) string {
	switch ext {
	case ".csproj":
		return "csharp"
	case ".fsproj":
		return "fsharp"
	case ".vbproj":
		return "vb"
	}
	return ""
}

// parseSlnMembers walks the Project(...) lines of a .sln file and returns
// the directory-relative paths of every C#/F#/VB project it references.
// Solution folders (identified by their type GUID) reuse the Project(...)
// prefix but point at a virtual name rather than a real project file —
// they are filtered out by extension so members line up with the
// dotnet-project components emitted by the walker.
//
// .sln paths are Windows-style with backslashes; they are normalized to
// forward slashes for cross-platform stability.
func parseSlnMembers(path string, cfg options) []string {
	data, err := readManifest(path, cfg)
	if err != nil {
		return nil
	}
	var members []string
	seen := make(map[string]bool)
	for line := range strings.SplitSeq(string(data), "\n") {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, "Project(") {
			continue
		}
		m := slnProjectLineRE.FindStringSubmatch(trim)
		if len(m) != 2 {
			continue
		}
		p := strings.ReplaceAll(m[1], `\`, "/")
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".csproj" && ext != ".fsproj" && ext != ".vbproj" {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(p))
		if dir == "" || dir == "." {
			continue
		}
		if !seen[dir] {
			seen[dir] = true
			members = append(members, dir)
		}
	}
	return members
}
