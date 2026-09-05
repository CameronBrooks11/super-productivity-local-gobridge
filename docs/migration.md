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

**Order matters.** Both projects install console scripts with the same five
names — `sp-local-bridge` and the `-mcp`, `-doctor`, `-print-config` and
`-configure` variants — into the same directory (`~/.local/bin` by default).
Installing the Python bridge while the Go one is still there fails with
`Executable already exists: sp-local-bridge`.

### 1. Remove the Go bridge first

```sh
scripts/uninstall.sh
```

Or by hand:

```sh
rm -f ~/.local/bin/sp-local-bridge \
      ~/.local/bin/sp-local-bridge-{mcp,doctor,print-config,configure}
```

### 2. Install the Python bridge

It is not on PyPI; install from its release wheel:

```sh
uv tool install https://github.com/CameronBrooks11/super-productivity-local-bridge/releases/download/v0.2.0/sp_local_bridge-0.2.0-py3-none-any.whl
```

Or from a checkout:

```sh
git clone https://github.com/CameronBrooks11/super-productivity-local-bridge.git
cd super-productivity-local-bridge
scripts/install.sh
```

### 3. Rewrite your host config

The Python bridge has no `configure` subcommand — it ships separate
executables:

```sh
sp-local-bridge-configure <host>       # writes the config
sp-local-bridge-print-config <host>    # prints it to add by hand
```

Doing this before step 1 would find the Go binary still on `PATH` and write a
config pointing back at the Go bridge, leaving the rollback silently incomplete.

### 4. Verify

```sh
sp-local-bridge-doctor
```

## Behavioral Differences

The Go bridge has feature parity with the Python bridge v0.2.0 and adds Go-specific installation advantages. Known differences:

- Error messages may differ in wording (same error codes).
- The `doctor` command output format differs slightly.
- The Go bridge adds `configure --remove` for host config cleanup.
