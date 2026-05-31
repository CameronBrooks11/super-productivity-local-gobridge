# Host Setup

The bridge works as an MCP server for any host that supports stdio-based MCP tools.

## Supported Hosts

| Host | Config Format | Auto-configure |
|------|--------------|----------------|
| [VS Code Copilot](./vscode-copilot) | JSON | `sp-local-bridge configure vscode-copilot` |
| [Claude Desktop](./claude-desktop) | JSON | `sp-local-bridge configure claude-desktop` |
| [Codex CLI](./codex) | TOML | `sp-local-bridge configure codex` |

## How It Works

1. Run `sp-local-bridge configure <host>` to write the MCP entry
2. Restart the host application
3. The host discovers the bridge tools via MCP `tools/list`
4. Tools are available in your AI conversations

## Preview Without Writing

```bash
sp-local-bridge configure --dry-run <host>
```

## Remove Configuration

```bash
sp-local-bridge configure --remove <host>
```

## Print Config (Manual Setup)

If you prefer to add config manually:

```bash
sp-local-bridge print-config <host>
```

This prints the config snippet and the file path where it should be added.
