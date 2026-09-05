# Migration from Python Bridge

Guide for migrating from the Python bridge to the Go bridge (single binary).

::: warning The Python bridge is archived
[super-productivity-local-bridge](https://github.com/CameronBrooks11/super-productivity-local-bridge)
was archived on 2026-09-05 and is read-only: no further fixes, releases, or
dependency updates. Migration is one-way in practice — see
[Rollback](#rollback) before relying on going back.
:::

## What Changes

| Aspect | Python Bridge | Go Bridge |
|--------|--------------|-----------|
| Install | `uv tool install <release wheel URL>` | Single binary download |
| Runtime | Python 3.11+ required | None (static binary) |
| Binary name | `sp-local-bridge` | `sp-local-bridge` |
| MCP tools | 16 tools | Same 16 tools |
| Config format | Same JSON/TOML | Same JSON/TOML |
| CLI interface | Subcommands via separate binaries | Subcommands via single multicall binary |
| Host config | `sp-local-bridge-print-config <host>` | `sp-local-bridge configure <host>` |

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

::: danger Rolling back lands on an archived project
The Python bridge is read-only. It will not receive fixes, and it is pinned to
whatever Super Productivity behaviour existed at v0.2.0. Treat a rollback as a
stopgap while a problem here is reported, not as a destination.
:::

The Python bridge was **never published to PyPI**. It installs from a GitHub
release wheel or from a checkout:

```sh
uv tool install https://github.com/CameronBrooks11/super-productivity-local-bridge/releases/download/v0.2.0/sp_local_bridge-0.2.0-py3-none-any.whl
```

Or from source:

```sh
git clone https://github.com/CameronBrooks11/super-productivity-local-bridge.git
cd super-productivity-local-bridge
uv tool install .
```

Then:

1. Re-run: `sp-local-bridge configure <host>`
2. Remove the Go binary from your PATH or delete it.

Earlier revisions of this page said `uv tool install sp-local-bridge`. That
never worked, and `sp-local-bridge` is an **unclaimed name on PyPI** — anyone
may register it, at which point the old instruction would install a stranger's
package. Use the release URL above.

## Behavioral Differences

The Go bridge has feature parity with the Python bridge v0.2.0 and adds Go-specific installation advantages. Known differences:

- Error messages may differ in wording (same error codes).
- The `doctor` command output format differs slightly.
- The Go bridge adds `configure --remove` for host config cleanup.
