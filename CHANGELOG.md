# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `claude-code` host target for `configure` and `print-config`, writing user
  scope in `~/.claude.json`. Claude Code is a separate host from Claude Desktop
  and was previously not configurable at all (#25)
- `install.sh --from-source` (also `SP_FROM_SOURCE=1`) builds and installs the
  local checkout. Previously running the script from a checkout silently
  installed the latest published release instead (#23)
- `hostcfg.ConfigTargets()`, so `doctor` inspects host configs from the same
  table `configure` writes from instead of a private copy

### Fixed

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
- Doubled `v` in `install.sh`'s success line when installing from source

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
