package repometa

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- Option constructors ----

func TestWithMaxDepthCapsDescent(t *testing.T) {
	// Arrange — build a 3-level nested Go module tree.
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"go.mod":         "module a\n\ngo 1.24\n",
		"a/b/c/go.mod":   "module abc\n\ngo 1.24\n",
		"a/b/c/main.go":  "package main\n",
		"a/b/c/d/go.mod": "module abcd\n\ngo 1.24\n",
	})

	// Act — cap depth so a/b/c and below are never visited.
	m, err := Scan(root, WithMaxDepth(2))

	// Assert.
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if m.Stats.DepthCapHits == 0 {
		t.Errorf("expected DepthCapHits > 0, got 0")
	}
	if findComponent(m.Components, "a/b/c", KindGoModule) != nil {
		t.Errorf("component at depth 3 should have been skipped by WithMaxDepth(2)")
	}
	if findComponent(m.Components, ".", KindGoModule) == nil {
		t.Errorf("root component should still be reported")
	}
}

func TestWithMaxDirsCapsWalk(t *testing.T) {
	// Arrange — many sibling directories.
	root := t.TempDir()
	files := map[string]string{"go.mod": "module a\n\ngo 1.24\n"}
	for i := range 20 {
		files[filepath.Join("dir", string(rune('a'+i)), "keep.txt")] = "x"
	}
	buildFixture(t, root, files)

	// Act — cap directory count aggressively.
	m, err := Scan(root, WithMaxDirs(3))

	// Assert.
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if m.Stats.DirCapHits == 0 {
		t.Errorf("expected DirCapHits > 0, got 0")
	}
	if m.Stats.DirsVisited > 4 {
		t.Errorf("DirsVisited=%d exceeds cap of 3 by more than one iteration", m.Stats.DirsVisited)
	}
}

func TestWithMaxFileSizeMakesManifestUnreadable(t *testing.T) {
	// Arrange — a package.json larger than the tiny cap we'll set.
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"package.json": `{"name":"pad","dependencies":{"next":"14.0.0"},"padding":"` +
			strings.Repeat("x", 512) + `"}`,
	})

	// Act — cap file size well below the manifest.
	m, err := Scan(root, WithMaxFileSize(16))

	// Assert — the component is still reported but with reduced confidence
	// and evidence noting the size cap.
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	pkg := findComponent(m.Components, ".", KindNodePackage)
	if pkg == nil {
		t.Fatalf("expected node-package even with unreadable manifest")
	}
	if pkg.Confidence >= 1.0 {
		t.Errorf("confidence should drop below 1.0, got %v", pkg.Confidence)
	}
	sawCapEvidence := false
	for _, e := range pkg.Evidence {
		if strings.Contains(e.Reason, "unreadable") || strings.Contains(e.Reason, "exceeds cap") {
			sawCapEvidence = true
		}
	}
	if !sawCapEvidence {
		t.Errorf("expected evidence noting unreadable/size cap, got %+v", pkg.Evidence)
	}
}

// ---- Scan error branches ----

func TestScanRejectsMissingRoot(t *testing.T) {
	// Act — path that does not exist.
	_, err := Scan(filepath.Join(t.TempDir(), "does-not-exist"))

	// Assert.
	if err == nil {
		t.Errorf("expected error for missing root")
	}
}

// ---- Node detector: pnpm, angular, malformed JSON, yarn object workspaces, nx, turbo ----

func TestScanDetectsPnpmWorkspace(t *testing.T) {
	// Arrange.
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"package.json":            `{"name":"root","private":true}`,
		"pnpm-workspace.yaml":     "packages:\n  - packages/*\n",
		"packages/a/package.json": `{"name":"a"}`,
		"packages/b/package.json": `{"name":"b"}`,
	})

	// Act.
	m, err := Scan(root)

	// Assert.
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	pkg := findComponent(m.Components, ".", KindNodePackage)
	if pkg == nil {
		t.Fatalf("expected node-package at root")
	}
	var pnpm *Workspace
	for i := range pkg.Workspaces {
		if pkg.Workspaces[i].Kind == WorkspacePnpm {
			pnpm = &pkg.Workspaces[i]
		}
	}
	if pnpm == nil {
		t.Fatalf("expected pnpm workspace on root, got %+v", pkg.Workspaces)
	}
	if len(pnpm.Members) != 2 {
		t.Errorf("expected 2 pnpm members, got %v", pnpm.Members)
	}
}

func TestScanDetectsAngularViaAngularJSON(t *testing.T) {
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"package.json": `{"name":"app"}`,
		"angular.json": `{"version":1}`,
	})
	m, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	pkg := findComponent(m.Components, ".", KindNodePackage)
	if pkg == nil {
		t.Fatalf("expected node-package at root")
	}
	if pkg.Attributes["js.framework"] != "angular" {
		t.Errorf("js.framework: got %q, want angular", pkg.Attributes["js.framework"])
	}
}

