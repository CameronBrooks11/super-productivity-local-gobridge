# VS Code Copilot

## Auto-Configure

```bash
sp-local-bridge configure vscode-copilot
```

This adds an entry to your VS Code MCP config file:

| OS | Path |
|----|------|
| Linux | `~/.config/Code/User/mcp.json` |
| macOS | `~/Library/Application Support/Code/User/mcp.json` |
| Windows | `%APPDATA%\Code\User\mcp.json` |

## Manual Setup

Add to your `mcp.json`:

```json
{
  "servers": {
    "superProductivity": {
      "type": "stdio",
      "command": "/path/to/sp-local-bridge",
      "args": ["mcp"]
    }
  }
}
```

Replace `/path/to/sp-local-bridge` with the actual binary path. Find it with:

```bash
which sp-local-bridge
```

## Verify

After restarting VS Code:

1. Open Copilot Chat
2. The bridge tools should appear (16 tools)
3. Try: "List my tasks" or "Create a task called Test"

## Troubleshooting

- **Tools not appearing**: Check that the binary path is absolute and the binary is executable
- **Connection errors**: Ensure Super Productivity is running with Local REST API enabled
- **Permission errors**: The binary needs read/write access to the config file
