# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `limit`, `offset` and `full` on `list_tasks`, `list_projects` and `list_tags`
  (`--limit`, `--offset`, `--full` on the CLI). SP ignores paging and field
  selection, so the bridge applies them. There is no default limit: a silently
  capped list would be a wrong answer to "how many tasks do I have". When a
  limit does cut a list, the response reports `truncated`, `returned` and
  `matched`, so a short list is never mistaken for a complete one (#29)
- List responses carry a compact field set by default, dropping fields nothing
  consumes — the per-day time map that grows for the life of a task, theme
  colours, worklog export column lists. `full` returns entities untouched.
  On a 284-task store `list_tasks` with `includeDone` goes from ~24k tokens to
  ~18.5k compact, or ~1.5k with `limit=20`; `list_projects` from ~3.9k to ~270
  (#29)
- CLI time flags accept durations: `--time-estimate 1h30m`, `90m`, `2d`. A bare
  integer is still milliseconds, so existing scripts are unaffected (#10)

- `make test-live`, a build-tagged suite that checks the client against a running
  Super Productivity: that every field the bridge depends on is still present
  with the expected type, that a missing task and a missing route still report
  different codes, and that no committed fixture claims a field SP never
  returns. Excluded from `go test ./...` and from CI, which has no SP to reach,
  though CI type-checks it with `go vet -tags live` so a rename cannot break it
  unnoticed.
  Read-only (#30)
- Response fixtures now carry the `{"ok":true,"data":...}` envelope SP actually
  sends. As bare arrays they exercised a branch of `translateResponse` that a
  real response never takes (#30)

- `doctor --deep` cross-references task entities against the project and tag
  indexes that point at them, reporting dangling references and orphaned active
  tasks. `--json` prints only that report. Exit code 3 distinguishes "SP
  answered but its data is inconsistent" from a failed check. Also flags tasks
  present in both the active and archived pools, which is a partially applied
  archive or restore. Anomalies are confirmed by a second pass before being
  reported, so a store edited mid-check is not called corrupt (#28)
- `bridge.NewClientWithTimeout`, so a caller needing longer than the default
  10s per-request timeout can raise it. `http.Client.Timeout` caps every
  request regardless of the context deadline
- `claude-code` host target for `configure` and `print-config`, writing user
  scope in `~/.claude.json`. Claude Code is a separate host from Claude Desktop
  and was previously not configurable at all (#25)
- `install.sh --from-source` (also `SP_FROM_SOURCE=1`) builds and installs the
  local checkout. Previously running the script from a checkout silently
  installed the latest published release instead (#23)
- `hostcfg.ConfigTargets()`, so `doctor` inspects host configs from the same
  table `configure` writes from instead of a private copy

### Fixed

- Corrected fixtures that described responses Super Productivity does not send:
  the tag fixture used `name` where SP sends `title`, the health fixture
  invented a `status` field that a test then asserted (so both were wrong
  together and the suite stayed green), the status fixture invented
  `currentProject`, and task fixtures carried a `plannedAt` that never appears
  in a response (#30)

- Every HTTP 404 was reported as `TASK_NOT_FOUND`. The client short-circuited on
  the status before reading the body, discarding the distinction Super
  Productivity draws there between a missing task (`TASK_NOT_FOUND`) and a
  missing route (`NOT_FOUND`). A mistyped or removed route reported "task not
  found", and `archive`'s existence guard could not tell an absent task from an
  absent probe route. SP's own code and message now pass through; a 404 with an
  empty or non-JSON body reports `SP_ERROR` rather than guessing (#37)
- An HTTP error status carrying `{"ok":true}` is reported as an error rather
  than a success. Believing the body over the status turned a failed request
  into a successful one — and for `archive`, whose guard reads a task to decide
  whether it exists, a 404 would have been read as "it exists" and the archive
  attempted anyway (#37)
- `archive_task` only archives when its existence probe returns the task itself.
  A successful status alone was enough before, so an empty body, a non-JSON body
  or `{"ok":true,"data":null}` all read as "the task exists" and the archive was
  attempted for an id never confirmed (#37)
- `archive_task` / `tasks archive` verifies the task exists before archiving and
  returns `TASK_NOT_FOUND` otherwise. Super Productivity's archive endpoint
  reports success for ids that never existed, unlike every other single-task
  route, so a mistaken or invented id was reported as a completed archive (#27)
- `doctor --deep` treats a missing or malformed index field as unreadable rather
  than as an empty index. Reading one as zero references turned every task it
  owned into an orphan, reporting a healthy store as corrupt — and
  deterministically, so it survived both confirmation passes. The run now
  degrades to unconfirmed, keeping the pool counts and `duplicated`, which no
  index can invalidate (#33)
- `doctor` rejects its own subcommand word as an argument at any position. It
  was tolerated at index 0, where the multicall alias puts it, so
  `sp-local-bridge-doctor doctor` ran a shallow check and printed "All checks
  passed"
- `doctor` names the first unrecognised argument rather than the last, and an
  argument that is itself the empty string is now reported instead of ignored

- `--source archived` needing `--include-done` is now documented in CLI help,
  the `list_tasks` MCP tool description, and the operations reference. SP
  applies the done filter to the archived pool whatever a task's `isDone`
  value, so `--source archived` alone returns an empty list and reads as a
  failed archive (#24, follow-up to #8)
- Host config JSON is no longer round-tripped through `float64`. Integers above
  2^53 were silently rewritten, which matters now that `configure` edits
  `~/.claude.json` alongside unrelated state
- `sortedHostNames()` derived from the hosts map rather than a hardcoded list.
  The two had already drifted, which would have hidden `claude-code` from
  `--help`, the unknown-host error, and doctor detection
- Doubled `v` in `install.sh`'s success line when installing from source. The
  version is now printed bare unless it looks like a version number, since
  `git describe --always` returns a SHA on a shallow or tagless clone
- Host config `.bak` files inherit the source file's permissions instead of a
  hardcoded `0644`. `~/.claude.json` is commonly `0600` and holds account
  identifiers, so the backup was widening access to them
- `readJSON` rejects data after the top-level JSON value. `json.Decoder` stops
  at the first value where `json.Unmarshal` did not, so a config corrupted by a
  doubled write would have been silently rewritten with the remainder dropped
- `configure` and `configure --remove` abort if the config file changes between
  read and write, instead of reverting whatever wrote it. Claude Code rewrites
  `~/.claude.json` while it runs, so a whole-file rewrite from a stale read can
  lose its state
- Restored the `%APPDATA%` fallback that the doctor host table used to carry.
  With `APPDATA` unset, the Windows config paths resolved relative to the
  working directory
- `install.sh --from-source` fails with a clear message when piped from stdin
  (`curl | bash`), where `BASH_SOURCE` is unset and there is no checkout to build

## [0.1.0] - 2026-05-31

### Added

- Complete Go rewrite — single static binary, zero runtime dependencies
- MCP stdio server (hand-rolled JSON-RPC 2.0, protocol version 2024-11-05)
- 16 MCP tools: task CRUD, time tracking, projects, tags, health, status
- Structured content in MCP tool results (structuredContent field)
- CLI with subcommands: health, status, tasks, projects, tags
- Host configuration: claude-desktop, vscode-copilot, codex
- `configure` command with --dry-run and --remove
- `print-config` command with --absolute/--bare
- Surgical TOML editing for Codex config (preserves other entries)
- Atomic file writes with backup (.bak) for all host config formats
- Doctor diagnostics: health, status, PATH, host config, task smoke test
- Doctor MCP self-check (spawns binary, verifies 16 tools via protocol)
- Doctor multicall alias validation
- Doctor line-based TOML parse for Codex config detection
- Multicall binary (symlink as sp-local-bridge-mcp, etc.)
- VitePress documentation site (architecture, operations, hosts, security)
- Setup skill for guided agent-driven installation
- Cross-platform CI (Linux, macOS, Windows) with docs build
- GoReleaser v2 release automation
- GitHub Pages deployment workflow
- Install script with fail-closed SHA256 checksum verification
- Integer validation without float64 precision loss
- Dependabot for Actions, Go modules, and npm
- .editorconfig for consistent formatting

### Changed

- Rewritten from Python to Go for single-binary portability
- Install default changed from /usr/local/bin to ~/.local/bin

### Removed

- Python runtime dependency
- uv/pip packaging
- Entry point scripts (replaced by multicall binary)

## [0.2.0] - 2026-05-31

Python release (separate repo). See [super-productivity-local-bridge](https://github.com/CameronBrooks11/super-productivity-local-bridge) for history.

[Unreleased]: https://github.com/CameronBrooks11/super-productivity-local-gobridge/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/CameronBrooks11/super-productivity-local-gobridge/releases/tag/v0.1.0
[0.2.0]: https://github.com/CameronBrooks11/super-productivity-local-bridge/releases/tag/v0.2.0
