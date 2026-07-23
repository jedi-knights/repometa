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
			// A .sln is the Visual Studio solution format used across
			// every MSBuild language (C#, F#, VB, and C++). We only
			// emit dotnet-solution when the .sln references at least
			// one .NET project file — a pure-C++ solution or one whose
			// projects are all solution folders is not a .NET workspace
			// and should not be labeled as such. The C++ projects it
			// does contain still surface as cpp-project components at
			// their own directories.
			members := parseSlnMembers(filepath.Join(dv.abs, name), cfg)
			if len(members) == 0 {
				continue
			}
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

		case ".vcxproj":
			// Visual Studio C++ project. Shares the MSBuild machinery
			// with .csproj but targets native C/C++ code, so it maps to
			// LanguageC in the polyglot classifier and suppresses loose
			// C-source detection inside the same directory (see
			// isCBuildKind in scan.go).
			out = append(out, finding{
				Kind:       KindCppProject,
				Confidence: 1.0,
				Evidence: []Evidence{{
					Path:   relJoin(dv.rel, name),
					Reason: ".vcxproj project file",
				}},
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
	data := readManifestOrNil(path, cfg)
	if data == nil {
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
