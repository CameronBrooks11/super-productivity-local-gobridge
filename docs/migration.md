# Migration from Python Bridge

Guide for migrating from the Python bridge (`sp-local-bridge` via `uv`/pip) to the Go bridge (single binary).

## What Changes

| Aspect | Python Bridge | Go Bridge |
|--------|--------------|-----------|
| Install | `uv tool install` or `pip install` | Single binary download |
| Runtime | Python 3.11+ required | None (static binary) |
| Binary name | `sp-local-bridge` | `sp-local-bridge` |
| MCP tools | 16 tools | Same 16 tools |
| Config format | Same JSON/TOML | Same JSON/TOML |
| CLI interface | Same subcommands | Same subcommands |

## Migration Steps

### 1. Verify the Go bridge works

Download the Go binary and run diagnostics before removing the Python version:

```sh
./sp-local-bridge doctor
```

This confirms the binary can reach Super Productivity's local API.

### 2. Update host config paths

If your host config points to the Python entry point (e.g. a `uv` shim path), update the `command` field to point to the new Go binary location:

```sh
# Remove old Python config entry
sp-local-bridge configure --remove claude-desktop

# Write new entry pointing to Go binary
sp-local-bridge configure claude-desktop
```

The `configure` command auto-detects the running binary path.

### 3. Remove the Python bridge

```sh
uv tool uninstall sp-local-bridge
# or
pip uninstall sp-local-bridge
```

### 4. Verify

```sh
sp-local-bridge doctor
```

## Rollback

To revert to the Python bridge:

1. Reinstall: `uv tool install sp-local-bridge`
2. Re-run: `sp-local-bridge configure <host>`
3. Remove the Go binary from your PATH or delete it.

## Behavioral Differences

The Go bridge is a strict superset of the Python bridge v0.2.0. Known differences:

- Error messages may differ in wording (same error codes).
- The `doctor` command output format differs slightly.
- The Go bridge adds `configure --remove` for host config cleanup.
