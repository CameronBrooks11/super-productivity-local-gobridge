# Getting Started

## Requirements

- [Super Productivity](https://super-productivity.com/) desktop app
- Local REST API enabled: Settings → Sync & Export → Local REST API

The bridge communicates with SP on `http://127.0.0.1:3876`.

## Install

### From source

```bash
git clone https://github.com/CameronBrooks11/super-productivity-local-gobridge
cd super-productivity-local-gobridge
make build
install -m 755 sp-local-bridge ~/.local/bin/
```

### From releases

```bash
curl -sSL https://raw.githubusercontent.com/CameronBrooks11/super-productivity-local-gobridge/main/scripts/install.sh | bash
```

The installer verifies SHA256 checksums (fails closed on mismatch), creates multicall symlinks, and installs to `~/.local/bin`.

## Verify

```bash
sp-local-bridge doctor
```

This checks binary info, PATH visibility, host config status, SP connectivity, and task list access.

## Configure an MCP Host

```bash
sp-local-bridge configure claude-code     # Claude Code (CLI agent)
sp-local-bridge configure vscode-copilot   # VS Code Copilot
sp-local-bridge configure claude-desktop   # Claude Desktop
sp-local-bridge configure codex            # Codex CLI
```

Use `--dry-run` to preview changes. Use `--remove` to remove an entry.

See [Host Setup](./hosts/) for detailed per-host instructions.

## Quick CLI Usage

```bash
sp-local-bridge health                    # Check SP connectivity
sp-local-bridge tasks list                # List active tasks
sp-local-bridge tasks add "My new task"   # Create a task
sp-local-bridge tasks start <id>          # Start time tracking
sp-local-bridge tasks stop-current        # Stop tracking
sp-local-bridge tasks complete <id>       # Mark done
```
