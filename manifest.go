package repometa

import "slices"

// Manifest is the result of scanning a repository. Consumers iterate
// [Manifest.Components]; the ordering is deterministic (by root path,
// then by kind) so serialized output is diff-friendly.
type Manifest struct {
	// Root is the absolute path that was scanned. All [Component.Root]
	// and [Evidence.Path] values in this manifest are relative to Root.
	Root string

	// Components lists every buildable unit detected under Root. May be
	// empty if the scan encountered no known ecosystem manifests.
	Components []Component

	// Stats reports counters from the walk. Non-zero cap-hit fields
	// indicate the scan was truncated and may be incomplete.
	Stats ScanStats
}

// Kind identifies the type of a [Component]. Values are open-ended
// strings so new detectors can be added without breaking consumers on
// enum drift; unknown values are safe to log or ignore.
type Kind string

// Kind values for every ecosystem detector shipped in this package. See
// the README's "Supported detectors" table for the mapping to ecosystem.
const (
	KindGoModule       Kind = "go-module"
	KindRustCrate      Kind = "rust-crate"
	KindRustWorkspace  Kind = "rust-workspace"
	KindPythonPackage  Kind = "python-package"
	KindNodePackage    Kind = "node-package"
	KindDotNetProject  Kind = "dotnet-project"
	KindDotNetSolution Kind = "dotnet-solution"
	KindCppProject     Kind = "cpp-project"
	KindJavaProject    Kind = "java-project"
	KindCMakeProject   Kind = "cmake-project"
	KindMakeProject    Kind = "make-project"
	KindCSource        Kind = "c-source-tree"
	KindAsmSource      Kind = "asm-source-tree"
)

// Component describes a single buildable unit inside the repo, anchored
// at a specific directory. A directory may produce multiple Components
// when more than one ecosystem's manifest is present (e.g. a Rust crate
// with a co-located Node package).
type Component struct {
	// Kind identifies the ecosystem and shape of the component.
	Kind Kind

	// Root is the path relative to [Manifest.Root]; "." for the repo root.
	Root string

	// Evidence lists the files that led to this component being reported.
	Evidence []Evidence

	// Confidence is in [0.0, 1.0]; heuristic hits report < 1.0, and
	// manifest-driven detections report 1.0.
	Confidence float64

	// Workspaces lists any monorepo layouts anchored at this component.
	Workspaces []Workspace

	// Attributes carries ecosystem-specific metadata. Keys are namespaced
	// (e.g. "js.framework", "python.pm"). See the README for the current
	// set; unknown keys are safe to ignore.
	Attributes map[string]string
}

// Evidence records a single fact that led to a [Component] being
// reported. Path is relative to [Manifest.Root] so evidence remains
// meaningful when the manifest is transported.
type Evidence struct {
	// Path is relative to [Manifest.Root]; forward slashes on every
	// platform.
	Path string

	// Reason is a short human-readable description of why this file
	// counts as evidence (e.g. "go.mod present", "workspace = [...]").
	Reason string
}

// Workspace describes a monorepo layout anchored at a [Component].
// Members may be empty when the workspace kind was detected by file
// presence alone rather than by parsing an explicit member list.
type Workspace struct {
	// Kind identifies the monorepo tooling.
	Kind WorkspaceKind

	// Members lists the workspace's constituent package paths, relative
	// to [Manifest.Root], forward-slash separated.
	Members []string
}

// WorkspaceKind identifies the workspace / monorepo tooling in use. Like
// [Kind], values are open-ended strings — unknown values are safe to
// log or ignore.
type WorkspaceKind string

// WorkspaceKind values for every workspace layout recognized by the
// detectors in this package.
const (
	WorkspaceGo                 WorkspaceKind = "go-workspace"
	WorkspaceCargo              WorkspaceKind = "cargo-workspace"
	WorkspaceNpmYarn            WorkspaceKind = "npm-yarn-workspace"
	WorkspacePnpm               WorkspaceKind = "pnpm-workspace"
	WorkspaceNx                 WorkspaceKind = "nx"
	WorkspaceTurborepo          WorkspaceKind = "turborepo"
	WorkspaceUv                 WorkspaceKind = "uv-workspace"
	WorkspaceDotNetSolution     WorkspaceKind = "dotnet-solution"
	WorkspaceMavenMultiModule   WorkspaceKind = "maven-multi-module"
	WorkspaceGradleMultiProject WorkspaceKind = "gradle-multi-project"
)

