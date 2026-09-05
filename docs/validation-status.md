# Validation Status

Current validation state for the Go bridge across platforms and host applications.

Each row records what was actually exercised, and when. A version in a cell is
the version that was validated, not necessarily the latest release.

## Platform Matrix

| Platform | CI Tests | Artifact Validation |
|----------|----------|---------------------|
| Linux x86_64 | `ubuntu-latest` | v0.3.0 validated (2026-09-05) |
| Linux arm64 | Not run | Archive checksum verified only |
| macOS arm64 | `macos-latest` | Archive checksum verified only |
| macOS x86_64 | Not run | Archive checksum verified only |
| Windows x86_64 | `windows-latest` | Archive checksum verified only |
| Windows arm64 | Not run | Archive checksum verified only |

The CI matrix is three runner images — `ubuntu-latest`, `macos-latest` and
`windows-latest` — so three of the six published targets are exercised by CI and
three are built but never run there. The rows name the image rather than assert
which architecture each currently maps to.

"Archive checksum verified only" means the published archive was downloaded and
matched `checksums.txt`, but the binary inside it has not been run on that
platform. All six v0.3.0 archives were verified this way on 2026-09-05; Linux
x86_64 additionally got the full validation below.

## Host Application Validation

| Host | Config Generation | Config Write | Live Host Session |
|------|-------------------|--------------|-------------------|
| Claude Code | Tested (automated) | Tested (automated) | Not yet validated |
| Claude Desktop | Tested (automated) | Tested (automated) | Not yet validated |
| VS Code Copilot | Tested (automated) | Tested (automated) | v0.1.1 (2026-05-31) |
| Codex CLI | Tested (automated) | Tested (automated) | Not yet validated |

"Live Host Session" means a human has connected the Go bridge to the specific
host application and confirmed MCP tool invocations work end-to-end through the
host's MCP client. This is distinct from raw stdio protocol validation below.
No live host session has been run against v0.3.0.

## Release Artifact Validation (v0.3.0)

Linux x86_64, 2026-09-05, against the published release
(`sp-local-bridge 0.3.0`, commit `1492aa5`, built `2026-09-05T21:08:25Z`).

- Release job completed successfully; all six platform archives and
  `checksums.txt` published
- All six archives downloaded and verified: `sha256sum -c checksums.txt`
  reports OK for each
- `scripts/install.sh` downloaded, checksum-verified, extracted and installed
  the binary, and created the four multicall alias symlinks
- The installed binary is byte-identical to the separately downloaded archive
  contents (same SHA-256)
- `--version` reports `0.3.0` with the release commit and build date
- `doctor` passes every check: PATH visibility, host configs, health, status,
  task list, MCP self-check (16 tools), multicall aliases
- `doctor --deep` reports store integrity OK against a live store of 284 active
  and 17 archived tasks, with all 284 referenced by the project and tag indexes
  or as a subtask of another task, which is the reference set the check builds:
  no dangling references, orphaned entities, duplicates or unresolved
  anomalies
- `configure --dry-run` generates config for all four hosts: `claude-code`,
  `claude-desktop`, `vscode-copilot`, `codex`
- Raw MCP stdio: `initialize` returns protocol `2024-11-05` and
  `serverInfo {"name":"sp-local-bridge","version":"0.3.0"}`; `tools/list`
  returns all 16 tools, with `limit` and `offset` present on `list_tasks`,
  `list_projects` and `list_tags`

## Live Client Validation (v0.3.0)

`make test-live` against a running Super Productivity on Linux x86_64,
2026-09-05. Read-only: the suite issues GET requests only, including a lookup
of a non-existent task id and of a non-existent route. It does not create,
modify, archive or delete anything.

- `TestLive_TaskFields` — every field the client depends on is present with the
  expected type, in both the active and archived pools
- `TestLive_ProjectFields`, `TestLive_TagFields`, `TestLive_StatusAndHealthFields`
  — same check for the remaining entity types
- `TestLive_NotFoundCodesAreDistinct` — a missing task and a missing route still
  report different error codes, which is the bug #37 fixed
- `TestLive_FixturesDoNotInventFields` — all seven committed success-response
  fixtures claim only fields SP actually returns. Fields that are null
  throughout the store have their presence checked but not their type, which
  the run reports
- `TestLive_StoreHasSomethingToCheck` — guards against the suite passing
  vacuously against an empty store

## What "Tested" Means

- **Config Generation**: Automated tests run `print-config` for the host and
  assert it succeeds. They check the exit status, not the content of the output.
- **Config Write**: JSON configs are parsed before mutation; Codex TOML uses structural guard tests plus backup and atomic-write tests. For `claude-code` this additionally covers preserving foreign keys and large integers in `~/.claude.json`.
- **Artifact Validation**: Release binary downloaded, checksum-verified, installed, and exercised via raw MCP stdio protocol.
- **Live Host Session**: A human has connected the bridge inside the actual host application and confirmed tool invocations work through the host's MCP client.

## VS Code Copilot Host Validation (v0.1.1)

Historical. Live host session validated on Linux x86_64, 2026-05-31, against
v0.1.1. Not repeated for v0.3.0.

- Binary: v0.1.1 installed via `scripts/install.sh` to `~/.local/bin`
- Config: `sp-local-bridge configure vscode-copilot` wrote to `~/.config/Code/User/mcp.json`
- Tool discovery: VS Code Copilot discovered all 16 MCP tools after window reload
- Live invocations confirmed through Copilot's MCP client:
  - `get_status`: returned task count and current task state
  - `list_tasks`: returned full task list with metadata (titles, projects, time tracking)

## Parity with Python Bridge

The Go bridge implements all 16 MCP tools from the Python bridge, whose last
release was v0.2.2. That project is archived and read-only, and its PyPI
releases are yanked; the Go bridge is where the work continues. Black-box MCP
tests check the request and response shapes against expectations written by
hand in this repository; nothing here compares output against a running Python
bridge.
