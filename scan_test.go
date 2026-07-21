package repometa

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// buildFixture writes a tree of files under dir where each key is a
// slash-separated path relative to dir and each value is the file
// contents. Missing directories are created as needed.
func buildFixture(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
}

// findComponent returns the first component matching the given kind at
// the given relative root, or nil if not found.
func findComponent(cs []Component, root string, kind Kind) *Component {
	for i := range cs {
		if cs[i].Root == root && cs[i].Kind == kind {
			return &cs[i]
		}
	}
	return nil
}

func TestScanMultiEcosystemMonorepo(t *testing.T) {
	root := t.TempDir()

	buildFixture(t, root, map[string]string{
		// Root is a Go workspace with two modules.
		"go.work": `go 1.24

use (
	./services/api
	./services/worker
)
`,
		"services/api/go.mod":    "module example.com/api\n\ngo 1.24\n",
		"services/api/main.go":   "package main\nfunc main() {}\n",
		"services/worker/go.mod": "module example.com/worker\n\ngo 1.24\n",

		// A Rust workspace with two crates.
		"rust/Cargo.toml": `[workspace]
members = ["crates/*"]
`,
		"rust/crates/core/Cargo.toml": `[package]
name = "core"
version = "0.1.0"
`,
		"rust/crates/cli/Cargo.toml": `[package]
name = "cli"
version = "0.1.0"
`,

		// A Python package that declares a uv workspace.
		"py/pyproject.toml": `[project]
name = "root-pkg"
version = "0.1.0"

[tool.uv.workspace]
members = ["packages/*"]
`,
		"py/uv.lock":                    "# lockfile placeholder\n",
		"py/packages/alpha/pyproject.toml": "[project]\nname = \"alpha\"\nversion = \"0.1.0\"\n",
		"py/packages/beta/pyproject.toml":  "[project]\nname = \"beta\"\nversion = \"0.1.0\"\n",

		// A Node package with Next.js and an npm workspace declaration.
		"web/package.json": `{
  "name": "web-root",
  "private": true,
  "workspaces": ["apps/*"],
  "dependencies": {"next": "14.0.0"}
}`,
		"web/apps/site/package.json": `{"name": "site", "dependencies": {"next": "14.0.0"}}`,

		// A CMake project with C sources; the loose-C detector must be
		// suppressed inside this directory.
		"native/CMakeLists.txt": "cmake_minimum_required(VERSION 3.10)\nproject(native)\n",
		"native/src/main.c":     "int main(void){return 0;}\n",
		"native/src/util.h":     "#pragma once\n",

		// A make project.
		"tools/legacy/Makefile": "all:\n\techo hi\n",
		"tools/legacy/tool.c":   "int main(void){return 0;}\n",

		// A loose C source tree that should be reported (no Make/CMake).
		"experiments/foo.c": "int foo(void){return 0;}\n",
		"experiments/foo.h": "int foo(void);\n",

		// An assembly source dir with no build system.
		"asm/boot.S": "// boot\n",

		// A directory that should be skipped entirely.
		"node_modules/react/package.json": `{"name": "react"}`,
	})

	manifest, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// The Go workspace at "." should exist and reference both modules.
	goWS := findComponent(manifest.Components, ".", KindGoModule)
	if goWS == nil {
		t.Fatalf("expected go-module at root; components=%+v", manifest.Components)
	}
	if len(goWS.Workspaces) != 1 || goWS.Workspaces[0].Kind != WorkspaceGo {
		t.Fatalf("root go-module missing go-workspace: %+v", goWS.Workspaces)
	}
	wantGoMembers := []string{"services/api", "services/worker"}
	gotGoMembers := append([]string(nil), goWS.Workspaces[0].Members...)
	sort.Strings(gotGoMembers)
	if strings.Join(gotGoMembers, ",") != strings.Join(wantGoMembers, ",") {
		t.Errorf("go workspace members: got %v want %v", gotGoMembers, wantGoMembers)
	}

	// The Rust workspace at "rust" should expand crates/* into two crates.
	rustWS := findComponent(manifest.Components, "rust", KindRustWorkspace)
	if rustWS == nil {
		t.Fatalf("expected rust-workspace at rust; components=%+v", manifest.Components)
	}
	if len(rustWS.Workspaces) != 1 || rustWS.Workspaces[0].Kind != WorkspaceCargo {
		t.Fatalf("rust workspace missing cargo-workspace: %+v", rustWS.Workspaces)
	}
	wantRustMembers := []string{"rust/crates/cli", "rust/crates/core"}
	gotRustMembers := append([]string(nil), rustWS.Workspaces[0].Members...)
	sort.Strings(gotRustMembers)
	if strings.Join(gotRustMembers, ",") != strings.Join(wantRustMembers, ",") {
		t.Errorf("cargo workspace members: got %v want %v", gotRustMembers, wantRustMembers)
	}

	// The two Rust member crates should also be reported as components.
	if findComponent(manifest.Components, "rust/crates/core", KindRustCrate) == nil {
		t.Errorf("expected rust-crate at rust/crates/core")
	}
	if findComponent(manifest.Components, "rust/crates/cli", KindRustCrate) == nil {
		t.Errorf("expected rust-crate at rust/crates/cli")
	}

	// Python package at py with uv workspace + uv PM attribution.
	py := findComponent(manifest.Components, "py", KindPythonPackage)
	if py == nil {
		t.Fatalf("expected python-package at py")
	}
	if py.Attributes["python.pm"] != "uv" {
		t.Errorf("python.pm: got %q want uv", py.Attributes["python.pm"])
	}
	if len(py.Workspaces) != 1 || py.Workspaces[0].Kind != WorkspaceUv {
		t.Errorf("expected uv workspace on py: %+v", py.Workspaces)
	}
	wantPyMembers := []string{"py/packages/alpha", "py/packages/beta"}
	gotPyMembers := append([]string(nil), py.Workspaces[0].Members...)
	sort.Strings(gotPyMembers)
	if strings.Join(gotPyMembers, ",") != strings.Join(wantPyMembers, ",") {
		t.Errorf("uv workspace members: got %v want %v", gotPyMembers, wantPyMembers)
	}

	// Node package at web with Next.js + npm/yarn workspace.
	web := findComponent(manifest.Components, "web", KindNodePackage)
	if web == nil {
		t.Fatalf("expected node-package at web")
	}
	if web.Attributes["js.framework"] != "nextjs" {
		t.Errorf("js.framework: got %q want nextjs", web.Attributes["js.framework"])
	}
	if len(web.Workspaces) != 1 || web.Workspaces[0].Kind != WorkspaceNpmYarn {
		t.Errorf("expected npm-yarn workspace on web: %+v", web.Workspaces)
	}

	// CMake project detected; loose C tree under it must be suppressed.
	if findComponent(manifest.Components, "native", KindCMakeProject) == nil {
		t.Errorf("expected cmake-project at native")
	}
	if findComponent(manifest.Components, "native/src", KindCSource) != nil {
		t.Errorf("loose C source under CMake project should have been suppressed")
	}

	// Make project detected; loose C in same dir must be suppressed.
	if findComponent(manifest.Components, "tools/legacy", KindMakeProject) == nil {
		t.Errorf("expected make-project at tools/legacy")
	}
	if findComponent(manifest.Components, "tools/legacy", KindCSource) != nil {
		t.Errorf("loose C source at make project root should have been suppressed")
	}

	// Loose C source outside any build system stays visible.
	if findComponent(manifest.Components, "experiments", KindCSource) == nil {
		t.Errorf("expected loose c-source-tree at experiments")
	}

	// Assembly source dir with no build system stays visible.
	if findComponent(manifest.Components, "asm", KindAsmSource) == nil {
		t.Errorf("expected asm-source-tree at asm")
	}

	// node_modules must have been skipped entirely.
	for _, c := range manifest.Components {
		if strings.HasPrefix(c.Root, "node_modules") {
			t.Errorf("node_modules should be skipped, got component: %+v", c)
		}
	}

	// Sanity check: no negative counters and at least one dir visited.
	if manifest.Stats.DirsVisited == 0 {
		t.Errorf("Stats.DirsVisited is 0; walker did not run")
	}
}

