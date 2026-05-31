# AGENTS.md

Instructions for AI coding agents working in this repository.

## Project

Super Productivity Local Go Bridge — a local automation bridge for the Super Productivity desktop app, rewritten in Go for single-binary portability. Uses the SP Local REST API (`http://127.0.0.1:3876`) as the primary app-control path, with MCP as one thin host adapter.

## Stack

- **Go 1.23+** — zero external dependencies
- **Standard library** for HTTP, JSON, testing, and concurrency
- **Hand-rolled JSON-RPC 2.0** for MCP over stdio
- **GoReleaser v2** for release automation
- **GitHub Actions** for CI (Linux, macOS, Windows matrix)

## Layout

```
cmd/sp-local-bridge/       Entry point (multicall + subcommand dispatch)
internal/
  bridge/                  Core types, errors, validation, REST client, service
  cli/                     CLI command handling
  mcpadapter/              MCP stdio server adapter
  doctor/                  Connectivity and environment diagnostics
  hostcfg/                 Host app configuration writer (JSON + TOML)
  version/                 Build-time version info
scripts/                   Install/uninstall scripts
testdata/fixtures/         JSON response fixtures for tests
docs/                      Documentation + host guides
```

## Commands

```sh
make build                 # Build binary
make test                  # Run tests
make test-cover            # Tests with coverage
make check                 # Format check + vet + test (non-mutating)
make race                  # Run tests with race detector
make fmt                   # Format code (mutates files)
make clean                 # Remove build artifacts
go test ./... -count=1     # Run all tests
gofmt -l .                 # Check formatting (should be empty)
go vet ./...               # Static analysis
```

## Conventions

- All Go code lives under `cmd/` and `internal/` (standard Go layout).
- Zero external dependencies — stdlib only.
- MCP adapter is thin — all logic lives in `internal/bridge/`.
- No Claude/agent-specific language in tool descriptions or core code.
- Use SP-native camelCase field names at REST boundaries (`projectId`, `tagIds`).
- Type annotations on all exported functions.
- Tests in `internal/*/` alongside the code they test.
- Integer fields parsed via `strconv.ParseInt` from raw JSON (no float64).
- Atomic file writes with backup for host config mutations.
- Fail-closed on malformed config files.

## Do NOT

- Add runtime deps without discussing (the dep tree is intentionally zero).
- Put business logic in the MCP adapter.
- Reference specific AI hosts (Claude, Cursor, etc.) outside `docs/hosts/`.
- Create files named after phases or milestones.
- Commit `working/`, temp review files, or build artifacts.
- Use `float64` for integer JSON fields.
- Expose `task.delete` at any layer.
