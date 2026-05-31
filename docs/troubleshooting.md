# Troubleshooting

## Quick Diagnosis

```bash
sp-local-bridge doctor
```

This checks:
- Binary is installed and executable
- Super Productivity is reachable on `127.0.0.1:3876`
- Host config files are correctly configured

## Common Issues

### SP_UNAVAILABLE: Cannot connect to Super Productivity

**Cause**: The Super Productivity desktop app is not running, or the Local REST API is not enabled.

**Fix**:
1. Open Super Productivity
2. Go to Settings → Sync & Export → Local REST API
3. Enable the API
4. Verify: `curl http://127.0.0.1:3876/health`

### Tools not appearing in MCP host

**Cause**: Host config is missing or has wrong binary path.

**Fix**:
```bash
sp-local-bridge doctor        # Check what's wrong
sp-local-bridge configure <host>  # Re-run configure
```
Then restart the host application.

### "command not found" after install

**Cause**: Binary is not on PATH.

**Fix**:
```bash
# Check where it was installed
ls ~/.local/bin/sp-local-bridge

# Add to PATH if needed (add to ~/.bashrc or ~/.zshrc)
export PATH="$HOME/.local/bin:$PATH"
```

### TIMEOUT errors

**Cause**: SP is responding slowly or hung.

**Fix**:
1. Check SP is not in a stuck state (restart if needed)
2. The bridge uses a 10-second timeout per request

### INVALID_INPUT errors

**Cause**: Payload validation failed.

**Fix**: Check the error message for details. Common issues:
- Missing required field (`title` for create)
- Invalid `projectId` or `tagId`
- `parentId` combined with `projectId`/`tagIds`
- Negative `timeEstimate` or `timeSpent`
- String where integer expected

### Config file backup/restore

If `configure` made an unwanted change:

```bash
# Backups are saved as .bak files next to the config
ls ~/.config/Code/User/mcp.json.bak
ls ~/.config/Claude/claude_desktop_config.json.bak

# Restore manually
cp ~/.config/Code/User/mcp.json.bak ~/.config/Code/User/mcp.json
```

### MCP protocol errors

If you see JSON-RPC errors in host logs:

```bash
# Test MCP manually
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.1.0"}}}' | sp-local-bridge mcp
```

The response should be a valid JSON-RPC result with server capabilities.

## Getting Help

- [GitHub Issues](https://github.com/CameronBrooks11/super-productivity-local-gobridge/issues)
- Run `sp-local-bridge --version` to include version info in bug reports
