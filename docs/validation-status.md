# Validation Status

Current validation state for the Go bridge across platforms and host applications.

Each row records what was actually exercised, and when. A version in a cell is
the version that was validated, not necessarily the latest release.

## Platform Matrix

| Platform | CI Tests | Artifact Validation |
|----------|----------|---------------------|
| Linux x86_64 | `ubuntu-latest` | v0.3.3 validated (2026-09-07) |
| Linux arm64 | Not run | Archive checksum verified only |
| macOS arm64 | `macos-latest` | Archive checksum verified only |
| macOS x86_64 | Not run | Archive checksum verified only |
| Windows x86_64 | `windows-latest` | Archive checksum verified only |
| Windows arm64 | Not run | Archive checksum verified only |

The CI matrix is three runner images — `ubuntu-latest`, `macos-latest` and
`windows-latest` — so three of the six published targets are exercised by CI and
three are built but never run there. The rows name the image rather than assert
which architecture each currently maps to.

"Archive checksum verified only" means the published archive was downloaded and
matched `checksums.txt`, but the binary inside it has not been run on that
platform. All six v0.3.3 archives were verified this way on 2026-09-07; Linux
x86_64 additionally got the full validation below.

## Host Application Validation

| Host | Config Generation | Config Write | Live Host Session |
|------|-------------------|--------------|-------------------|
| Claude Code | Tested (automated) | Tested (automated) | Not yet validated |
| Claude Desktop | Tested (automated) | Tested (automated) | Not yet validated |
| VS Code Copilot | Tested (automated) | Tested (automated) | v0.1.1 (2026-05-31) |
| Codex CLI | Tested (automated) | Tested (automated) | Not yet validated |

"Live Host Session" means a human has connected the Go bridge to the specific
host application and confirmed MCP tool invocations work end-to-end through the
host's MCP client. This is distinct from raw stdio protocol validation below.
No live host session has been run against v0.3.3.

## Release Artifact Validation (v0.3.3)

Linux x86_64, 2026-09-07, against the published release
(`sp-local-bridge 0.3.3`, commit `d5697cd`, built `2026-09-07T22:42:04Z`).
The binary was removed before installing, so this exercised a real install
rather than an overwrite.

- Release job completed successfully; all six platform archives and
  `checksums.txt` published
- All six archives downloaded and verified: `sha256sum -c checksums.txt`
  reports OK for each
- `scripts/install.sh` downloaded, checksum-verified, extracted and installed
  the binary, and created the four multicall alias symlinks — confirmed with
  `ls -la`, which shows four symlinks to `sp-local-bridge` carrying the install
  timestamp. `doctor`'s alias check is not evidence for this: it stats four
  filenames, so a regular file of the right name passes, and it warns without
  failing the run
- The installed binary is byte-identical to the separately downloaded archive
  contents (same SHA-256)
- `--version` reports `0.3.3` with the release commit and build date
- `doctor` passes every check: PATH visibility, host configs, health, status,
  task list, MCP self-check (16 tools). It also reports the multicall aliases,
  which is advisory — that line warns but never fails the run
- `doctor --deep` reports store integrity OK against a live store of 198 active
  and 17 archived tasks, with all 198 referenced by the project and tag indexes
  or as a subtask of another task, which is the reference set the check builds:
  no dangling references, orphaned entities, duplicates or unresolved
  anomalies

  The active count is down from 284 at v0.3.0. That is accounted for: the owner
  moved a block of work tasks to a separate tracker between the two runs. It is
  recorded here because `doctor --deep` reporting OK does not by itself
  distinguish a store that shrank deliberately from one that lost data — the
  check verifies that the indexes and the entity set agree with each other,
  which they would either way
- `configure --dry-run` generates config for all four hosts and writes nothing:
  `claude-code`, `claude-desktop`, `vscode-copilot`, `codex`
- Raw MCP stdio: `initialize` returns protocol `2024-11-05` and
  `serverInfo {"name":"sp-local-bridge","version":"0.3.3"}`; `tools/list`
  returns all 16 tools, none of them a delete, with `limit` and `offset`
  present on `list_tasks`, `list_projects` and `list_tags`

### Argument handling: the six cases exercised (v0.3.2)

Run against the installed release binary with `HOME` pointed at a throwaway
directory, so a write would have been visible and nothing real was touched. The
file count after each run is what distinguishes a rejection from a silent write.

These six are a sample, not the whole change. `CHANGELOG.md` names three
further inputs that now exit 2 — a bare `-`, the `--flag=value` form, and an
empty-string second positional — and `print-config` received the identical
fix. None of those were re-run for this entry.

These rows were measured against v0.3.2 and have not been re-run since. They
are kept because the behaviour is unchanged in v0.3.3, and dropping them would
lose the only written record of it.

### Access token handling (v0.3.3)

The subject of this release. Run against the installed release binary with
`SP_BASE_URL` pointed at a server reproducing Super Productivity 18.19.0 and
newer — `GET /health` unauthenticated, every other route 401 with SP's own body
and `WWW-Authenticate: Bearer`.

