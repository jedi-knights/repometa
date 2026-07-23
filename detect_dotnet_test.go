package repometa

import (
	"testing"
)

func TestScanDetectsCSharpProject(t *testing.T) {
	// Arrange.
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"App.csproj": `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup></Project>`,
	})

	// Act.
	m, err := Scan(root)

	// Assert.
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	c := findComponent(m.Components, ".", KindDotNetProject)
	if c == nil {
		t.Fatalf("expected dotnet-project at root; components=%+v", m.Components)
	}
	if c.Attributes["dotnet.language"] != "csharp" {
		t.Errorf("dotnet.language: got %q want csharp", c.Attributes["dotnet.language"])
	}
}

func TestScanDetectsFSharpAndVBProjects(t *testing.T) {
	tests := []struct {
		name string
		file string
		lang string
	}{
		{"fsharp", "Lib.fsproj", "fsharp"},
		{"vb", "Legacy.vbproj", "vb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange.
			root := t.TempDir()
			buildFixture(t, root, map[string]string{
				tt.file: `<Project Sdk="Microsoft.NET.Sdk"></Project>`,
			})

			// Act.
			m, err := Scan(root)

			// Assert.
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			c := findComponent(m.Components, ".", KindDotNetProject)
			if c == nil {
				t.Fatalf("expected dotnet-project at root; components=%+v", m.Components)
			}
			if c.Attributes["dotnet.language"] != tt.lang {
				t.Errorf("dotnet.language: got %q want %q", c.Attributes["dotnet.language"], tt.lang)
			}
		})
	}
}

func TestScanDetectsSolutionWithMembers(t *testing.T) {
	// Arrange — a canonical .sln listing two C# projects, a solution
	// folder (should be filtered), and a Windows-style backslash path
	// (should be normalized).
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"Root.sln": `Microsoft Visual Studio Solution File, Format Version 12.00
# Visual Studio Version 17
Project("{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}") = "App", "src\App\App.csproj", "{11111111-1111-1111-1111-111111111111}"
EndProject
Project("{F2A71F9B-5D33-465A-A702-920D77279786}") = "Lib", "src/Lib/Lib.fsproj", "{22222222-2222-2222-2222-222222222222}"
EndProject
Project("{2150E333-8FDC-42A3-9474-1A3956D46DE8}") = "SolutionFolder", "SolutionFolder", "{33333333-3333-3333-3333-333333333333}"
EndProject
Global
EndGlobal
`,
		"src/App/App.csproj": `<Project Sdk="Microsoft.NET.Sdk"></Project>`,
		"src/App/Program.cs": "class Program { static void Main() {} }",
		"src/Lib/Lib.fsproj": `<Project Sdk="Microsoft.NET.Sdk"></Project>`,
	})

	// Act.
	m, err := Scan(root)

	// Assert — solution component with the two projects as members.
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	sln := findComponent(m.Components, ".", KindDotNetSolution)
	if sln == nil {
		t.Fatalf("expected dotnet-solution at root; components=%+v", m.Components)
	}
	if len(sln.Workspaces) != 1 || sln.Workspaces[0].Kind != WorkspaceDotNetSolution {
		t.Fatalf("expected dotnet-solution workspace; got %+v", sln.Workspaces)
	}
	got := map[string]bool{}
	for _, mem := range sln.Workspaces[0].Members {
		got[mem] = true
	}
	for _, want := range []string{"src/App", "src/Lib"} {
		if !got[want] {
			t.Errorf("missing member %q; got %v", want, sln.Workspaces[0].Members)
		}
	}
	// Solution folders are filtered out (no .csproj/.fsproj/.vbproj ext).
	for m := range got {
		if m == "SolutionFolder" {
			t.Errorf("solution folder should be filtered; got members=%v", sln.Workspaces[0].Members)
		}
	}

	// Individual projects are still emitted as their own components.
	if findComponent(m.Components, "src/App", KindDotNetProject) == nil {
		t.Errorf("expected dotnet-project at src/App")
	}
	if findComponent(m.Components, "src/Lib", KindDotNetProject) == nil {
		t.Errorf("expected dotnet-project at src/Lib")
	}
}

func TestParseSlnMembersHandlesUnreadable(t *testing.T) {
	// Unreadable file returns nil rather than panicking.
	if got := parseSlnMembers("/no/such/file.sln", defaultOptions()); got != nil {
		t.Errorf("expected nil for missing file, got %v", got)
	}
}

func TestScanSkipsEmptySolutionEmission(t *testing.T) {
	// A .sln with no Project(...) lines is not a .NET workspace — no
	// dotnet-solution component should be emitted for it.
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"Empty.sln": "Microsoft Visual Studio Solution File, Format Version 12.00\nGlobal\nEndGlobal\n",
	})
	m, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if sln := findComponent(m.Components, ".", KindDotNetSolution); sln != nil {
		t.Errorf("empty .sln should not produce dotnet-solution, got %+v", sln)
	}
}

func TestScanDetectsVCXProject(t *testing.T) {
	// A .vcxproj declares a Visual Studio C++ project — surfaces as
	// KindCppProject, buckets into LanguageC in the polyglot classifier.
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"Native.vcxproj": `<?xml version="1.0"?><Project><ItemGroup><ClCompile Include="main.cpp"/></ItemGroup></Project>`,
		"main.cpp":       "int main(){return 0;}\n",
	})
	m, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	c := findComponent(m.Components, ".", KindCppProject)
	if c == nil {
		t.Fatalf("expected cpp-project at root; components=%+v", m.Components)
	}
	if c.Language() != LanguageC {
		t.Errorf("cpp-project should bucket to LanguageC, got %q", c.Language())
	}
	// Loose C source suppression should hide the c-source-tree finding
	// that would otherwise fire on main.cpp.
	if findComponent(m.Components, ".", KindCSource) != nil {
		t.Errorf("c-source-tree under cpp-project should have been suppressed")
	}
}

func TestScanCppOnlySolutionEmitsNoDotNetSolution(t *testing.T) {
	// Reproduces the VSNvim shape found in the live smoke test: a .sln
	// whose only Project(...) reference is a .vcxproj. The solution
	// itself is not a .NET workspace and should not emit dotnet-solution,
	// but the C++ project should surface as cpp-project.
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"Native.sln": `Microsoft Visual Studio Solution File, Format Version 12.00
Project("{8BC9CEB8-8B4A-11D0-8D11-00A0C91BC942}") = "Native", "Native\Native.vcxproj", "{11111111-1111-1111-1111-111111111111}"
EndProject
Global
EndGlobal
`,
		"Native/Native.vcxproj": `<?xml version="1.0"?><Project></Project>`,
	})
	m, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if sln := findComponent(m.Components, ".", KindDotNetSolution); sln != nil {
		t.Errorf("pure-C++ .sln should not emit dotnet-solution, got %+v", sln)
	}
	if cpp := findComponent(m.Components, "Native", KindCppProject); cpp == nil {
		t.Errorf("expected cpp-project at Native; got components=%+v", m.Components)
	}
}
