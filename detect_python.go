package repometa

import (
	"io/fs"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type pythonDetector struct{}

type pyprojectTOML struct {
	Tool struct {
		Uv *struct {
			Workspace *struct {
				Members []string `toml:"members"`
			} `toml:"workspace"`
		} `toml:"uv"`
		Poetry *struct{} `toml:"poetry"`
	} `toml:"tool"`
	Project *struct {
		Name string `toml:"name"`
	} `toml:"project"`
}

func (pythonDetector) detect(dv dirVisit, cfg options) []finding {
	// Any of these markers makes the directory a Python package for our
	// purposes. pyproject.toml wins the "primary" evidence slot when
	// present because it is the modern canonical form.
	markers := []string{"pyproject.toml", "setup.py", "setup.cfg", "requirements.txt", "Pipfile"}
	primary, ok := firstFile(dv.files, markers...)
	if !ok {
		return nil
	}

	attrs := detectPythonPM(dv.files)
	if attrs == nil {
		attrs = map[string]string{}
	}
	evidence := []Evidence{{Path: relJoin(dv.rel, primary), Reason: "Python packaging marker"}}
	var workspaces []Workspace

	confidence := 1.0
	if primary == "pyproject.toml" {
		data, err := readManifest(filepath.Join(dv.abs, "pyproject.toml"), cfg)
		switch {
		case err != nil:
			confidence = 0.8
			evidence = append(evidence, Evidence{
				Path:   relJoin(dv.rel, "pyproject.toml"),
				Reason: "pyproject.toml unreadable: " + err.Error(),
			})
		default:
			var pp pyprojectTOML
			if uerr := toml.Unmarshal(data, &pp); uerr != nil {
				confidence = 0.7
				evidence = append(evidence, Evidence{
					Path:   relJoin(dv.rel, "pyproject.toml"),
					Reason: "pyproject.toml parse error: " + uerr.Error(),
				})
				break
			}
			if pp.Tool.Uv != nil && pp.Tool.Uv.Workspace != nil {
				members := expandMembers(dv.abs, pp.Tool.Uv.Workspace.Members, dv.rel)
				workspaces = append(workspaces, Workspace{Kind: WorkspaceUv, Members: members})
				evidence = append(evidence, Evidence{
					Path:   relJoin(dv.rel, "pyproject.toml"),
					Reason: "pyproject.toml declares [tool.uv.workspace]",
				})
			}
			if pp.Tool.Poetry != nil {
				// Poetry is not a workspace tool in v0 — recorded as
				// package-manager attribute only.
				attrs["python.pm"] = "poetry"
			}
		}
	}

	return []finding{{
		Kind:       KindPythonPackage,
		Confidence: confidence,
		Evidence:   evidence,
		Workspaces: workspaces,
		Attributes: attrs,
	}}
}

// detectPythonPM returns the most specific package-manager attribution
// available from lockfiles / marker files in the directory. Order reflects
// modern preference: uv > poetry > pipenv > pip. The result may be empty
// when no lockfile or requirements.txt is present.
func detectPythonPM(files []fs.DirEntry) map[string]string {
	attrs := map[string]string{}
	switch {
	case hasFile(files, "uv.lock"):
		attrs["python.pm"] = "uv"
	case hasFile(files, "poetry.lock"):
		attrs["python.pm"] = "poetry"
	case hasFile(files, "Pipfile.lock") || hasFile(files, "Pipfile"):
		attrs["python.pm"] = "pipenv"
	case hasFile(files, "requirements.txt"):
		attrs["python.pm"] = "pip"
	}
	return attrs
}