| Token state | Outcome |
|---|---|
| SP's own token file | `read from <path>`; all checks pass |
| `SP_API_TOKEN` set | `set (SP_API_TOKEN)`; all checks pass |
| none found | `none found`; 401s, and names the cause |
| rejected by SP | 401, reported as stale rather than as missing |
| unusable value | reported unusable; `Health check... OK` |

The last row is the one worth keeping: a bad token cannot make a reachable
Super Productivity look unreachable, because `net/http` would otherwise refuse
to send the request at all — including to `/health`, which needs no token.

`tasks list` returned task data and MCP `tools/call list_tasks` returned
`isError: false` with content, so this is not a `doctor`-only result.

The token appears in none of `doctor`, `doctor --json`, `doctor --deep`,
`status`, `tasks list`, or `--version`.

| Command | Exit | Files written |
|---------|------|---------------|
| `configure --dry-runn claude-code` | 2 | 0 |
| `configure claude-code claude-desktop` | 2 | 0 |
| `configure -- claude-code --dry-run` | 2 | 0 |
| `configure --dry-run claude-code` | 0 | 0 |
| `configure -- claude-code` | 0 | 1 |
| `configure claude-code` | 0 | 1 |

The first three are the fixes (#53, #60, and the hazard the `--` change
introduced). The last three are the controls: a real `--dry-run` still previews
without writing, and both the bare and `--`-prefixed forms still configure.

## Live Client Validation (v0.3.3)

`make test-live` against a running Super Productivity on Linux x86_64,
2026-09-07. Read-only: the suite issues GET requests only, including a lookup
of a non-existent task id and of a non-existent route. It does not create,
modify, archive or delete anything.

- `TestLive_TaskFields` — every field the client depends on is present with the
  expected type, in both the active and archived pools
- `TestLive_ProjectFields`, `TestLive_TagFields`, `TestLive_StatusAndHealthFields`
  — same check for the remaining entity types
- `TestLive_NotFoundCodesAreDistinct` — a missing task and a missing route still
  report different error codes, which is the bug #37 fixed
- `TestLive_FixturesDoNotInventFields` — all seven committed success-response
  fixtures claim only fields SP actually returns. Fields that are null
  throughout the store have their presence checked but not their type, which
  the run reports
- `TestLive_StoreHasSomethingToCheck` — guards against the suite passing
  vacuously against an empty store

## Validation Against Current Super Productivity

Everything above is measured against the maintainer's Super Productivity
18.10.0. That is no longer the only answer.

`.github/workflows/upstream-live.yml` builds Super Productivity from source
daily and runs the live suite against it headless, for both the latest release
and `master`. First run: **2026-09-08, both targets passing** against `v18.21.2`
and `master` (workflow run `34184837194`).

That run also confirms the token handling above against the real thing rather
than a stand-in: the job asserts an unauthenticated `/tasks` returns 401 before
running the suite, so a pass means the bridge authenticated against a Super
Productivity that genuinely enforces a token.

It exists because nothing else could see Super Productivity change. SP began
requiring an access token in 18.19.0 and every published release of the bridge
was broken against it for about a month, with CI green throughout — correctly,
since CI tests fixtures and a stub, both of which encode what we believe SP
does.

**The limit worth stating:** the daily job verifies the bridge's *code* against
current Super Productivity. It does not validate a published *artifact* on any
platform, and it runs only on `ubuntu-latest`. The artifact validation above is
still Linux x86_64 by hand.

## What "Tested" Means

- **Config Generation**: Automated tests run `print-config` for the host and
  assert it succeeds. They check the exit status, not the content of the output.
- **Config Write**: JSON configs are parsed before mutation; Codex TOML uses structural guard tests plus backup and atomic-write tests. For `claude-code` this additionally covers preserving foreign keys and large integers in `~/.claude.json`.
- **Artifact Validation**: Release binary downloaded, checksum-verified, installed, and exercised via raw MCP stdio protocol.
- **Live Host Session**: A human has connected the bridge inside the actual host application and confirmed tool invocations work through the host's MCP client.

## VS Code Copilot Host Validation (v0.1.1)

Historical. Live host session validated on Linux x86_64, 2026-05-31, against
v0.1.1. Not repeated for v0.3.3.

- Binary: v0.1.1 installed via `scripts/install.sh` to `~/.local/bin`
- Config: `sp-local-bridge configure vscode-copilot` wrote to `~/.config/Code/User/mcp.json`
- Tool discovery: VS Code Copilot discovered all 16 MCP tools after window reload
- Live invocations confirmed through Copilot's MCP client:
  - `get_status`: returned task count and current task state
  - `list_tasks`: returned full task list with metadata (titles, projects, time tracking)

## Parity with Python Bridge

The Go bridge implements all 16 MCP tools from the Python bridge, whose last
release was v0.2.2. That project is archived and read-only, and its PyPI
releases are yanked; the Go bridge is where the work continues. Black-box MCP
tests check the request and response shapes against expectations written by
hand in this repository; nothing here compares output against a running Python
bridge.