// Language is a coarse ecosystem label derived from a [Component.Kind].
// Multiple Kinds may map to the same Language: [KindRustCrate] and
// [KindRustWorkspace] both report [LanguageRust]; [KindCMakeProject],
// [KindMakeProject], and [KindCSource] all report [LanguageC];
// [KindDotNetProject] and [KindDotNetSolution] both report
// [LanguageDotNet]. This grouping is used by the helpers on [Manifest]
// to distinguish single-language repositories from polyglot ones —
// treat Component.Kind as the source of truth for finer-grained work.
type Language string

// Language values. LanguageUnknown is used when a Component.Kind is not
// in the mapping table (added by a future detector; safely reported
// rather than dropped).
const (
	LanguageGo         Language = "go"
	LanguageRust       Language = "rust"
	LanguagePython     Language = "python"
	LanguageJavaScript Language = "javascript"
	LanguageDotNet     Language = "dotnet"
	LanguageJava       Language = "java"
	LanguageC          Language = "c"
	LanguageAssembly   Language = "assembly"
	LanguageUnknown    Language = "unknown"
)

// Language returns the coarse ecosystem label for this Component. See
// [Language] for the mapping and rationale; unrecognized kinds return
// [LanguageUnknown].
func (c Component) Language() Language {
	switch c.Kind {
	case KindGoModule:
		return LanguageGo
	case KindRustCrate, KindRustWorkspace:
		return LanguageRust
	case KindPythonPackage:
		return LanguagePython
	case KindNodePackage:
		return LanguageJavaScript
	case KindDotNetProject, KindDotNetSolution:
		return LanguageDotNet
	case KindJavaProject:
		return LanguageJava
	case KindCMakeProject, KindMakeProject, KindCppProject, KindCSource:
		return LanguageC
	case KindAsmSource:
		return LanguageAssembly
	}
	return LanguageUnknown
}

// Languages returns the sorted, de-duplicated list of [Language] values
// present across every [Component] in the manifest. A manifest with no
// components returns an empty slice.
func (m *Manifest) Languages() []Language {
	seen := make(map[Language]struct{}, len(m.Components))
	for _, c := range m.Components {
		seen[c.Language()] = struct{}{}
	}
	out := make([]Language, 0, len(seen))
	for l := range seen {
		out = append(out, l)
	}
	slices.Sort(out)
	return out
}

// Polyglot reports whether the manifest contains components spanning
// more than one [Language]. A manifest with zero components, or with
// components in a single Language, returns false. [LanguageUnknown]
// counts as its own language, so a repo mixing a known ecosystem with
// one repometa doesn't yet recognize is reported as polyglot.
func (m *Manifest) Polyglot() bool {
	return len(m.Languages()) > 1
}

// ScanStats reports counters from the walk. Non-zero DepthCapHits or
// DirCapHits indicate the scan was truncated by [WithMaxDepth] or
// [WithMaxDirs] respectively — callers should widen the cap and rescan
// if a complete manifest is required.
type ScanStats struct {
	// DirsVisited is the total number of directories entered by the walk.
	DirsVisited int

	// FilesSeen is the total number of file entries observed (whether or
	// not their contents were read).
	FilesSeen int

	// DepthCapHits counts the number of directories the walk refused to
	// descend into because the [WithMaxDepth] cap was reached.
	DepthCapHits int

	// DirCapHits is 1 if the [WithMaxDirs] cap fired and aborted the walk;
	// 0 otherwise.
	DirCapHits int

	// SymlinksSkipped counts the number of symlink entries skipped —
	// symlinks are never traversed regardless of target.
	SymlinksSkipped int
}