// Regression test for the coveredBy bug: a Make or CMake project at
// the repo root must still suppress loose C sources in subdirectories.
// Before the fix, Component.Root == "." did not prefix-match any child
// path because "./" is stripped during canonicalization.
func TestScanSuppressesLooseCUnderRootMake(t *testing.T) {
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"Makefile":   "all:\n\techo hi\n",
		"src/main.c": "int main(void){return 0;}\n",
		"src/util.h": "#pragma once\n",
	})

	m, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if findComponent(m.Components, ".", KindMakeProject) == nil {
		t.Fatalf("expected make-project at root; components=%+v", m.Components)
	}
	if c := findComponent(m.Components, "src", KindCSource); c != nil {
		t.Errorf("loose C source under root make-project should be suppressed, got %+v", c)
	}
}

// TestSmokeRealRepo is a manual smoke test — set REPOMETA_SMOKE_PATH
// to any real repository path to see what repometa reports for it.
// Skipped by default so it never fails in CI.
func TestSmokeRealRepo(t *testing.T) {
	path := os.Getenv("REPOMETA_SMOKE_PATH")
	if path == "" {
		t.Skip("set REPOMETA_SMOKE_PATH to run")
	}
	m, err := Scan(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("scanned %s — %d components, dirs visited=%d, files seen=%d",
		m.Root, len(m.Components), m.Stats.DirsVisited, m.Stats.FilesSeen)
	for _, c := range m.Components {
		t.Logf("  %-16s %s  attrs=%v", c.Kind, c.Root, c.Attributes)
		for _, ws := range c.Workspaces {
			t.Logf("    workspace: %s members=%v", ws.Kind, ws.Members)
		}
	}
}

func TestScanRejectsBadRoot(t *testing.T) {
	if _, err := Scan(""); err == nil {
		t.Errorf("expected error for empty root")
	}
	tmp := filepath.Join(t.TempDir(), "regular-file")
	if err := os.WriteFile(tmp, []byte("hi"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Scan(tmp); err == nil {
		t.Errorf("expected error when root is not a directory")
	}
}
