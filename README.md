# Super Productivity Local Go Bridge

> **Status: Pre-release / WIP** — builds and passes tests on Linux. Cross-platform CI is configured but not yet validated against a real release. MCP protocol has not been tested against live hosts beyond unit tests.

A local automation bridge for the [Super Productivity](https://super-productivity.com/) desktop app, written in Go. Provides CLI access and an MCP (Model Context Protocol) server for AI-assisted task management.

This is a Go rewrite of [super-productivity-local-bridge](https://github.com/CameronBrooks11/super-productivity-local-bridge) (Python v0.2.0) targeting single-binary portability.

## Features

- **Single binary** — no runtime dependencies, ~5 MB static binary
- **MCP server** — hand-rolled JSON-RPC 2.0 over stdio (protocol version 2024-11-05)
- **CLI** — task CRUD, time tracking, projects, tags
- **16 operations** — same operation set as the Python v0.2.0 bridge
- **Host configuration** — configure claude-desktop, vscode-copilot, codex
- **Strict validation** — integer fields reject exponents and overflow

## Requirements

- Super Productivity desktop app with Local REST API enabled (`http://127.0.0.1:3876`)
- Enable: Settings → Sync & Export → Local REST API

## Installation

### From source (recommended until first release)

```bash
git clone https://github.com/CameronBrooks11/super-productivity-local-gobridge
cd super-productivity-local-gobridge
make build
install -m 755 sp-local-bridge ~/.local/bin/
```

### From releases (after first tagged release)

```bash
curl -sSL https://raw.githubusercontent.com/CameronBrooks11/super-productivity-local-gobridge/main/scripts/install.sh | bash
```

The install script downloads the binary + checksums, verifies SHA256 (fails closed on mismatch or missing checksum), creates multicall symlinks, and installs to `~/.local/bin` by default. Works on both Linux (sha256sum) and macOS (shasum).

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

## Host Configuration

The `configure` command writes MCP config directly to a host's config file:

```bash
sp-local-bridge configure <host>           # Add entry
sp-local-bridge configure --dry-run <host> # Preview without writing
sp-local-bridge configure --remove <host>  # Remove entry
```

Supported hosts:

| Host | Format | Config path (Linux) |
|------|--------|---------------------|
| `claude-desktop` | JSON | `~/.config/Claude/claude_desktop_config.json` |
| `vscode-copilot` | JSON | `~/.config/Code/User/mcp.json` |
| `codex` | TOML | `~/.codex/config.toml` |

macOS uses `~/Library/Application Support/...` and Windows uses `%APPDATA%\...` equivalents automatically.

Features: atomic writes (temp + rename), backup (.bak), fail-closed on malformed configs, JSON merge preserves existing entries, surgical TOML editing preserves other sections.

## CLI Reference

```
sp-local-bridge health                       Check SP connectivity
sp-local-bridge status                       Get SP app status
sp-local-bridge tasks list [filters]         List tasks
sp-local-bridge tasks get <id>               Get a task by ID
sp-local-bridge tasks add <title>            Create a new task
sp-local-bridge tasks update <id> [flags]    Update a task
sp-local-bridge tasks complete <id>          Mark as done
sp-local-bridge tasks uncomplete <id>        Mark as not done
sp-local-bridge tasks start <id>             Start time tracking
sp-local-bridge tasks stop-current           Stop current task tracking
sp-local-bridge tasks current                Get currently tracked task
sp-local-bridge tasks set-current <id>       Set current task by ID
sp-local-bridge tasks clear-current          Clear current task
sp-local-bridge tasks archive <id>           Archive a task
sp-local-bridge tasks restore <id>           Restore an archived task
sp-local-bridge projects list [--query ...]  List projects
sp-local-bridge tags list [--query ...]      List tags
sp-local-bridge doctor                       Run diagnostics
sp-local-bridge configure <host>             Write host config
sp-local-bridge print-config <host>          Print config snippet
```

### Task list filters

```
--query <text>                    Filter by title substring
--project-id <id>                 Filter by project
--tag-id <id>                     Filter by tag (use TODAY for today's tasks)
--include-done                    Include completed tasks
--source <active|archived|all>    Task pool to query
```

### Task update flags

```
--title <text>           New title
--notes <text>           New notes
--project-id <id>        Set project
--due-day <YYYY-MM-DD>   Set due date
--time-estimate <ms>     Set time estimate (milliseconds)
--time-spent <ms>        Set time spent (milliseconds)
--done                   Mark complete
--not-done               Mark incomplete
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

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SP_BASE_URL` | `http://127.0.0.1:3876` | SP Local REST API base URL |

## Development

```bash
make build          # Build binary
make test           # Run tests
make test-cover     # Tests with coverage
make check          # Format check + vet + test (non-mutating)
make race           # Run tests with race detector
make fmt            # Format code (mutates files)
make clean          # Remove build artifacts
```

## Architecture

```
cmd/sp-local-bridge/    Entry point (multicall + subcommand dispatch)
internal/
  bridge/               Core types, errors, validation, REST client, service
  cli/                  CLI command handling
  mcpadapter/           MCP stdio server adapter
  doctor/               Connectivity and environment diagnostics
  hostcfg/              Host app configuration writer (JSON + TOML)
  version/              Build-time version info
scripts/                Install/uninstall scripts
```

## Design Decisions

- **No MCP SDK**: Hand-rolled JSON-RPC keeps the dependency tree at zero. Acceptable only after real host validation (not yet complete).
- **Multicall binary**: Single binary responds to `argv[0]` for MCP host compatibility (hosts launch `sp-local-bridge-mcp` directly).
- **No float64 for integers**: Integer fields use `strconv.ParseInt` directly on raw JSON to avoid precision loss.

## License

MIT