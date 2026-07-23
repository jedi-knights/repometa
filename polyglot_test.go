package repometa

import (
	"reflect"
	"testing"
)

func TestComponentLanguageMapping(t *testing.T) {
	tests := []struct {
		kind Kind
		want Language
	}{
		{KindGoModule, LanguageGo},
		{KindRustCrate, LanguageRust},
		{KindRustWorkspace, LanguageRust},
		{KindPythonPackage, LanguagePython},
		{KindNodePackage, LanguageJavaScript},
		{KindDotNetProject, LanguageDotNet},
		{KindDotNetSolution, LanguageDotNet},
		{KindJavaProject, LanguageJava},
		{KindCMakeProject, LanguageC},
		{KindMakeProject, LanguageC},
		{KindCppProject, LanguageC},
		{KindCSource, LanguageC},
		{KindAsmSource, LanguageAssembly},
		{Kind("something-nobody-added"), LanguageUnknown},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			c := Component{Kind: tt.kind}
			if got := c.Language(); got != tt.want {
				t.Errorf("Component{Kind: %q}.Language() = %q, want %q", tt.kind, got, tt.want)
			}
		})
	}
}

func TestManifestLanguagesAndPolyglot(t *testing.T) {
	tests := []struct {
		name       string
		components []Component
		wantLangs  []Language
		wantPoly   bool
	}{
		{
			name:       "empty manifest",
			components: nil,
			wantLangs:  []Language{},
			wantPoly:   false,
		},
		{
			name: "single-language Go",
			components: []Component{
				{Kind: KindGoModule, Root: "."},
			},
			wantLangs: []Language{LanguageGo},
			wantPoly:  false,
		},
		{
			name: "single-language Rust despite crate + workspace kinds",
			components: []Component{
				{Kind: KindRustWorkspace, Root: "."},
				{Kind: KindRustCrate, Root: "crates/a"},
				{Kind: KindRustCrate, Root: "crates/b"},
			},
			wantLangs: []Language{LanguageRust},
			wantPoly:  false,
		},
		{
			name: "single-language dotnet despite solution + project kinds",
			components: []Component{
				{Kind: KindDotNetSolution, Root: "."},
				{Kind: KindDotNetProject, Root: "src/App"},
				{Kind: KindDotNetProject, Root: "src/Lib"},
			},
			wantLangs: []Language{LanguageDotNet},
			wantPoly:  false,
		},
		{
			name: "single-language C via CMake + Make + source",
			components: []Component{
				{Kind: KindCMakeProject, Root: "."},
				{Kind: KindMakeProject, Root: "tools/legacy"},
				{Kind: KindCSource, Root: "experiments"},
			},
			wantLangs: []Language{LanguageC},
			wantPoly:  false,
		},
		{
			name: "polyglot Go + Python + JavaScript",
			components: []Component{
				{Kind: KindGoModule, Root: "services/api"},
				{Kind: KindPythonPackage, Root: "ml"},
				{Kind: KindNodePackage, Root: "web"},
			},
			wantLangs: []Language{LanguageGo, LanguageJavaScript, LanguagePython},
			wantPoly:  true,
		},
		{
			name: "polyglot Java + dotnet + assembly + unknown",
			components: []Component{
				{Kind: KindJavaProject, Root: "api"},
				{Kind: KindDotNetSolution, Root: "client"},
				{Kind: KindAsmSource, Root: "boot"},
				{Kind: Kind("future-detector-kind"), Root: "novel"},
			},
			wantLangs: []Language{LanguageAssembly, LanguageDotNet, LanguageJava, LanguageUnknown},
			wantPoly:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manifest{Components: tt.components}
			if got := m.Languages(); !reflect.DeepEqual(got, tt.wantLangs) {
				t.Errorf("Languages() = %v, want %v", got, tt.wantLangs)
			}
			if got := m.Polyglot(); got != tt.wantPoly {
				t.Errorf("Polyglot() = %v, want %v", got, tt.wantPoly)
			}
		})
	}
}

// TestScanReportsPolyglotOnMixedTree exercises the classifier through a
// live Scan of a realistic mixed-ecosystem tree, catching any regression
// in the Kind → Language mapping that a unit test alone would miss.
func TestScanReportsPolyglotOnMixedTree(t *testing.T) {
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"services/api/go.mod":   "module a\n\ngo 1.24\n",
		"web/package.json":      `{"name":"w"}`,
		"ml/pyproject.toml":     "[project]\nname = \"m\"\n",
		"legacy/build.gradle":   "plugins { id 'java' }\n",
		"native/CMakeLists.txt": "cmake_minimum_required(VERSION 3.10)\nproject(n)\n",
	})
	m, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !m.Polyglot() {
		t.Errorf("expected Polyglot=true; languages=%v", m.Languages())
	}
	got := map[Language]bool{}
	for _, l := range m.Languages() {
		got[l] = true
	}
	for _, want := range []Language{LanguageGo, LanguageJavaScript, LanguagePython, LanguageJava, LanguageC} {
		if !got[want] {
			t.Errorf("expected %q in Languages; got %v", want, m.Languages())
		}
	}
}