func TestDetectJSFrameworkCoversAllBranches(t *testing.T) {
	// Table-drives the four dependency/devDependency × next/@angular/core
	// paths in detectJSFramework.
	tests := []struct {
		name string
		pkg  packageJSON
		want string
	}{
		{"next in deps", packageJSON{Dependencies: map[string]string{"next": "14"}}, "nextjs"},
		{"next in devDeps", packageJSON{DevDependencies: map[string]string{"next": "14"}}, "nextjs"},
		{"angular in deps", packageJSON{Dependencies: map[string]string{"@angular/core": "18"}}, "angular"},
		{"angular in devDeps", packageJSON{DevDependencies: map[string]string{"@angular/core": "18"}}, "angular"},
		{"unknown", packageJSON{Dependencies: map[string]string{"react": "18"}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectJSFramework(tt.pkg); got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestScanHandlesMalformedPackageJSON(t *testing.T) {
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"package.json": `{ this is not valid json`,
	})
	m, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	pkg := findComponent(m.Components, ".", KindNodePackage)
	if pkg == nil {
		t.Fatalf("expected node-package even when package.json is malformed")
	}
	if pkg.Confidence >= 1.0 {
		t.Errorf("expected confidence < 1.0 on parse error, got %v", pkg.Confidence)
	}
	sawParseError := false
	for _, e := range pkg.Evidence {
		if strings.Contains(e.Reason, "parse error") {
			sawParseError = true
		}
	}
	if !sawParseError {
		t.Errorf("expected evidence citing parse error, got %+v", pkg.Evidence)
	}
}

func TestParseNpmYarnWorkspacesObjectShape(t *testing.T) {
	// Yarn's alternate workspaces shape is an object with a "packages"
	// array — the array form is already covered by the integration test.
	raw := json.RawMessage(`{"packages": ["a/*", "b/*"], "nohoist": ["**/foo"]}`)
	members, ok := parseNpmYarnWorkspaces(raw)
	if !ok || len(members) != 2 {
		t.Fatalf("expected 2 members from object shape, got %v ok=%v", members, ok)
	}
}

func TestParseNpmYarnWorkspacesRejectsGarbage(t *testing.T) {
	if _, ok := parseNpmYarnWorkspaces(json.RawMessage(`42`)); ok {
		t.Errorf("expected ok=false on garbage input")
	}
	if _, ok := parseNpmYarnWorkspaces(nil); ok {
		t.Errorf("expected ok=false on nil input")
	}
}

func TestScanDetectsNxAndTurborepo(t *testing.T) {
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"package.json": `{"name":"root"}`,
		"nx.json":      `{"npmScope":"acme"}`,
		"turbo.json":   `{"pipeline":{}}`,
	})
	m, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	pkg := findComponent(m.Components, ".", KindNodePackage)
	if pkg == nil {
		t.Fatalf("expected node-package at root")
	}
	kinds := map[WorkspaceKind]bool{}
	for _, ws := range pkg.Workspaces {
		kinds[ws.Kind] = true
	}
	if !kinds[WorkspaceNx] {
		t.Errorf("expected nx workspace, got %+v", pkg.Workspaces)
	}
	if !kinds[WorkspaceTurborepo] {
		t.Errorf("expected turborepo workspace, got %+v", pkg.Workspaces)
	}
}

// ---- Python detector: package-manager attribution + poetry + parse error ----

func TestDetectPythonPMAllVariants(t *testing.T) {
	// Each fixture isolates a single package-manager signal so the
	// detectPythonPM switch/case order is exercised end to end.
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name: "uv",
			files: map[string]string{
				"pyproject.toml": `[project]` + "\n" + `name = "p"` + "\n",
				"uv.lock":        "x",
			},
			want: "uv",
		},
		{
			name: "poetry via lock file",
			files: map[string]string{
				"pyproject.toml": `[project]` + "\n" + `name = "p"` + "\n",
				"poetry.lock":    "x",
			},
			want: "poetry",
		},
		{
			name: "pipenv via Pipfile",
			files: map[string]string{
				"Pipfile": "[[source]]\nname = \"pypi\"\n",
			},
			want: "pipenv",
		},
		{
			name: "pip via requirements.txt",
			files: map[string]string{
				"requirements.txt": "requests==2.31\n",
			},
			want: "pip",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			buildFixture(t, root, tt.files)
			m, err := Scan(root)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			p := findComponent(m.Components, ".", KindPythonPackage)
			if p == nil {
				t.Fatalf("expected python-package at root")
			}
			if p.Attributes["python.pm"] != tt.want {
				t.Errorf("python.pm: got %q want %q", p.Attributes["python.pm"], tt.want)
			}
		})
	}
}

func TestScanDetectsPoetryFromToolTable(t *testing.T) {
	// Poetry attribution when the marker is [tool.poetry] in pyproject
	// rather than a poetry.lock file.
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"pyproject.toml": "[project]\nname = \"p\"\n\n[tool.poetry]\nversion = \"0.1.0\"\n",
	})
	m, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	p := findComponent(m.Components, ".", KindPythonPackage)
	if p == nil {
		t.Fatalf("expected python-package at root")
	}
	if p.Attributes["python.pm"] != "poetry" {
		t.Errorf("python.pm: got %q want poetry", p.Attributes["python.pm"])
	}
}

