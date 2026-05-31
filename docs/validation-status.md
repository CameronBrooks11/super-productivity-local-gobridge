# Validation Status

Current validation state for the Go bridge across platforms and host applications.

## Platform Matrix

| Platform | CI Tests | Artifact Validation |
|----------|----------|---------------------|
| Linux x86_64 | Passing | v0.1.0-rc.1 validated (2026-05-31) |
| macOS arm64 | Passing | Not yet validated |
| macOS x86_64 | Passing | Not yet validated |
| Windows x86_64 | Passing | Not yet validated |

## Host Application Validation

| Host | Config Generation | Config Write | Live Host Session |
|------|-------------------|--------------|-------------------|
| Claude Desktop | Tested (automated) | Tested (automated) | Not yet validated |
| VS Code Copilot | Tested (automated) | Tested (automated) | Not yet validated |
| Codex CLI | Tested (automated) | Tested (automated) | Not yet validated |

"Live Host Session" means a human has connected the Go bridge to the specific host application and confirmed MCP tool invocations work end-to-end through the host's MCP client. This is distinct from raw stdio protocol validation below.

## Release Artifact Validation (v0.1.0-rc.1)

Raw MCP stdio protocol validation on Linux x86_64, 2026-05-31. This validates the release binary works correctly at the protocol level but does not validate host-app integration.

- Release workflow completed successfully (GoReleaser v7, all 6 platform archives published)
- `checksums.txt` verified: all 6 archives pass `sha256sum -c`
- Install script (`scripts/install.sh`) downloads, verifies checksum, extracts, and installs correctly
- `sp-local-bridge --version` reports `0.1.0-rc.1` with correct commit and date
- `sp-local-bridge doctor` passes all checks (health, status, task list, MCP self-check 16 tools, multicall aliases)
- `configure --dry-run` generates correct configs for claude-desktop, vscode-copilot, and codex
- All 16 MCP tools invoked successfully via raw stdio:
  - health, get_status, list_tasks, get_task, create_task, update_task
  - complete_task, uncomplete_task, start_task, stop_current_task
  - get_current_task, set_current_task, archive_task, restore_task
  - list_projects, list_tags

## What "Tested" Means

- **Config Generation**: The `print-config` command produces correct output, verified by automated tests.
- **Config Write**: JSON configs are parsed before mutation; Codex TOML uses structural guard tests plus backup and atomic-write tests.
- **Artifact Validation**: Release binary downloaded, checksum-verified, installed, and exercised via raw MCP stdio protocol.
- **Live Host Session**: A human has connected the bridge inside the actual host application and confirmed tool invocations work through the host's MCP client.

## Parity with Python Bridge

The Go bridge implements all 16 MCP tools from the Python bridge (v0.2.0). Black-box MCP tests verify request/response shapes match the Python SDK's output format. All 16 tools confirmed working via raw stdio artifact validation on Linux x86_64 (v0.1.0-rc.1). Host application validation is pending.
