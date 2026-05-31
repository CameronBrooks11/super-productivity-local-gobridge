# Super Productivity Local Go Bridge

> **Status: Pre-release** — CI passes on Linux, macOS, and Windows. MCP protocol shape is tested via black-box tests but not yet live-validated against host applications. See [Validation Status](https://cameronbrooks11.github.io/super-productivity-local-gobridge/validation-status).

A local automation bridge for the [Super Productivity](https://super-productivity.com/) desktop app, written in Go. Provides CLI access and an MCP (Model Context Protocol) server for AI-assisted task management.

Go rewrite of [super-productivity-local-bridge](https://github.com/CameronBrooks11/super-productivity-local-bridge) (Python v0.2.0) targeting single-binary portability.

**[Documentation Site](https://cameronbrooks11.github.io/super-productivity-local-gobridge/)**

## Features

- **Single binary** — no runtime dependencies, ~5 MB static binary
- **MCP server** — JSON-RPC 2.0 over stdio (protocol version 2024-11-05)
- **16 operations** — full parity with Python bridge v0.2.0
- **Host auto-config** — configures Claude Desktop, VS Code Copilot, Codex CLI
- **Strict validation** — integer fields reject exponents and overflow

## Requirements

- Super Productivity desktop app with Local REST API enabled (`http://127.0.0.1:3876`)
- Enable: Settings → Sync & Export → Local REST API

## Install

See the full [Install Guide](https://cameronbrooks11.github.io/super-productivity-local-gobridge/install) for all platforms and methods.

### From source

```bash
git clone https://github.com/CameronBrooks11/super-productivity-local-gobridge
cd super-productivity-local-gobridge
make build
install -m 755 sp-local-bridge ~/.local/bin/
```

## Quick Start

```bash
# Check connectivity
sp-local-bridge doctor

# List tasks
sp-local-bridge tasks list

# Create a task
sp-local-bridge tasks add "Review PR #42"

# Start tracking
sp-local-bridge tasks start <task-id>

# Configure a host
sp-local-bridge configure claude-desktop
sp-local-bridge configure vscode-copilot
sp-local-bridge configure codex

# Run MCP server (usually invoked by a host, not manually)
sp-local-bridge mcp
```

## Configure

```bash
sp-local-bridge configure claude-desktop   # Write MCP config
sp-local-bridge configure vscode-copilot
sp-local-bridge configure codex
sp-local-bridge configure --remove <host>  # Remove entry
```

See [Host Setup](https://cameronbrooks11.github.io/super-productivity-local-gobridge/hosts/) for details.

## Validate

```bash
sp-local-bridge doctor    # Check SP connectivity + environment
sp-local-bridge health    # Quick health check
```

## CLI

Full CLI reference is in the [Operations docs](https://cameronbrooks11.github.io/super-productivity-local-gobridge/operations).

```bash
sp-local-bridge tasks list            # List tasks
sp-local-bridge tasks add "Title"     # Create task
sp-local-bridge tasks start <id>      # Start tracking
sp-local-bridge tasks stop-current    # Stop tracking
sp-local-bridge projects list         # List projects
sp-local-bridge tags list             # List tags
```

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

## License

MIT
