# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Complete Go rewrite — single static binary, zero runtime dependencies
- MCP stdio server (hand-rolled JSON-RPC 2.0, protocol version 2024-11-05)
- 16 MCP tools: task CRUD, time tracking, projects, tags, health, status
- CLI with subcommands: health, status, tasks, projects, tags
- Host configuration: claude-desktop, vscode-copilot, codex
- `configure` command with --dry-run and --remove
- `print-config` command with --absolute/--bare
- Surgical TOML editing for Codex config (preserves other entries)
- Atomic file writes with backup (.bak) for all host config formats
- Doctor diagnostics: health, status, PATH, host config, task smoke test
- Multicall binary (symlink as sp-local-bridge-mcp, etc.)
- Cross-platform CI (Linux, macOS, Windows)
- GoReleaser v2 release automation
- Install script with fail-closed SHA256 checksum verification
- Integer validation without float64 precision loss

### Changed

- Rewritten from Python to Go for single-binary portability
- Install default changed from /usr/local/bin to ~/.local/bin

### Removed

- Python runtime dependency
- uv/pip packaging
- Entry point scripts (replaced by multicall binary)

## [0.2.0] - 2026-05-31

Python release (separate repo). See [super-productivity-local-bridge](https://github.com/CameronBrooks11/super-productivity-local-bridge) for history.

[Unreleased]: https://github.com/CameronBrooks11/super-productivity-local-gobridge/commits/main
[0.2.0]: https://github.com/CameronBrooks11/super-productivity-local-bridge/releases/tag/v0.2.0
