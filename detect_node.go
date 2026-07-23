package repometa

import (
	"encoding/json"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type jsDetector struct{}

type packageJSON struct {
	Name            string            `json:"name"`
	Workspaces      json.RawMessage   `json:"workspaces"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

type pnpmWorkspaceYAML struct {
	Packages []string `yaml:"packages"`
}

func (jsDetector) detect(dv dirVisit, cfg options) []finding {
	if !hasFile(dv.files, "package.json") {
		return nil
	}

	attrs := map[string]string{}
	evidence := []Evidence{{Path: relJoin(dv.rel, "package.json"), Reason: "package.json at directory root"}}
	var workspaces []Workspace
	confidence := 1.0

	data, err := readManifest(filepath.Join(dv.abs, "package.json"), cfg)
	switch {
	case err != nil:
		confidence = confidenceUnreadable
		evidence = append(evidence, evidenceUnreadable(dv.rel, "package.json", err))
	default:
		var pkg packageJSON
		if uerr := json.Unmarshal(data, &pkg); uerr != nil {
			confidence = confidenceUnparsable
			evidence = append(evidence, evidenceUnparsable(dv.rel, "package.json", uerr))
			break
		}
		if fw := detectJSFramework(pkg); fw != "" {
			attrs["js.framework"] = fw
		}
		if members, ok := parseNpmYarnWorkspaces(pkg.Workspaces); ok {
			expanded := expandMembers(dv.abs, members, dv.rel)
			workspaces = append(workspaces, Workspace{Kind: WorkspaceNpmYarn, Members: expanded})
			evidence = append(evidence, Evidence{
				Path: relJoin(dv.rel, "package.json"), Reason: `package.json "workspaces" declared`,
			})
		}
	}

	if hasFile(dv.files, "pnpm-workspace.yaml") {
		members := parsePnpmWorkspace(filepath.Join(dv.abs, "pnpm-workspace.yaml"), cfg)
		expanded := expandMembers(dv.abs, members, dv.rel)
		workspaces = append(workspaces, Workspace{Kind: WorkspacePnpm, Members: expanded})
		evidence = append(evidence, Evidence{Path: relJoin(dv.rel, "pnpm-workspace.yaml"), Reason: "pnpm-workspace.yaml present"})
	}
	if hasFile(dv.files, "nx.json") {
		workspaces = append(workspaces, Workspace{Kind: WorkspaceNx})
		evidence = append(evidence, Evidence{Path: relJoin(dv.rel, "nx.json"), Reason: "nx.json present"})
	}
	if hasFile(dv.files, "turbo.json") {
		workspaces = append(workspaces, Workspace{Kind: WorkspaceTurborepo})
		evidence = append(evidence, Evidence{Path: relJoin(dv.rel, "turbo.json"), Reason: "turbo.json present"})
	}

	// Angular's canonical marker is angular.json, which may exist even
	// without @angular/core being an obvious dependency line.
	if hasFile(dv.files, "angular.json") {
		attrs["js.framework"] = "angular"
		evidence = append(evidence, Evidence{Path: relJoin(dv.rel, "angular.json"), Reason: "angular.json present"})
	}

	return []finding{{
		Kind:       KindNodePackage,
		Confidence: confidence,
		Evidence:   evidence,
		Workspaces: workspaces,
		Attributes: attrs,
	}}
}

func detectJSFramework(pkg packageJSON) string {
	if _, ok := pkg.Dependencies["next"]; ok {
		return "nextjs"
	}
	if _, ok := pkg.DevDependencies["next"]; ok {
		return "nextjs"
	}
	if _, ok := pkg.Dependencies["@angular/core"]; ok {
		return "angular"
	}
	if _, ok := pkg.DevDependencies["@angular/core"]; ok {
		return "angular"
	}
	return ""
}

// parseNpmYarnWorkspaces handles both shapes:
//
//	"workspaces": ["packages/*"]
//	"workspaces": {"packages": ["packages/*"], "nohoist": [...]}
func parseNpmYarnWorkspaces(raw json.RawMessage) ([]string, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, true
	}
	var obj struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj.Packages, true
	}
	return nil, false
}

func parsePnpmWorkspace(path string, cfg options) []string {
	data, err := readManifest(path, cfg)
	if err != nil {
		return nil
	}
	var ws pnpmWorkspaceYAML
	if err := yaml.Unmarshal(data, &ws); err != nil {
		return nil
	}
	return ws.Packages
}
