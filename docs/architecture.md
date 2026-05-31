# Architecture

## Overview

```
MCP host / CLI / future adapter
  └─ thin adapter layer
      └─ core service (validation, dispatch)
          └─ REST client
              └─ Super Productivity desktop app (127.0.0.1:3876)
```

MCP is a **host adapter**, not the business logic layer. All operation logic lives in `internal/bridge/`.

## Package Structure

| Package | Responsibility |
|---------|---------------|
| `cmd/sp-local-bridge` | Entry point, multicall dispatch |
| `internal/bridge` | Core types, validation, REST client, service |
| `internal/cli` | CLI command parsing and dispatch |
| `internal/mcpadapter` | MCP JSON-RPC server over stdio |
| `internal/doctor` | Diagnostics and environment checks |
| `internal/hostcfg` | Host config file writer (JSON + TOML) |
| `internal/version` | Build-time version injection |

## Design Decisions

### Zero Dependencies

The binary has zero external Go module dependencies. This keeps the supply chain minimal and the binary small.

### No MCP SDK

The MCP server is hand-rolled JSON-RPC 2.0. This avoids adding a dependency for a relatively simple protocol (line-delimited JSON on stdio). The trade-off is tested more rigorously via black-box subprocess tests.

### No float64 for Integers

JSON numbers are parsed from `json.RawMessage` using `strconv.ParseInt`. This avoids the common Go pitfall of `encoding/json` unmarshaling numbers to `float64`, which loses precision above 2^53.

### Multicall Binary

A single binary responds to multiple names via `os.Args[0]`:
- `sp-local-bridge` — full CLI
- `sp-local-bridge-mcp` — starts MCP server directly
- `sp-local-bridge-doctor` — runs diagnostics
- `sp-local-bridge-configure` — host config writer
- `sp-local-bridge-print-config` — prints config snippets

### Config Safety

For JSON configs, existing files are parsed before modification; malformed JSON causes an error exit. For Codex TOML configs, a structural guard checks table headers and key=value lines before surgical editing — this is not a full TOML parser, but it catches obvious corruption. Backups are created before every write.

### Atomic Writes

All config file writes use temp file + rename, ensuring readers never see partial content.

## Data Flow

1. Input arrives via CLI args or MCP JSON-RPC stdin
2. Adapter translates to a `bridge.Request{Operation, Payload}`
3. Service validates payload, dispatches to handler
4. Handler calls REST client
5. REST client sends HTTP to SP, translates response
6. Result flows back through the adapter to the caller
