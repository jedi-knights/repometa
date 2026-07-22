# repometa

A Go library that scans a source repository and reports the components (buildable units) it contains, plus any monorepo workspace layouts it recognizes.

![CI](https://github.com/jedi-knights/repometa/actions/workflows/ci.yml/badge.svg)
![Release](https://github.com/jedi-knights/repometa/actions/workflows/release.yml/badge.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/jedi-knights/repometa.svg)](https://pkg.go.dev/github.com/jedi-knights/repometa)
[![Go Report Card](https://goreportcard.com/badge/github.com/jedi-knights/repometa)](https://goreportcard.com/report/github.com/jedi-knights/repometa)

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Usage](#usage)
- [Configuration](#configuration)
- [Development](#development)
- [Releases](#releases)
- [Contributing](#contributing)
- [License](#license)

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
go test -race ./...
```

Lint locally (matches CI):

```bash
golangci-lint run
```

Coverage report:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Releases

Releases are automated by [go-semantic-release](https://github.com/jedi-knights/go-semantic-release). On every push to `main`:

1. CI runs lint, tests, and the 70% coverage gate.
2. On success, the Release workflow analyzes conventional commits since the last tag.
3. If a release is warranted, semantic-release writes `CHANGELOG.md` and `VERSION`, pushes a `vX.Y.Z` tag, and creates the matching GitHub Release.

Consumers pull the tagged version via `go get github.com/jedi-knights/repometa@vX.Y.Z`.

Commit messages must follow [Conventional Commits](https://www.conventionalcommits.org/): `feat:` and `fix:` drive minor/patch bumps, `feat!:` or a `BREAKING CHANGE:` footer drives a major bump.

## Contributing

Issues and pull requests are welcome at [github.com/jedi-knights/repometa](https://github.com/jedi-knights/repometa). Commit messages must follow Conventional Commits (see [Releases](#releases)) so the release pipeline can compute the next version.

## License

N/A — no `LICENSE` file has been added yet.
