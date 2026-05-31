# Super Productivity Local Go Bridge

A local automation bridge for the [Super Productivity](https://super-productivity.com/) desktop app, written in Go. Provides CLI access and an MCP (Model Context Protocol) server for AI-assisted task management.

This is a rewrite of [super-productivity-local-bridge](https://github.com/CameronBrooks11/super-productivity-local-bridge) (Python) as a single static binary with zero runtime dependencies.

## Features

- **Single binary** — no runtime dependencies, cross-platform (Linux, macOS, Windows)
- **MCP server** — stdio transport for AI host integration (Claude Desktop, VS Code, etc.)
- **CLI** — direct command-line access to all operations
- **16 operations** — tasks (CRUD, complete/uncomplete, archive/restore, time tracking), projects, tags, status
- **Strict validation** — same contract as the Python v0.2.0 bridge

## Requirements

- Super Productivity desktop app with Local REST API enabled (`http://127.0.0.1:3876`)
- Enable: Settings → Sync & Export → Local REST API

## Installation

### From releases (recommended)

```bash
curl -sSL https://raw.githubusercontent.com/CameronBrooks11/super-productivity-local-gobridge/main/scripts/install.sh | bash
```

### From source

```bash
git clone https://github.com/CameronBrooks11/super-productivity-local-gobridge
cd super-productivity-local-gobridge
make build
sudo install -m 755 sp-local-bridge /usr/local/bin/
```

## Quick Start

```bash
# Check connectivity
sp-local-bridge health

# List tasks
sp-local-bridge tasks list

# Create a task
sp-local-bridge tasks add "Review PR #42"

# Start tracking
sp-local-bridge tasks start <task-id>

# Run MCP server
sp-local-bridge mcp
```

## MCP Configuration

### Claude Desktop

```bash
sp-local-bridge configure
```

Or manually add to your Claude Desktop config:

```json
{
  "mcpServers": {
    "sp-local-bridge": {
      "command": "/usr/local/bin/sp-local-bridge",
      "args": ["mcp"]
    }
  }
}
```

### VS Code (Copilot)

Add to `.vscode/mcp.json`:

```json
{
  "servers": {
    "sp-local-bridge": {
      "command": "sp-local-bridge",
      "args": ["mcp"]
    }
  }
}
```

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

| Tool | Operation | Description |
|------|-----------|-------------|
| `health` | bridge.health | Check SP connectivity and status |
| `get_status` | status.get | Get current SP application status |
| `list_tasks` | task.list | List tasks with optional filters |
| `get_task` | task.get | Get a task by ID |
| `create_task` | task.create | Create a new task |
| `update_task` | task.update | Update a task |
| `complete_task` | task.complete | Mark a task as done |
| `uncomplete_task` | task.uncomplete | Mark a task as not done |
| `start_task` | task.start | Start time tracking |
| `stop_current_task` | task.stop_current | Stop time tracking |
| `get_current_task` | task.get_current | Get currently tracked task |
| `set_current_task` | task.set_current | Set or clear current task |
| `archive_task` | task.archive | Archive a task |
| `restore_task` | task.restore | Restore an archived task |
| `list_projects` | project.list | List projects |
| `list_tags` | tag.list | List tags |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SP_BASE_URL` | `http://127.0.0.1:3876` | SP Local REST API base URL |

## Development

```bash
# Build
make build

# Test
make test

# Test with coverage
make test-cover

# All checks (format, vet, test)
make check

# Clean
make clean
```

## Architecture

```
cmd/sp-local-bridge/    Entry point (multicall + subcommand dispatch)
internal/
  bridge/               Core types, errors, validation, REST client, service
  cli/                  CLI command handling
  mcpadapter/           MCP stdio server adapter
  doctor/               Connectivity diagnostics
  hostcfg/              Host app configuration writer
  version/              Build-time version info
testdata/fixtures/      Test fixture JSON files
scripts/                Install/uninstall scripts
```

## License

MIT