# Super Productivity Local Go Bridge

Single-binary MCP server and CLI that lets AI agents manage [Super Productivity](https://super-productivity.com/) tasks over the app's own Local REST API — no plugin, no Node runtime, and no way for it to delete a task.

It is for a Super Productivity user who already drives an MCP-capable AI coding agent and wants it reading and updating their real task list, without installing a Node toolchain or trusting a third-party plugin inside the app.

**[Documentation Site](https://cameronbrooks11.github.io/super-productivity-local-gobridge/)**

## Features

- **Single binary** — no runtime dependencies, small static binary
- **MCP server** — JSON-RPC 2.0 over stdio (protocol version 2024-11-05)
- **16 MCP tools** — read, create, update, complete and uncomplete, track time, archive and restore
- **No task deletion** — no operation that deletes a task is exposed at any layer, by design
- **Store integrity checking** — `doctor --deep` cross-checks the task and archive pools against the project, tag and subtask indexes, and against each other, for dangling references, orphaned active tasks, and ids present in both pools
- **Host auto-config** — configures Claude Code, Claude Desktop, VS Code Copilot, Codex CLI
- **Strict validation** — integer fields reject exponents and overflow

## Requirements

- Super Productivity desktop app with Local REST API enabled (`http://127.0.0.1:3876`)
- Enable: Settings → Sync & Export → Local REST API

## Get Started

Clone the repository:

```bash
git clone https://github.com/CameronBrooks11/super-productivity-local-gobridge.git
cd super-productivity-local-gobridge
```

Deploy your coding agent and ask it to set up the bridge:

```
Set up the Super Productivity Local Bridge
```

The setup skill detects your platform, downloads and installs the binary, writes your agent's MCP configuration, and verifies connectivity to Super Productivity.

### Verify

Ask your agent:

```
What's the status of my Super Productivity tasks?
```

### Manual Install

If you prefer not to use agent-driven setup:

```bash
curl -sSL https://raw.githubusercontent.com/CameronBrooks11/super-productivity-local-gobridge/main/scripts/install.sh | bash
sp-local-bridge configure claude-code      # or claude-desktop, vscode-copilot, codex
sp-local-bridge doctor
```

See the full [Install Guide](https://cameronbrooks11.github.io/super-productivity-local-gobridge/install) for all platforms, manual download, Windows, and from-source options.

## CLI Quick Reference

```bash
sp-local-bridge doctor                 # Check SP connectivity + environment
sp-local-bridge tasks list             # List tasks
sp-local-bridge tasks add "Title" [--project-id <id>] [--tag-id <id>]  # Create task
sp-local-bridge tasks start <id>       # Start time tracking
sp-local-bridge tasks stop-current     # Stop time tracking
sp-local-bridge configure <host>       # Write MCP config (claude-code, claude-desktop, vscode-copilot, codex)
sp-local-bridge configure --remove <host>  # Remove MCP config entry
sp-local-bridge mcp                    # Run MCP server (invoked by host, not manually)
```

See [Host Setup](https://cameronbrooks11.github.io/super-productivity-local-gobridge/hosts/) for details.

Full CLI reference: [Operations docs](https://cameronbrooks11.github.io/super-productivity-local-gobridge/operations).

## MCP Tools

16 tools exposed via stdio JSON-RPC:

| Tool | Description |
|------|-------------|
| `health` | Check SP connectivity and status |
| `get_status` | Get current SP application status |
| `list_tasks` | List tasks with optional filters |
| `get_task` | Get a task by ID |
| `create_task` | Create a new task |
| `update_task` | Update a task |
| `complete_task` | Mark a task as done |
| `uncomplete_task` | Mark a task as not done |
| `start_task` | Start time tracking |
| `stop_current_task` | Stop time tracking |
| `get_current_task` | Get currently tracked task |
| `set_current_task` | Set or clear current task |
| `archive_task` | Archive a task |
| `restore_task` | Restore an archived task |
| `list_projects` | List projects |
| `list_tags` | List tags |

## Security

- Localhost-only by design — no network exposure
- Atomic config writes with backup
- SHA256-verified release artifacts
- See [Security docs](https://cameronbrooks11.github.io/super-productivity-local-gobridge/security)

## Development

```bash
make check          # Format check + vet + test (non-mutating)
make build          # Build binary
make test           # Run tests
make race           # Run tests with race detector
```

## Documentation

Full docs: **https://cameronbrooks11.github.io/super-productivity-local-gobridge/**

- [Getting Started](https://cameronbrooks11.github.io/super-productivity-local-gobridge/getting-started)
- [Operations Reference](https://cameronbrooks11.github.io/super-productivity-local-gobridge/operations)
- [Migration from Python](https://cameronbrooks11.github.io/super-productivity-local-gobridge/migration)
- [Validation Status](https://cameronbrooks11.github.io/super-productivity-local-gobridge/validation-status)

## Status

Pre-release. CI passes on Linux, macOS, and Windows. One live host session has
been run — VS Code Copilot on Linux x86_64, against v0.1.1 on 2026-05-31 — and
it has not been repeated since; Claude Code, Claude Desktop, Codex, and
macOS/Windows live validation are pending. See
[Validation Status](https://cameronbrooks11.github.io/super-productivity-local-gobridge/validation-status)
for what each claim rests on.

This is the Go rewrite of
[super-productivity-local-bridge](https://github.com/CameronBrooks11/super-productivity-local-bridge)
(Python, last released v0.2.2), targeting single-binary portability. The Python bridge is
archived and read-only; work continues here.

## License

MIT
