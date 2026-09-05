# Claude Code

Claude Code is the CLI agent, and is a **different host from [Claude Desktop](./claude-desktop)** — it has its own config file. Configuring one does not configure the other.

## Auto-Configure

```bash
sp-local-bridge configure claude-code
```

This adds an entry to `~/.claude.json` at **user scope**, making the bridge available in every project:

| OS | Path |
|----|------|
| Linux / macOS | `~/.claude.json` |
| Windows | `%USERPROFILE%\.claude.json` |

The file also holds unrelated Claude Code state. The bridge parses it, adds one key, and writes it back atomically with a `.bak` alongside (copied with the original's permissions); numbers are preserved as written rather than round-tripped through floats.

::: warning Close Claude Code first
Unlike the other hosts, Claude Code **rewrites this file while it runs**. `configure` re-checks the file immediately before writing and aborts rather than reverting a concurrent change, but a session that was already open can still overwrite the entry when it exits.

Close all Claude Code sessions before running `configure claude-code`. If `sp-local-bridge doctor` later reports the host as unconfigured, that is what happened — re-run the command with no sessions open.
:::

## Scopes

Claude Code supports three scopes. `configure` writes the user scope, because that is the one with a single fixed path per host. For the other two, use Claude Code's own CLI:

```bash
# This project only (private to you)
claude mcp add -s local super-productivity -- ~/.local/bin/sp-local-bridge mcp

# Committed to the repo, shared with collaborators
claude mcp add -s project super-productivity -- ~/.local/bin/sp-local-bridge mcp
```

Note that `-s local` is the default, so a bare `claude mcp add` configures this
project only.

`sp-local-bridge configure --status` reports which scope it found, naming the
project directory for a local-scope entry:

```
  claude-code          configured (local scope: /home/you/repo)
```

It does not read the project scope in a repo's `.mcp.json`, so an entry added
with `-s project` is reported as not configured.

## Manual Setup

Add to `~/.claude.json`:

```json
{
  "mcpServers": {
    "super-productivity": {
      "type": "stdio",
      "command": "/path/to/sp-local-bridge",
      "args": ["mcp"]
    }
  }
}
```

Replace `/path/to/sp-local-bridge` with the actual binary path.

## Verify

```bash
claude mcp get super-productivity
```

A working entry reports `✔ Connected`. Then try: "List my current tasks".

## Troubleshooting

- **Server not listed**: check the scope. `claude mcp list` shows what the current directory resolves to; a user-scope entry is shadowed by a local-scope entry of the same name.
- **Connection errors**: Super Productivity must be running with the Local REST API enabled.
- **Wrong host configured**: if you ran the setup skill inside a VS Code integrated terminal, confirm it chose `claude-code` and not `vscode-copilot` — Claude Code inherits `$VSCODE_PID` there.
