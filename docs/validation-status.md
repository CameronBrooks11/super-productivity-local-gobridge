# Validation Status

Current validation state for the Go bridge across platforms and host applications.

## Platform Matrix

| Platform | CI Tests | Live Validation |
|----------|----------|-----------------|
| Linux x86_64 | Passing | Not yet validated |
| macOS arm64 | Passing | Not yet validated |
| macOS x86_64 | Passing | Not yet validated |
| Windows x86_64 | Passing | Not yet validated |

## Host Validation

| Host | Config Generation | Config Write | Live MCP Session |
|------|-------------------|--------------|------------------|
| Claude Desktop | Tested | Tested | Not yet validated |
| VS Code Copilot | Tested | Tested | Not yet validated |
| Codex CLI | Tested | Tested | Not yet validated |

## What "Tested" Means

- **Config Generation**: The `print-config` command produces correct output, verified by automated tests.
- **Config Write**: The `configure` command writes valid config files with backup, verified by automated tests including fail-closed behavior on malformed input.
- **Live MCP Session**: A human has connected the bridge to the host application and confirmed tool invocations work end-to-end.

## Parity with Python Bridge

The Go bridge implements all 16 MCP tools from the Python bridge (v0.2.0). Black-box MCP tests verify request/response shapes match the Python SDK's output format. The Go bridge does not yet have independent live validation separate from the Python bridge's existing user base.
