package repometa

import (
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type rustDetector struct{}

type cargoTOML struct {
	Package   *cargoPackage   `toml:"package"`
	Workspace *cargoWorkspace `toml:"workspace"`
}

type cargoPackage struct {
	Name string `toml:"name"`
}

type cargoWorkspace struct {
	Members []string `toml:"members"`
}

func (rustDetector) detect(dv dirVisit, cfg options) []finding {
	if !hasFile(dv.files, "Cargo.toml") {
		return nil
	}
	path := filepath.Join(dv.abs, "Cargo.toml")
	data, err := readManifest(path, cfg)
	if err != nil {
		return []finding{{
			Kind:       KindRustCrate,
			Confidence: 0.8,
			Evidence:   []Evidence{{Path: relJoin(dv.rel, "Cargo.toml"), Reason: "Cargo.toml unreadable: " + err.Error()}},
		}}
	}

	var manifest cargoTOML
	if err := toml.Unmarshal(data, &manifest); err != nil {
		return []finding{{
			Kind:       KindRustCrate,
			Confidence: 0.7,
			Evidence:   []Evidence{{Path: relJoin(dv.rel, "Cargo.toml"), Reason: "Cargo.toml at directory root (parse error)"}},
		}}
	}

	kind := KindRustCrate
	reason := "Cargo.toml with [package] section"
	if manifest.Package == nil {
		kind = KindRustWorkspace
		reason = "Cargo.toml with [workspace] section, no [package] — virtual workspace"
	}

	var workspaces []Workspace
	if manifest.Workspace != nil {
		members := expandMembers(dv.abs, manifest.Workspace.Members, dv.rel)
		workspaces = append(workspaces, Workspace{Kind: WorkspaceCargo, Members: members})
	}

	return []finding{{
		Kind:       kind,
		Confidence: 1.0,
		Evidence:   []Evidence{{Path: relJoin(dv.rel, "Cargo.toml"), Reason: reason}},
		Workspaces: workspaces,
	}}
}