func TestScanHandlesMalformedPyproject(t *testing.T) {
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"pyproject.toml": `[project` + "\n" + `name = broken`,
	})
	m, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	p := findComponent(m.Components, ".", KindPythonPackage)
	if p == nil {
		t.Fatalf("expected python-package even with malformed toml")
	}
	if p.Confidence >= 1.0 {
		t.Errorf("expected confidence < 1.0, got %v", p.Confidence)
	}
}

// ---- Go detector: single-line use, comments in go.work ----

func TestParseGoWorkUseHandlesSingleLineAndComments(t *testing.T) {
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		// Mix of single-line 'use ./mod' and block form, with an
		// end-of-line comment and a full-line comment inside the block.
		"go.work": `go 1.24

use ./svc

// leading comment ignored
use (
	./a  // trailing comment
	./b
)
`,
		"svc/go.mod": "module svc\n\ngo 1.24\n",
		"a/go.mod":   "module a\n\ngo 1.24\n",
		"b/go.mod":   "module b\n\ngo 1.24\n",
	})
	m, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	root_ := findComponent(m.Components, ".", KindGoModule)
	if root_ == nil {
		t.Fatalf("expected go-module at root; components=%+v", m.Components)
	}
	if len(root_.Workspaces) != 1 || root_.Workspaces[0].Kind != WorkspaceGo {
		t.Fatalf("expected go workspace, got %+v", root_.Workspaces)
	}
	got := map[string]bool{}
	for _, m := range root_.Workspaces[0].Members {
		got[m] = true
	}
	for _, want := range []string{"svc", "a", "b"} {
		if !got[want] {
			t.Errorf("missing go.work member %q; got %v", want, root_.Workspaces[0].Members)
		}
	}
}

// ---- expandMembers: double-star and no-match ----

func TestExpandMembersDropsDoubleStarAndEmitsUnmatched(t *testing.T) {
	base := t.TempDir()
	// Only 'a' exists; 'ghost' does not.
	if err := os.MkdirAll(filepath.Join(base, "packages", "a"), 0o755); err != nil {
		t.Fatal(err)
	}

	out := expandMembers(base, []string{
		"packages/*",  // resolves to packages/a
		"packages/**", // dropped: double-star unsupported
		"ghost",       // no glob, no match — emitted as-is
		"",            // ignored (empty)
	}, "root")

	seen := map[string]bool{}
	for _, m := range out {
		seen[m] = true
	}
	if !seen["root/packages/a"] {
		t.Errorf("expected root/packages/a in %v", out)
	}
	if !seen["root/ghost"] {
		t.Errorf("expected literal root/ghost preserved when no glob matched, got %v", out)
	}
	for _, m := range out {
		if strings.Contains(m, "**") {
			t.Errorf("double-star pattern should have been dropped, got %v", out)
		}
	}
}

// ---- readManifest: nonexistent path and size cap ----

func TestReadManifestSurfacesStatAndSizeErrors(t *testing.T) {
	// stat failure
	if _, err := readManifest("/no/such/path", defaultOptions()); err == nil {
		t.Errorf("expected stat error on nonexistent path")
	}

	// size cap
	dir := t.TempDir()
	path := filepath.Join(dir, "big")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 128)), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := defaultOptions()
	cfg.maxFileSize = 16
	if _, err := readManifest(path, cfg); err == nil {
		t.Errorf("expected size-cap error")
	}
}

// ---- walker: symlinked directory is skipped ----

func TestWalkerSkipsSymlinkedDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "real", "go.mod"),
		[]byte("module r\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create a symlink child pointing to the real dir.
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink not supported on this platform: %v", err)
	}

	m, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if m.Stats.SymlinksSkipped == 0 {
		t.Errorf("expected SymlinksSkipped > 0, got 0")
	}
	if findComponent(m.Components, "link", KindGoModule) != nil {
		t.Errorf("component under symlink should not be reported")
	}
}

// ---- sortComponents: tie-break on kind at the same root ----

func TestSortComponentsTieBreaksOnKind(t *testing.T) {
	cs := []Component{
		{Root: "b", Kind: KindGoModule},
		{Root: "a", Kind: KindNodePackage},
		{Root: "a", Kind: KindGoModule},
	}
	sortComponents(cs)
	// Expected order: (a, go-module), (a, node-package), (b, go-module).
	if cs[0].Root != "a" || cs[0].Kind != KindGoModule {
		t.Errorf("first: got (%s, %s)", cs[0].Root, cs[0].Kind)
	}
	if cs[1].Root != "a" || cs[1].Kind != KindNodePackage {
		t.Errorf("second: got (%s, %s)", cs[1].Root, cs[1].Kind)
	}
	if cs[2].Root != "b" {
		t.Errorf("third: got (%s, %s)", cs[2].Root, cs[2].Kind)
	}
}
