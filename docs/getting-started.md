# Getting Started

## Requirements

- [Super Productivity](https://super-productivity.com/) desktop app
- Local REST API enabled: Settings → Sync & Export → Local REST API

The bridge communicates with SP on `http://127.0.0.1:3876`.

## Access token

Super Productivity **18.19.0 and newer** rejects every request that does not
carry an access token. `GET /health` is the one exception, which is why a
misconfigured setup looks connected while nothing works.

The bridge finds the token in one of two ways:

1. **`SP_API_TOKEN`**, if set.
2. **SP's own token file**, otherwise — the file SP writes when you enable the
   API:

   | Platform | Path |
   |---|---|
   | Linux | `~/.config/superProductivity/local-rest-api-token` |
   | macOS | `~/Library/Application Support/superProductivity/local-rest-api-token` |
   | Windows | `%APPDATA%\superProductivity\local-rest-api-token` |

The file is the reason most setups need no configuration at all — including MCP
hosts, which launch the bridge as a subprocess and would otherwise need the
token written into a host config file.

Reach for `SP_API_TOKEN` when the file is not where the bridge looks: a portable
install, a non-default `--user-data-dir`, or a container. The token itself is
shown at **Settings → Misc → Access Token**.

`sp-local-bridge doctor` reports which source was used, and never prints the
token:

```
Access token... read from /home/you/.config/superProductivity/local-rest-api-token
```

Older Super Productivity has no token. `Access token... none found` is the
correct and harmless result there.

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
