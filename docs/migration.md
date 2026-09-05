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

**Remove the Python bridge first.** Both projects install the same five console
scripts — `sp-local-bridge` and the `-mcp`, `-doctor`, `-print-config` and
`-configure` variants — and `~/.local/bin` is both this repo's default install
directory and uv's default bin directory. `uv tool uninstall` deletes every
executable in its receipt without checking whether the file still belongs to
it, so running it *after* installing the Go bridge deletes the Go binary and
all four of its symlinks, leaving you with neither bridge.

::: tip Keeping a working bridge throughout
Removing first means there is a window with no bridge, and `install.sh` can
fail — unsupported architecture, an unreachable GitHub API, a checksum
mismatch. To avoid the gap, install the Go bridge to a scratch directory and
check it before removing anything:

```sh
INSTALL_DIR=/tmp/sp-go bash scripts/install.sh
/tmp/sp-go/sp-local-bridge doctor
```

Then continue with step 1 and reinstall normally in step 2.
:::

### 1. Note your configured hosts, then remove the Python bridge

The Python bridge writes host config for `claude-desktop`, `vscode-copilot` and
`codex`. Note which you use — you will rewrite them in step 3.

```sh
uv tool uninstall sp-local-bridge
```

If you installed it from a checkout with `pip` instead, use
`pip uninstall sp-local-bridge`.

### 2. Install the Go bridge and check it works

```sh
curl -sSL https://raw.githubusercontent.com/CameronBrooks11/super-productivity-local-gobridge/main/scripts/install.sh | bash
sp-local-bridge doctor
```

This confirms the binary can reach Super Productivity's local API. See the
[Install guide](./install) for manual and from-source options.

If `doctor` reports `command not found`, the installer's target directory is not
on your `PATH` — it warns about this and still exits 0. Add it:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

### 3. Rewrite host config

The old entries point at the Python entry point (a `uv` shim path), so they must
be rewritten rather than left alone:

```sh
sp-local-bridge configure claude-desktop     # repeat per host you noted
```

`configure` auto-detects the running binary path. The Go bridge also supports
`claude-code`, which the Python bridge did not.

### 4. Verify

```sh
sp-local-bridge doctor
```

## Rollback

::: danger Rolling back lands on an archived project
The Python bridge is read-only. It will not receive fixes, and it is pinned to
whatever Super Productivity behaviour existed at v0.2.2. Treat a rollback as a
stopgap while a problem here is reported, not as a destination.
:::

**Order matters.** Both projects install console scripts with the same five
names — `sp-local-bridge` and the `-mcp`, `-doctor`, `-print-config` and
`-configure` variants — into the same directory (`~/.local/bin` by default).
Installing the Python bridge while the Go one is still there fails with:

```
error: Executables already exist: sp-local-bridge, sp-local-bridge-configure,
sp-local-bridge-doctor, sp-local-bridge-mcp, sp-local-bridge-print-config
(use --force to overwrite)
```

`--force` overwrites them in one step, but removing the Go bridge properly is
cleaner and is what the steps below do.

### 1. Remove the Go bridge first

Find where it actually is — the installer defaults to `~/.local/bin`, but the
manual install puts it in `/usr/local/bin` and `go install` in `~/go/bin`, and
removing the wrong directory leaves the Go binary on `PATH` and the rollback
silently incomplete:

```sh
command -v sp-local-bridge
```

::: warning Remove a claude-code entry first
The Python bridge cannot write or remove `claude-code` host config — it only
knows `claude-desktop`, `vscode-copilot` and `codex`. If you configured
`claude-code`, clear it while the Go bridge is still installed:

```sh
sp-local-bridge configure --remove claude-code
```

Otherwise `~/.claude.json` keeps an entry pointing at a binary that no longer
exists, and you will have to edit it by hand.
:::

From a checkout of this repo — pass the directory, or the script looks only in
its default `~/.local/bin`, reports "not found", and exits 0 while the binary
is still installed elsewhere:

```sh
INSTALL_DIR="$(dirname "$(command -v sp-local-bridge)")" scripts/uninstall.sh
```

Otherwise remove the binary and its four aliases directly. Add `sudo` if they
live in `/usr/local/bin`:

```sh
DIR=$(dirname "$(command -v sp-local-bridge)")
rm -f "$DIR"/sp-local-bridge \
      "$DIR"/sp-local-bridge-mcp \
      "$DIR"/sp-local-bridge-doctor \
      "$DIR"/sp-local-bridge-print-config \
      "$DIR"/sp-local-bridge-configure
```

Leaving the aliases behind is not harmless: step 2 aborts on those four names
even once the main binary is gone.

### 2. Install the Python bridge

The name `sp-local-bridge` is registered on PyPI, but the release is
**yanked** — it holds the name so nobody else can take it, and resolvers skip
it so nothing installs it by accident. An exact pin still works if you need it:

```sh
uv tool install 'sp-local-bridge==0.2.2'
```

Or install the release wheel directly, which does not depend on PyPI at all:

```sh
uv tool install https://github.com/CameronBrooks11/super-productivity-local-bridge/releases/download/v0.2.2/sp_local_bridge-0.2.2-py3-none-any.whl
```

Or from a checkout:

```sh
git clone --branch v0.2.2 \
  https://github.com/CameronBrooks11/super-productivity-local-bridge.git
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
