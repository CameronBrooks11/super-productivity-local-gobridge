# Security

## Threat Model

The bridge operates as a **localhost-only** process communicating with the Super Productivity desktop app on `127.0.0.1:3876`.

### What the bridge can do

- Read all tasks, projects, and tags
- Create, update, complete, archive, and restore tasks
- Start and stop time tracking
- Modify host config files (when `configure` is run)

### What the bridge cannot do

- **Delete tasks** — `task.delete` is intentionally excluded
- **Access the network** — only connects to localhost
- **Modify SP settings** — only uses the task/project/tag APIs
- **Persist credentials** — no auth tokens or secrets are stored

## Data Flow

```
MCP Host → stdin/stdout → sp-local-bridge → HTTP localhost → Super Productivity
```

All communication is local. No data leaves your machine via the bridge.

## MCP Host Trust

Your MCP host (VS Code Copilot, Claude Desktop, Codex) decides when to invoke bridge tools. Review your host's:

- **Data handling policy** — the host sees your task data in tool responses
- **Tool approval settings** — some hosts auto-approve, others ask per-call
- **Context window** — task data may be sent to the host's AI model

## Config File Safety

The `configure` command:

- Creates a backup (`.bak`) before modifying any config file
- Uses atomic writes (temp file + rename) to prevent corruption
- For JSON configs, parses the existing file before modification
- For TOML configs (Codex), applies a structural guard and creates a backup before surgical edits; this is not a full TOML parser
- Validates structure before writing

## Recommendations

1. **Back up SP data** before heavy automation use
2. **Use `--dry-run`** before first `configure` to preview changes
3. **Review host privacy** — your task data flows through the host
4. **Keep the binary updated** — install from tagged releases
5. **Restrict binary permissions** — standard user install to `~/.local/bin`
