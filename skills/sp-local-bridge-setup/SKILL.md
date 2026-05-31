# Super Productivity Local Go Bridge — Setup

You are setting up the Super Productivity Local Go Bridge, an MCP server that
gives AI agents access to the Super Productivity desktop app's task management
(create/update/complete tasks, list projects and tags, etc.) via its Local REST API.

This is a single static binary with zero runtime dependencies.

Follow all phases in order. Do not skip phases. Do not ask the user questions
except at the single checkpoint in Phase 2.

---

## Phase 1: Discovery (silent — do not print findings yet)

Run these checks silently and collect the results:

1. **Bridge installed** — run `sp-local-bridge --version 2>/dev/null`. If it
   succeeds, the bridge is already installed.
2. **SP app running** — run `curl -sf http://127.0.0.1:3876/health`. If it
   fails, the Super Productivity desktop app is not running or Local REST API is
   not enabled.
3. **Detect platform** — check `uname -s` (Linux/Darwin) and `uname -m`.
4. **Detect agent** — determine which agent you are running as:
   - If `$VSCODE_PID` or `$VSCODE_IPC_HOOK_CLI` is set → vscode-copilot
   - If `codex --version` succeeds → codex
   - Otherwise → unknown (will ask user which host to configure)
5. **Existing config** — if installed, run `sp-local-bridge configure --dry-run <detected-host>`
   to check if config is already written.
6. **Skills symlink** — check if `~/.agents/skills/sp-local-bridge-setup` exists.

---

## Phase 2: Plan (single interactive checkpoint)

Present ALL findings from Phase 1 in a clear summary table:

```
Prerequisite        Status
─────────────────────────────────
Bridge installed    ✓ version / ✗ not installed
SP app              ✓ reachable / ✗ not running
Platform            linux/amd64 / darwin/arm64 / etc.
Host detected       vscode-copilot / claude-desktop / codex / unknown
Config written      ✓ / ✗
Skills symlink      ✓ / ✗
```

Then state the proposed actions:
- Install bridge (if not installed)
- Configure host (if not configured)
- Create skills symlink (if missing)

Ask the user: **"Proceed with setup? [Y/n]"**

If SP app is not running, warn but offer to proceed (config can be written without SP running).

---

## Phase 3: Execute (uninterrupted after confirmation)

### 3a. Install the bridge (if not already installed)

If not installed, install from the latest GitHub release (Linux/macOS):

```bash
curl -sSL https://raw.githubusercontent.com/CameronBrooks11/super-productivity-local-gobridge/main/scripts/install.sh | bash
```

Or, if you have the repository checked out locally, identify the repo root as the
directory containing `go.mod` and `scripts/install.sh`:

```bash
REPO_ROOT="<path to checked-out super-productivity-local-gobridge>"
# Verify:
test -f "$REPO_ROOT/go.mod" && test -f "$REPO_ROOT/scripts/install.sh" || { echo "Error: REPO_ROOT is incorrect"; exit 1; }
bash "$REPO_ROOT/scripts/install.sh"
```

If already installed, skip.

### 3b. Configure the detected host

Run the configure command:

```bash
sp-local-bridge configure <detected-host>
```

Where `<detected-host>` is one of: `vscode-copilot`, `claude-desktop`, `codex`.

If the host could not be detected, ask the user which host to configure.

### 3c. Create skills symlink

If you have the repository checked out locally:

```bash
mkdir -p ~/.agents/skills
ln -sfn "$REPO_ROOT/skills/sp-local-bridge-setup" ~/.agents/skills/sp-local-bridge-setup
```

If installed from release without a local checkout, skip this step (the skill is
already loaded by the host through its normal skill discovery).

---

## Phase 4: Verify

Run the doctor command:

```bash
sp-local-bridge doctor
```

All checks should pass. If SP connectivity fails, remind the user to:
1. Open Super Productivity desktop app
2. Enable Local REST API: Settings → Sync & Export → Local REST API
3. Re-run doctor

---

## Phase 5: Report

Print a final summary:

```
Setup complete
──────────────
Bridge:     installed (sp-local-bridge vX.Y.Z)
Host:       <host> configured
Config:     <path-to-config-file>
Skills:     ~/.agents/skills/sp-local-bridge-setup → linked
SP status:  ✓ connected / ⚠ not running (start app when ready)

Available tools (16):
  list_tasks, get_task, create_task, update_task, complete_task, uncomplete_task,
  start_task, stop_current_task, get_current_task, set_current_task,
  archive_task, restore_task,
  list_projects, list_tags, get_status, health

Next: try asking me to "list my tasks" or "create a task called Test".
```
