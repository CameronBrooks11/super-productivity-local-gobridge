# Claude Desktop

## Auto-Configure

```bash
sp-local-bridge configure claude-desktop
```

This adds an entry to your Claude Desktop config file:

| OS | Path |
|----|------|
| Linux | `~/.config/Claude/claude_desktop_config.json` |
| macOS | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Windows | `%APPDATA%\Claude\claude_desktop_config.json` |

## Manual Setup

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "super-productivity": {
      "command": "/path/to/sp-local-bridge",
      "args": ["mcp"]
    }
  }
}
```

Replace `/path/to/sp-local-bridge` with the actual binary path.

## Verify

After restarting Claude Desktop:

1. Open a new conversation
2. Look for the MCP tools indicator
3. Try: "List my current tasks"

## Troubleshooting

- **Server not connecting**: Ensure the binary path is absolute (Claude Desktop does not resolve PATH)
- **Connection errors**: Super Productivity must be running with Local REST API enabled
- **Config file missing**: Run `sp-local-bridge configure claude-desktop` to create it
