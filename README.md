# repometa

A Go library that scans a source repository and reports the components
(buildable units) it contains, plus any monorepo workspace layouts it
recognizes. Intended to be imported by downstream tools that need to
reason about arbitrary repositories without re-implementing discovery.

## Status

v0 — API is unstable and will break without notice. This library exists
to unblock a small set of first-party consumer tools; when a second
consumer appears, the API will be reviewed for stability.

## Non-goals

- CLI. Import the library.
- JSON schema stability. Encode to whatever format you want in the caller.
- Behavioral detection ("does the team *use* uv"). Only *artifacts present*
  are reported — practices must be inferred by the caller.
- Full ecosystem coverage. Detectors are added when a consumer needs them.

## Supported detectors (v0)

| Ecosystem | Component kinds | Workspace kinds |
|-----------|----------------|-----------------|
| Go | `go-module` | `go-workspace` (from `go.work`) |
| Rust | `rust-crate`, `rust-workspace` | `cargo-workspace` |
| Python | `python-package` | `uv-workspace` |
| Node / JS / TS | `node-package` | `npm-yarn-workspace`, `pnpm-workspace`, `nx`, `turborepo` |
| CMake | `cmake-project` | — |
| Make | `make-project` | — |
| C | `c-source-tree` (only when no structured build system) | — |
| Assembly | `asm-source-tree` (only when no structured build system) | — |

Framework attributes on `node-package`: `js.framework=nextjs`, `js.framework=angular`.
Package-manager attributes on `python-package`: `python.pm={uv,poetry,pipenv,pip}`.

## Usage

```go
import "github.com/jedi-knights/repometa"

manifest, err := repometa.Scan("/path/to/repo")
if err != nil {
    return err
}
for _, c := range manifest.Components {
    fmt.Println(c.Kind, c.Root, c.Attributes)
    for _, ws := range c.Workspaces {
        fmt.Println("  workspace:", ws.Kind, ws.Members)
    }
}
```

## Bounds

Every recursive traversal is bounded. Defaults are set in `options.go`
(`defaultMaxDepth`, `defaultMaxDirs`, `defaultMaxFileSize`) and overridable
via `Option` functions. The walker also refuses to descend into a hardcoded
skip list (`.git`, `node_modules`, `.venv`, `vendor`, `target`, `dist`,
`build`, `.next`, `.angular`, etc.) and skips all symlinks.
