# Validation Status

Current validation state for the Go bridge across platforms and host applications.

## Platform Matrix

| Platform | CI Tests | Live Validation |
|----------|----------|-----------------|
| Linux x86_64 | Passing | v0.1.0-rc.1 validated (2026-05-31) |
| macOS arm64 | Passing | Not yet validated |
| macOS x86_64 | Passing | Not yet validated |
| Windows x86_64 | Passing | Not yet validated |

## Host Validation

| Host | Config Generation | Config Write | Live MCP Session |
|------|-------------------|--------------|------------------|
| Claude Desktop | Tested | Tested | Not yet validated |
| VS Code Copilot | Tested | Tested | Not yet validated |
| Codex CLI | Tested | Tested | Not yet validated |

## Release Artifact Validation (v0.1.0-rc.1)

Validated on Linux x86_64, 2026-05-31:

- Release workflow completed successfully (GoReleaser v7, all 6 platform archives published)
- `checksums.txt` verified: all 6 archives pass `sha256sum -c`
- Install script (`scripts/install.sh`) downloads, verifies checksum, extracts, and installs correctly
- `sp-local-bridge --version` reports `0.1.0-rc.1` with correct commit and date
- `sp-local-bridge doctor` passes all checks (health, status, task list, MCP self-check 16 tools, multicall aliases)
- `configure --dry-run` generates correct configs for claude-desktop, vscode-copilot, and codex
- Live MCP session validated: initialize, tools/list (16 tools), health, get_status, create_task, start_task, get_current_task, stop_current_task, complete_task, archive_task, list_projects, list_tags

## What "Tested" Means

- **Config Generation**: The `print-config` command produces correct output, verified by automated tests.
- **Config Write**: JSON configs are parsed before mutation; Codex TOML uses structural guard tests plus backup and atomic-write tests.
- **Live MCP Session**: A human has connected the bridge to the host application and confirmed tool invocations work end-to-end.

## Parity with Python Bridge

The Go bridge implements all 16 MCP tools from the Python bridge (v0.2.0). Black-box MCP tests verify request/response shapes match the Python SDK's output format. Live MCP validation from release artifacts confirmed on Linux x86_64 (v0.1.0-rc.1).
