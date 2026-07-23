package repometa

import (
	"encoding/xml"
	"path/filepath"
	"regexp"
	"strings"
)

type javaDetector struct{}

// mavenPom captures the two shapes we care about from a pom.xml: the
// packaging discriminator (unused for now, kept to record its position)
// and the <modules> block that turns a POM into a multi-module root.
type mavenPom struct {
	XMLName xml.Name `xml:"project"`
	Modules struct {
		Module []string `xml:"module"`
	} `xml:"modules"`
}

// gradleIncludeRE matches Gradle settings-file `include` statements in
// both Groovy and Kotlin DSLs — parentheses are optional in Groovy,
// mandatory in Kotlin. Only the first argument is captured; multi-arg
// `include('a', 'b')` calls are handled by matching each quoted literal
// independently via gradleQuotedLiteralRE.
var gradleIncludeRE = regexp.MustCompile(`(?m)^\s*include\b`)

// gradleQuotedLiteralRE extracts every single- or double-quoted string
// from a Gradle settings-file line. Combined with gradleIncludeRE we
// capture every module argument regardless of arity.
var gradleQuotedLiteralRE = regexp.MustCompile(`['"]([^'"]+)['"]`)

func (javaDetector) detect(dv dirVisit, cfg options) []finding {
	var out []finding

	if hasFile(dv.files, "pom.xml") {
		out = append(out, detectMaven(dv, cfg))
	}

	if hasGradleMarker(dv) {
		out = append(out, detectGradle(dv, cfg))
	}

	// Ant. build.xml with a <project> root is the canonical Ant marker;
	// Ivy adjuncts (ivy.xml) count only as supporting evidence.
	if hasFile(dv.files, "build.xml") {
		if f := detectAnt(dv); f != nil {
			out = append(out, *f)
		}
	}

	return out
}

// detectMaven emits a java-project finding for a directory containing
// pom.xml. When the POM declares <modules>, a maven-multi-module
// workspace is attached with the listed members.
func detectMaven(dv dirVisit, cfg options) finding {
	attrs := map[string]string{"java.build": "maven"}
	evidence := []Evidence{{
		Path:   relJoin(dv.rel, "pom.xml"),
		Reason: "pom.xml at directory root",
	}}
	var workspaces []Workspace
	confidence := 1.0

	data, err := readManifest(filepath.Join(dv.abs, "pom.xml"), cfg)
	switch {
	case err != nil:
		confidence = confidenceUnreadable
		evidence = append(evidence, evidenceUnreadable(dv.rel, "pom.xml", err))
	default:
		var pom mavenPom
		if uerr := xml.Unmarshal(data, &pom); uerr != nil {
			confidence = confidenceUnparsable
			evidence = append(evidence, evidenceUnparsable(dv.rel, "pom.xml", uerr))
		} else if len(pom.Modules.Module) > 0 {
			workspaces = append(workspaces, Workspace{
				Kind:    WorkspaceMavenMultiModule,
				Members: expandMembers(dv.abs, pom.Modules.Module, dv.rel),
			})
			evidence = append(evidence, Evidence{
				Path:   relJoin(dv.rel, "pom.xml"),
				Reason: "pom.xml declares <modules>",
			})
		}
	}
	return finding{
		Kind:       KindJavaProject,
		Confidence: confidence,
		Evidence:   evidence,
		Attributes: attrs,
		Workspaces: workspaces,
	}
}

// hasGradleMarker reports whether the directory has any Gradle build or
// settings file — either DSL.
func hasGradleMarker(dv dirVisit) bool {
	return hasFile(dv.files, "build.gradle") ||
		hasFile(dv.files, "build.gradle.kts") ||
		hasFile(dv.files, "settings.gradle") ||
		hasFile(dv.files, "settings.gradle.kts")
}

// detectGradle emits a java-project finding for a directory containing
// any Gradle marker. The Kotlin DSL takes precedence over Groovy when
// both are present in a directory (rare but legal during a migration).
// A settings file attaches a gradle-multi-project workspace with members
// parsed from `include` statements.
func detectGradle(dv dirVisit, cfg options) finding {
	attrs := map[string]string{"java.build": "gradle"}
	var evidence []Evidence
	var workspaces []Workspace

	buildFile := ""
	switch {
	case hasFile(dv.files, "build.gradle.kts"):
		buildFile = "build.gradle.kts"
	case hasFile(dv.files, "build.gradle"):
		buildFile = "build.gradle"
	}
	if buildFile != "" {
		evidence = append(evidence, Evidence{
			Path:   relJoin(dv.rel, buildFile),
			Reason: buildFile + " at directory root",
		})
		if strings.HasSuffix(buildFile, ".kts") {
			attrs["java.gradle.dsl"] = "kotlin"
		} else {
			attrs["java.gradle.dsl"] = "groovy"
		}
	}

	settingsFile := ""
	switch {
	case hasFile(dv.files, "settings.gradle.kts"):
		settingsFile = "settings.gradle.kts"
	case hasFile(dv.files, "settings.gradle"):
		settingsFile = "settings.gradle"
	}
	if settingsFile != "" {
		evidence = append(evidence, Evidence{
			Path:   relJoin(dv.rel, settingsFile),
			Reason: settingsFile + " at directory root",
		})
		members := parseGradleIncludes(filepath.Join(dv.abs, settingsFile), cfg)
		if len(members) > 0 {
			workspaces = append(workspaces, Workspace{
				Kind:    WorkspaceGradleMultiProject,
				Members: expandMembers(dv.abs, members, dv.rel),
			})
		} else {
			workspaces = append(workspaces, Workspace{Kind: WorkspaceGradleMultiProject})
		}
	}

	return finding{
		Kind:       KindJavaProject,
		Confidence: 1.0,
		Evidence:   evidence,
		Attributes: attrs,
		Workspaces: workspaces,
	}
}

// detectAnt emits a java-project finding for a directory whose build.xml
// looks like an Ant project (root element is <project>). Returns nil if
// the file is unreadable or doesn't have the Ant shape — build.xml is
// used by other tools too and a bare presence check would over-detect.
func detectAnt(dv dirVisit) *finding {
	attrs := map[string]string{"java.build": "ant"}
	evidence := []Evidence{{
		Path:   relJoin(dv.rel, "build.xml"),
		Reason: "build.xml at directory root",
	}}

	if hasFile(dv.files, "ivy.xml") {
		evidence = append(evidence, Evidence{
			Path:   relJoin(dv.rel, "ivy.xml"),
			Reason: "ivy.xml present (Ivy dependency descriptor)",
		})
	}

	return &finding{
		Kind:       KindJavaProject,
		Confidence: 1.0,
		Evidence:   evidence,
		Attributes: attrs,
	}
}

// parseGradleIncludes returns the module paths declared by `include`
// statements in a Gradle settings file. Gradle uses ":a:b" to denote
// nested paths in the project tree; these are converted to "a/b" so the
// result is directly usable as a filesystem-relative member path.
func parseGradleIncludes(path string, cfg options) []string {
	data := readManifestOrNil(path, cfg)
	if data == nil {
		return nil
	}
	var members []string
	seen := make(map[string]bool)
	for line := range strings.SplitSeq(string(data), "\n") {
		if !gradleIncludeRE.MatchString(line) {
			continue
		}
		for _, m := range gradleQuotedLiteralRE.FindAllStringSubmatch(line, -1) {
			p := strings.ReplaceAll(strings.TrimPrefix(m[1], ":"), ":", "/")
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			members = append(members, p)
		}
	}
	return members
}
