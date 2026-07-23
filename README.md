# repometa

A Go library that scans a source repository and reports the components (buildable units) it contains, plus any monorepo workspace layouts it recognizes.

[![CI](https://github.com/jedi-knights/repometa/actions/workflows/ci.yml/badge.svg)](https://github.com/jedi-knights/repometa/actions/workflows/ci.yml)
[![Release](https://github.com/jedi-knights/repometa/actions/workflows/release.yml/badge.svg)](https://github.com/jedi-knights/repometa/actions/workflows/release.yml)
[![Badge](https://github.com/jedi-knights/repometa/actions/workflows/badge.yaml/badge.svg)](https://github.com/jedi-knights/repometa/actions/workflows/badge.yaml)
[![Coverage](https://img.shields.io/badge/Coverage-unknown-lightgrey)](https://jedi-knights.github.io/repometa/)
[![Go Reference](https://pkg.go.dev/badge/github.com/jedi-knights/repometa.svg)](https://pkg.go.dev/github.com/jedi-knights/repometa)
[![Go Report Card](https://goreportcard.com/badge/github.com/jedi-knights/repometa)](https://goreportcard.com/report/github.com/jedi-knights/repometa)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-yellow.svg)](https://www.conventionalcommits.org/en/v1.0.0/)

[Installation](#installation) · [Usage](#usage) · [Configuration](#configuration) · [Development](#development) · [Releases](#releases) · [Contributing](#contributing) · [License](#license)

## Overview

repometa scans a directory tree and returns a manifest describing each buildable component it recognizes (Go module, Rust crate, Python package, Node package, CMake project, …) along with any monorepo workspace layout that groups them. It exists to unblock downstream tools that need to reason about arbitrary repositories without re-implementing discovery. Behavioral detection ("does the team *use* uv") is deliberately out of scope — only *artifacts present* are reported.

### Status

v0 — API is unstable and will break without notice. This library exists to unblock a small set of first-party consumer tools; when a second consumer appears, the API will be reviewed for stability.

### Non-goals

- CLI. Import the library.
- JSON schema stability. Encode to whatever format you want in the caller.
- Behavioral detection. Only artifacts present are reported.
- Full ecosystem coverage. Detectors are added when a consumer needs them.

## Features

- Bounded, symlink-safe directory traversal with a hardcoded skip list (`.git`, `node_modules`, `.venv`, `vendor`, `target`, `dist`, `build`, `.next`, `.angular`, …).
- Per-ecosystem detectors returning typed component and workspace records.
- Framework attributes on `node-package` (`js.framework=nextjs`, `js.framework=angular`).
- Package-manager attributes on `python-package` (`python.pm={uv,poetry,pipenv,pip}`).
- Configurable bounds (max depth, max directories visited, max file size read) via `Option` functions.

### Supported detectors (v0)

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

## Requirements

- Go 1.26 or newer (see `go.mod`).
- No runtime dependencies beyond the Go standard library and the two transitive indirect deps listed in `go.sum` (`github.com/BurntSushi/toml`, `gopkg.in/yaml.v3`).

## Installation

```bash
go get github.com/jedi-knights/repometa@latest
```

Pin to a specific version:

```bash
go get github.com/jedi-knights/repometa@v0.1.0
```

## Usage

```go
package main

import (
    "fmt"

    "github.com/jedi-knights/repometa"
)

func main() {
    manifest, err := repometa.Scan("/path/to/repo")
    if err != nil {
        panic(err)
    }
    for _, c := range manifest.Components {
        fmt.Println(c.Kind, c.Root, c.Attributes)
        for _, ws := range c.Workspaces {
            fmt.Println("  workspace:", ws.Kind, ws.Members)
        }
    }
}
```

## API Documentation

Full API reference is published on [pkg.go.dev](https://pkg.go.dev/github.com/jedi-knights/repometa). Every exported type, function, and option carries a godoc comment; treat pkg.go.dev as the authoritative reference and this README as an introduction.

## Configuration

`Scan` accepts `Option` functions to override traversal bounds. Defaults live in `options.go`.

| Option | Default | Purpose |
|--------|---------|---------|
| `WithMaxDepth(n)` | `defaultMaxDepth` | Cap directory descent depth. |
| `WithMaxDirs(n)` | `defaultMaxDirs` | Cap total directories visited. |
| `WithMaxFileSize(n)` | `defaultMaxFileSize` | Cap bytes read per file when inspecting content. |

The walker refuses to descend into a hardcoded skip list and skips all symlinks. These bounds are not tunable — they are safety invariants, not preferences.

## Development

```bash
git clone https://github.com/jedi-knights/repometa.git
cd repometa
go mod download
go build ./...
```

Lint locally (matches CI):

```bash
golangci-lint run
```

### Project layout

- `scan.go` — entry point (`Scan`, `ScanWith`).
- `walker.go` — bounded, symlink-safe directory traversal.
- `detect_*.go` — one file per ecosystem detector.
- `detectors.go` — detector registry and dispatch.
- `manifest.go` — `Manifest`, `Component`, `Workspace` types.
- `options.go` — traversal-bound `Option` functions and defaults.

## Testing

```bash
go test -race ./...
```

Coverage report:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

CI enforces a 70% coverage floor. The floor is deliberately below the current level so a modest regression does not block merges; raise it in `.github/workflows/ci.yml` once coverage stabilizes above 80%.

## Roadmap

Milestones roughly correspond to detector coverage and API stability.

- **v0.x** — additional detectors added on demand as consumers appear. API may break.
- **v1.0** — API frozen once a second first-party consumer exists and stresses the shape of `Manifest`, `Component`, and `Workspace`.
- **Post-v1** — detector plugins loaded at runtime (only if a consumer needs custom ecosystem detection).

Open issues track individual detector requests. Detector requests without a concrete consumer are declined by default — see `Non-goals`.

## Releases

Releases are automated by [go-semantic-release](https://github.com/jedi-knights/go-semantic-release). On every push to `main`:

1. CI runs lint, tests, and the 70% coverage gate.
2. On success, the Release workflow analyzes conventional commits since the last tag.
3. If a release is warranted, semantic-release writes `CHANGELOG.md` and `VERSION`, pushes a `vX.Y.Z` tag, and creates the matching GitHub Release.

Consumers pull the tagged version via `go get github.com/jedi-knights/repometa@vX.Y.Z`.

Commit messages must follow [Conventional Commits](https://www.conventionalcommits.org/): `feat:` and `fix:` drive minor/patch bumps, `feat!:` or a `BREAKING CHANGE:` footer drives a major bump.

## Versioning

repometa follows [Semantic Versioning 2.0.0](https://semver.org/). While the library is in v0, minor version bumps may include breaking API changes — pin to an exact version in production. Once v1 ships, breaking changes will only appear in major version bumps.

## Changelog

See [CHANGELOG.md](CHANGELOG.md). The changelog is generated automatically from conventional commits by the release workflow — do not edit it by hand.

## Contributing

Contributions are welcome. Please open an issue before starting substantial work so the direction can be agreed on.

- Fork the repository and create a topic branch from `main`.
- Write tests for new behavior; keep coverage above the CI floor.
- Ensure `golangci-lint run` and `go test -race ./...` pass locally.
- Use [Conventional Commits](https://www.conventionalcommits.org/) so the release pipeline can compute the next version.
- Open a pull request against `main` describing the change and its motivation.

One PR should carry one `type(scope):` pair — split unrelated changes into separate PRs.

## Code of Conduct

This project follows the [Contributor Covenant](https://www.contributor-covenant.org/version/2/1/code_of_conduct/) v2.1. By participating, you agree to abide by its terms. Report unacceptable behavior to the maintainer at omar.crosby@gmail.com.

## Security

Do **not** open public issues for security vulnerabilities. Instead, email omar.crosby@gmail.com with a description of the issue and, if possible, a reproduction. You will receive an acknowledgment within 72 hours and a coordinated disclosure timeline for the fix.

## Support

- **Bugs and feature requests** — [open a GitHub issue](https://github.com/jedi-knights/repometa/issues).
- **Questions and discussion** — [GitHub Discussions](https://github.com/jedi-knights/repometa/discussions).
- **API reference** — [pkg.go.dev](https://pkg.go.dev/github.com/jedi-knights/repometa).

This is a spare-time project; response times are best-effort.

## Acknowledgments

- [go-semantic-release](https://github.com/jedi-knights/go-semantic-release) — automates the release pipeline.
- [golangci-lint](https://github.com/golangci/golangci-lint) — meta-linter used in CI.
- The maintainers of each ecosystem's build-system conventions (Cargo, uv, pnpm, Turborepo, CMake, …) whose file formats this library reads.

## Maintainers

- [Omar Crosby](https://github.com/ocrosby) — omar.crosby@gmail.com

## License

MIT — see [LICENSE](LICENSE).
