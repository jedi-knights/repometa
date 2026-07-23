package repometa

import (
	"testing"
)

// ---- Maven ----

func TestScanDetectsMavenSingleModule(t *testing.T) {
	// Arrange.
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"pom.xml": `<?xml version="1.0"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>app</artifactId>
  <version>1.0.0</version>
</project>`,
	})

	// Act.
	m, err := Scan(root)

	// Assert.
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	c := findComponent(m.Components, ".", KindJavaProject)
	if c == nil {
		t.Fatalf("expected java-project at root")
	}
	if c.Attributes["java.build"] != "maven" {
		t.Errorf("java.build: got %q want maven", c.Attributes["java.build"])
	}
	if len(c.Workspaces) != 0 {
		t.Errorf("single-module pom should not produce a workspace, got %+v", c.Workspaces)
	}
}

func TestScanDetectsMavenMultiModule(t *testing.T) {
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"pom.xml": `<?xml version="1.0"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>root</artifactId>
  <version>1.0.0</version>
  <packaging>pom</packaging>
  <modules>
    <module>services/api</module>
    <module>services/worker</module>
  </modules>
</project>`,
		"services/api/pom.xml":    `<project><artifactId>api</artifactId></project>`,
		"services/worker/pom.xml": `<project><artifactId>worker</artifactId></project>`,
	})
	m, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	c := findComponent(m.Components, ".", KindJavaProject)
	if c == nil {
		t.Fatalf("expected java-project at root")
	}
	if len(c.Workspaces) != 1 || c.Workspaces[0].Kind != WorkspaceMavenMultiModule {
		t.Fatalf("expected maven-multi-module workspace, got %+v", c.Workspaces)
	}
	got := map[string]bool{}
	for _, m := range c.Workspaces[0].Members {
		got[m] = true
	}
	for _, want := range []string{"services/api", "services/worker"} {
		if !got[want] {
			t.Errorf("missing maven module %q; got %v", want, c.Workspaces[0].Members)
		}
	}
	if findComponent(m.Components, "services/api", KindJavaProject) == nil {
		t.Errorf("expected java-project at services/api")
	}
	if findComponent(m.Components, "services/worker", KindJavaProject) == nil {
		t.Errorf("expected java-project at services/worker")
	}
}

func TestScanHandlesMalformedPom(t *testing.T) {
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"pom.xml": `<project><modules><module>a</module`, // truncated
	})
	m, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	c := findComponent(m.Components, ".", KindJavaProject)
	if c == nil {
		t.Fatalf("expected java-project even with malformed pom.xml")
	}
	if c.Confidence >= 1.0 {
		t.Errorf("expected confidence < 1.0 on parse error, got %v", c.Confidence)
	}
}

// ---- Gradle ----

func TestScanDetectsGradleGroovyDSL(t *testing.T) {
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"build.gradle": "plugins { id 'java' }\n",
	})
	m, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	c := findComponent(m.Components, ".", KindJavaProject)
	if c == nil {
		t.Fatalf("expected java-project at root")
	}
	if c.Attributes["java.build"] != "gradle" {
		t.Errorf("java.build: got %q want gradle", c.Attributes["java.build"])
	}
	if c.Attributes["java.gradle.dsl"] != "groovy" {
		t.Errorf("java.gradle.dsl: got %q want groovy", c.Attributes["java.gradle.dsl"])
	}
}

func TestScanDetectsGradleKotlinDSL(t *testing.T) {
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"build.gradle.kts": `plugins { java }`,
	})
	m, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	c := findComponent(m.Components, ".", KindJavaProject)
	if c == nil {
		t.Fatalf("expected java-project at root")
	}
	if c.Attributes["java.gradle.dsl"] != "kotlin" {
		t.Errorf("java.gradle.dsl: got %q want kotlin", c.Attributes["java.gradle.dsl"])
	}
}

func TestScanDetectsGradleMultiProject(t *testing.T) {
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"settings.gradle": `rootProject.name = 'root'
include 'app', 'lib'
include ':services:api'
include(":services:worker")
`,
		"build.gradle":                 "plugins { id 'java' }\n",
		"app/build.gradle":             "plugins { id 'java' }\n",
		"lib/build.gradle":             "plugins { id 'java' }\n",
		"services/api/build.gradle":    "plugins { id 'java' }\n",
		"services/worker/build.gradle": "plugins { id 'java' }\n",
	})
	m, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	root_ := findComponent(m.Components, ".", KindJavaProject)
	if root_ == nil {
		t.Fatalf("expected java-project at root")
	}
	if len(root_.Workspaces) != 1 || root_.Workspaces[0].Kind != WorkspaceGradleMultiProject {
		t.Fatalf("expected gradle-multi-project workspace, got %+v", root_.Workspaces)
	}
	got := map[string]bool{}
	for _, m := range root_.Workspaces[0].Members {
		got[m] = true
	}
	for _, want := range []string{"app", "lib", "services/api", "services/worker"} {
		if !got[want] {
			t.Errorf("missing gradle include %q; got %v", want, root_.Workspaces[0].Members)
		}
	}
}

func TestScanEmitsEmptyGradleWorkspaceWhenNoIncludes(t *testing.T) {
	// A settings file with no include directives still marks the
	// directory as a Gradle root — the workspace is emitted with no
	// members so consumers can distinguish "root without children" from
	// "not a root at all".
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"settings.gradle": `rootProject.name = 'root'`,
	})
	m, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	c := findComponent(m.Components, ".", KindJavaProject)
	if c == nil {
		t.Fatalf("expected java-project at root")
	}
	if len(c.Workspaces) != 1 || c.Workspaces[0].Kind != WorkspaceGradleMultiProject {
		t.Errorf("expected empty gradle-multi-project workspace, got %+v", c.Workspaces)
	}
	if len(c.Workspaces[0].Members) != 0 {
		t.Errorf("expected 0 members, got %v", c.Workspaces[0].Members)
	}
}

// ---- Ant ----

func TestScanDetectsAntBuild(t *testing.T) {
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"build.xml": `<?xml version="1.0"?>
<project name="legacy" default="compile">
  <target name="compile">
    <javac srcdir="src" destdir="build"/>
  </target>
</project>`,
	})
	m, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	c := findComponent(m.Components, ".", KindJavaProject)
	if c == nil {
		t.Fatalf("expected java-project at root")
	}
	if c.Attributes["java.build"] != "ant" {
		t.Errorf("java.build: got %q want ant", c.Attributes["java.build"])
	}
}

func TestScanDetectsAntWithIvy(t *testing.T) {
	root := t.TempDir()
	buildFixture(t, root, map[string]string{
		"build.xml": `<project name="p" default="c"><target name="c"/></project>`,
		"ivy.xml":   `<ivy-module version="2.0"><info organisation="ex" module="p"/></ivy-module>`,
	})
	m, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	c := findComponent(m.Components, ".", KindJavaProject)
	if c == nil {
		t.Fatalf("expected java-project at root")
	}
	sawIvy := false
	for _, e := range c.Evidence {
		if e.Reason == "ivy.xml present (Ivy dependency descriptor)" {
			sawIvy = true
		}
	}
	if !sawIvy {
		t.Errorf("expected ivy.xml evidence, got %+v", c.Evidence)
	}
}

// ---- parseGradleIncludes: direct unit tests for edge cases ----

func TestParseGradleIncludesReturnsNilOnRead(t *testing.T) {
	if got := parseGradleIncludes("/no/such/settings.gradle", defaultOptions()); got != nil {
		t.Errorf("expected nil on missing file, got %v", got)
	}
}
