# Codex CLI

## Auto-Configure

```bash
sp-local-bridge configure codex
```

This adds an entry to `~/.codex/config.toml` (same path on all platforms).

## Manual Setup

Add to your `~/.codex/config.toml`:

```toml
[mcp_servers.superProductivity]
command = "/path/to/sp-local-bridge"
args = ["mcp"]
```

Replace `/path/to/sp-local-bridge` with the actual binary path.

## Verify

Start a Codex session and ask it to list your tasks or run a health check.
Live Codex host validation is pending — see [Validation Status](/validation-status).

## Troubleshooting

- **Entry not recognized**: Ensure the TOML is valid. The configure command uses surgical editing to preserve other entries.
- **Connection errors**: Super Productivity must be running with Local REST API enabled
- **Binary not found**: Use an absolute path in the config
