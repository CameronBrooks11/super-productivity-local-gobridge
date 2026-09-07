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

### UNAUTHORIZED: connected, but every call fails

**Cause**: Super Productivity 18.19.0 and newer require an access token on every
route except `GET /health`. That exemption is why this looks like a working
connection — `doctor` reports `Health check... OK` and everything after it fails.

```
Access token... none found

Health check... OK
Status check... FAILED: Authorization token required — send "Authorization: Bearer <token>".
```

**Fix**: enable the Local REST API in SP at least once so it writes its token
file, which the bridge reads automatically. If the bridge still reports
`none found`, its idea of where that file lives is wrong — check the path
`doctor` prints, and set `SP_API_TOKEN` to the value from
**Settings → Misc → Access Token** if the file is somewhere else.

If `doctor` says a token was sent and SP rejected it, the token is stale.
Re-copy it; regenerating it in SP invalidates the old one.

### Tools not appearing in MCP host

**Cause**: Host config is missing or has wrong binary path.

**Fix**:
```bash
sp-local-bridge configure --status  # Which hosts have an entry, and in which scope
sp-local-bridge doctor              # Check what else is wrong
sp-local-bridge configure <host>    # Re-run configure
```
Then restart the host application.

`--status` reports whether an entry exists, not whether the command it records
still resolves. An entry left pointing at a binary that has since moved is still
reported as configured; re-running `configure <host>` rewrites the path.

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
