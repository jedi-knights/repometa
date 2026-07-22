# repometa — Claude Instructions

## Pre-push verification (mandatory)

Before running `git push` for any branch — including main — the full local lint and test suite **must** pass. This is not advisory. Push CI failures back to `main` cost a broken build for every consumer that pulls between the failed push and the fix.

Run all three, in order, and confirm each succeeds:

```bash
gofmt -l .                       # must print nothing
golangci-lint run                # must exit 0
go test -race -count=1 ./...     # must exit 0
```

If any command fails or emits output that indicates a problem, fix the issue and re-run the full sequence from the top. Do not push a partial fix hoping the next step will catch the rest.

### If a tool is unavailable

- `gofmt` and `go test` are part of the Go toolchain — always present when `go` is on PATH. If they are missing, stop and report; do not push.
- `golangci-lint` may not be installed on every machine. If it is missing, install it (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4`) rather than skipping the check. If installation genuinely cannot succeed (offline, sandboxed), state that explicitly in the response before pushing and flag it as a follow-up.

### Do not bypass hook failures

Never push with `--no-verify` or otherwise skip pre-commit / pre-push hooks. If a hook blocks, the hook is correct — fix the underlying issue.

## Commit and release conventions

- All commits must follow [Conventional Commits](https://www.conventionalcommits.org/). `feat:` and `fix:` drive minor/patch releases through go-semantic-release; `feat!:` or a `BREAKING CHANGE:` footer drives a major bump.
- One PR = one `type(scope):` pair. Split unrelated changes.
- The release pipeline (`.github/workflows/release.yml`) runs on every successful CI on `main` and pushes `vX.Y.Z` tags + GitHub Releases automatically. Consumers pull via `go get`.

## Scope

- repometa is a library — no CLI, no binaries. Do not add `cmd/` or GoReleaser configuration.
- Detectors are added only when a first-party consumer needs them. See `README.md` "Non-goals".
