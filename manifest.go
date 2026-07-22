package repometa

// Manifest is the result of scanning a repository. Consumers are expected
// to iterate Components; the ordering is deterministic (depth-first,
// lexicographic within a directory) so diff-friendly serializations are
// possible.
type Manifest struct {
	Root       string
	Components []Component
	Stats      ScanStats
}

// Kind identifies the type of a Component. Values are open-ended strings
// so new detectors can be added without breaking consumers on enum drift.
type Kind string

// Kind values for every ecosystem detector shipped in this package. See
// the README's "Supported detectors" table for the mapping to ecosystem.
const (
	KindGoModule      Kind = "go-module"
	KindRustCrate     Kind = "rust-crate"
	KindRustWorkspace Kind = "rust-workspace"
	KindPythonPackage Kind = "python-package"
	KindNodePackage   Kind = "node-package"
	KindCMakeProject  Kind = "cmake-project"
	KindMakeProject   Kind = "make-project"
	KindCSource       Kind = "c-source-tree"
	KindAsmSource     Kind = "asm-source-tree"
)

// Component describes a single buildable unit inside the repo, anchored
// at a specific directory. A directory may produce multiple Components
// when more than one ecosystem's manifest is present (e.g. a Rust crate
// with a co-located Node package).
type Component struct {
	Kind       Kind
	Root       string // path relative to Manifest.Root; "." for the repo root
	Evidence   []Evidence
	Confidence float64 // 0.0 – 1.0; heuristic hits report < 1.0
	Workspaces []Workspace
	Attributes map[string]string
}

// Evidence records a single fact that led to a Component being reported.
// Path is relative to Manifest.Root so evidence remains meaningful when
// the manifest is transported.
type Evidence struct {
	Path   string
	Reason string
}

// Workspace describes a monorepo layout anchored at a Component. Members
// are paths (relative to Manifest.Root) of the workspace's constituent
// packages. Members may be empty when the workspace kind was detected by
// file presence alone (see docs on each WorkspaceKind).
type Workspace struct {
	Kind    WorkspaceKind
	Members []string
}

// WorkspaceKind identifies the workspace / monorepo tooling in use.
type WorkspaceKind string

// WorkspaceKind values for every workspace layout recognized by the
// detectors in this package.
const (
	WorkspaceGo        WorkspaceKind = "go-workspace"
	WorkspaceCargo     WorkspaceKind = "cargo-workspace"
	WorkspaceNpmYarn   WorkspaceKind = "npm-yarn-workspace"
	WorkspacePnpm      WorkspaceKind = "pnpm-workspace"
	WorkspaceNx        WorkspaceKind = "nx"
	WorkspaceTurborepo WorkspaceKind = "turborepo"
	WorkspaceUv        WorkspaceKind = "uv-workspace"
)

// ScanStats reports counters from the walk, useful for downstream tools
// that want to know whether they hit a cap and should widen the scan.
type ScanStats struct {
	DirsVisited     int
	FilesSeen       int
	DepthCapHits    int
	DirCapHits      int
	SymlinksSkipped int
}
