# AGENTS.md

Instructions for AI coding agents working in this repository.

## Project

Super Productivity Local Go Bridge — a local automation bridge for the Super Productivity desktop app, rewritten in Go for single-binary portability. Uses the SP Local REST API (`http://127.0.0.1:3876`) as the primary app-control path, with MCP as one thin host adapter.

## Stack

- **Go 1.23+** — zero external dependencies
- **Standard library** for HTTP, JSON, testing, and concurrency
- **Hand-rolled JSON-RPC 2.0** for MCP over stdio
- **GoReleaser v2** for release automation
- **GitHub Actions** for CI (Linux, macOS, Windows matrix)

## Layout

```
cmd/sp-local-bridge/       Entry point (multicall + subcommand dispatch)
internal/
  bridge/                  Core types, errors, validation, REST client, service
  cli/                     CLI command handling
  mcpadapter/              MCP stdio server adapter
  doctor/                  Connectivity and environment diagnostics
  hostcfg/                 Host app configuration writer (JSON + TOML)
  version/                 Build-time version info
scripts/                   Install/uninstall scripts
testdata/fixtures/         JSON response fixtures for tests
docs/                      Documentation + host guides
```

## Commands

```sh
make build                 # Build binary
make test                  # Run tests
make test-cover            # Tests with coverage
make check                 # Format check + vet + test (non-mutating)
make race                  # Run tests with race detector
make test-live             # Verify the client against a running SP (read-only)
make fmt                   # Format code (mutates files)
make clean                 # Remove build artifacts
go test ./... -count=1     # Run all tests
gofmt -l .                 # Check formatting (should be empty)
go vet ./...               # Static analysis
```

## Conventions

- All Go code lives under `cmd/` and `internal/` (standard Go layout).
- Zero external dependencies — stdlib only.
- MCP adapter is thin — all logic lives in `internal/bridge/`.
- No Claude/agent-specific language in tool descriptions or core code.
- Use SP-native camelCase field names at REST boundaries (`projectId`, `tagIds`).
- Type annotations on all exported functions.
- Tests in `internal/*/` alongside the code they test.
- Integer fields parsed via `strconv.ParseInt` from raw JSON (no float64).
- Atomic file writes with backup for host config mutations.
- JSON configs parsed before modification; TOML uses structural guard (not a full parser).

## Fixtures

`testdata/fixtures/*.json` are hand-written, not recordings. They must stay
synthetic — this is a public repository and real responses carry task titles,
project names and notes — but they must be **shape-accurate**: no field SP does
not return, and no type SP does not use.

This has bitten repeatedly. The tag fixture used `name` where SP sends `title`;
the health fixture invented `status` and a test asserted it, so fixture and test
were wrong together and the suite stayed green; task fixtures carried a
`plannedAt` that never appears in a response.

`make test-live` checks every response fixture the read-only suite can reach
against a running SP, and fails on any field SP does not return. Request
fixtures (`*-request.json`) and error fixtures are not checked — they describe
what we send, or a failure a read-only run cannot provoke. Run it after touching a fixture, and before a
release. There is deliberately no auto-update mode: regenerating from a live
store would commit personal data.

## Testing against a running Super Productivity

The Local REST API talks to the user's real task database. There is no dry-run
mode and no undo.

- **Read-only against a live app.** `GET` requests only. Never send `POST`,
  `PATCH`, or `DELETE` to an app holding real data.
- **Writes go to a throwaway profile.** Launch a scratch instance with
  `superproductivity --user-data-dir=/tmp/sp-scratch` and point the bridge at
  it. Note that the API port is a hardcoded 3876 and SP takes a single-instance
  lock, so the real app must be closed first.
- **Never probe an unknown handler with a fake ID.** A non-existent ID only
  bounds the damage if the handler rejects unknown IDs, which is exactly what an
  unknown handler has not been shown to do. Read the route's behaviour from the
  source instead.

This is not hypothetical. Probing `DELETE`/`archive` with a non-existent ID
against a live store crashed an NgRx effect in the host app, left its in-memory
store inconsistent (223 of 277 task entities dropped while every index still
referenced them), and caused the periodic backup writer to persist that corrupt
state over good snapshots. See #27, and #28 for what it prompted here.

Both halves are now reported upstream, which is the durable record of why this
rule exists:

- `super-productivity/super-productivity#9946` — a missing id yields a phantom
  task object with no `id`, so every `if (!task)` guard downstream is inert and
  `deleteTaskHelper` matches every top-level task. Shut at the REST entry point
  from SP `v18.15.0`; still reachable through the Plugin API on `master`. That
  check covers two known lookup helpers, not every route, so the rule above
  stands whatever version you are pointed at.
- `super-productivity/super-productivity#9945` — the backup writer serialises
  the resulting inconsistent store 30 seconds later and it becomes the newest
  file in the rotation.
- `super-productivity/super-productivity#9947` — the Local REST API logs no
  requests, which is why correlating the two took a seeded scratch profile.

## Review gate

Every PR with a human author passes an independent review before merge, capped
at three rounds. Run it after CI is green on the PR head SHA, and treat a
finding as open until it is either fixed or refuted with evidence. Dependency
bumps are exempt — CI is the gate there.

The review lives in commit messages and PR bodies, not in GitHub's review UI, so
`gh pr view --json reviews` returns nothing for PRs that were thoroughly
reviewed. Do not read that as evidence either way.

It is worth the cost because of what it actually finds. Across this repo's
history the gate has run on PRs containing **no code changes at all** and still
found real problems, and the finding is almost always the same one: **a claim
written from something that was not executed.** A comment explaining a rationale
that does not hold, a CHANGELOG entry describing behaviour nobody ran, a doc
snippet that was never pasted into a shell, a commit message asserting a
measurement taken from memory.

So the rule the gate enforces is narrow and mechanical: **anything stated as
fact carries the command that produced it.** An estimate is fine when labelled;
a guess presented as a measurement is not.

### A passing test proves nothing until you have seen it fail

Break the code deliberately and confirm the test catches it. This is cheap —
usually one `sed`, run the suite, restore — and it is the only thing that
distinguishes a real guard from a test that merely runs. One of the four below
was found by the mutation sweep itself; the other three were spotted by reading
and would have stayed arguable without it. Both are worth the minute:

- an assertion loop that could never fail, because the values it searched for
  were printed by an earlier part of the same output regardless
- a fixture that never reached its second pass, so the assertions it guarded
  executed zero times while the test reported PASS
- a category whose deletion left the whole suite green, while deleting its two
  siblings failed 13 and 2 tests — coverage that looked symmetric and was not
- a helper comment describing a failure mode that could not happen, next to a
  real one it did not mention

Measure every case rather than generalising from one. "Deleting a category from
that list now fails two tests" was written after checking a single category. The
full sweep was `dangling 13 | orphaned 0 | duplicated 2` — the quoted count was
right for exactly one of the three, and the category that failed nothing was
precisely the one with no coverage. The gap survived the commit that claimed to
close it. Print the matrix; it is three numbers and it cannot be misread.

## Do NOT

- Add runtime deps without discussing (the dep tree is intentionally zero).
- Put business logic in the MCP adapter.
- Reference specific AI hosts (Claude, Cursor, etc.) outside `docs/hosts/`.
- Create files named after phases or milestones.
- Commit `working/`, temp review files, or build artifacts.
- Use `float64` for integer JSON fields.
- Expose `task.delete` at any layer.
- File GitHub issues off your own initiative — write to `working/feedback/`
  first (see below).

## Reporting Issues

If you encounter unexpected behavior while using the SP bridge MCP tools or CLI,
write a concise report to `working/feedback/<descriptive-slug>.md` with:

- **What was attempted** (command or tool call)
- **What happened** (exact output or error)
- **What was expected**
- **Version** (`sp-local-bridge --version`)

`working/feedback/` is the default and the starting point, not a dead end. The
maintainer reviews it and decides what becomes an issue.

**A security finding never becomes a public issue, whoever asked.** It goes to
the private channel in `SECURITY.md` — that document's first instruction is "Do
not open a public GitHub issue". This overrides everything below: a
vulnerability is a behavioural finding reproducible against a released build,
so it satisfies the bullets and must still not be filed. Filing it publicly is
the disclosure.

Otherwise, **file directly only when the maintainer has asked**, and only with
the evidence that fits the claim:

- **A behavioural finding** — something this tool does — is reproduced against a
  released build, not a branch. A branch repro says nothing about what anyone is
  running.
- **A claim about the repository** — a coverage gap, a CI matrix that misses a
  target, a version string that drifts — carries the command that reads it.
  #47 and #48 have no released build to run against; #46's drift is visible in
  a tag. Requiring a runtime repro would just block them.
- **A finding about someone else's software** — a dependency, or Super
  Productivity itself — is reproduced against *its* released build, and checked
  against its default branch too when the question is whether a fix has already
  landed. Both halves matter: upstream #9946 could say "live in the current
  release, not already fixed and awaiting a version bump" only because it was
  verified against `v18.21.2` **and** `master`.

Write the `working/feedback/` note first either way. That step is not ceremony:
it is where the claim gets stated precisely enough to test, and re-running it on
the way to the issue is what turns a reasoned consequence into a measured one.
#53's note had already worked out that a `--dry-runn` typo would write the
config for real; re-running it against v0.3.1 is what let the issue *show* the
file being written instead of arguing that it would be.

Before filing, search the tracker — `gh issue list --state all --search '<terms>'`.
Annotating the note with the issue number afterwards is worth doing, but it
cannot be the duplicate check: `working/` is gitignored, so a fresh clone or
another machine has none of those notes.

Never open an issue on a repository that is not ours without explicit
per-issue approval. Before filing anywhere external, read that project's
`CONTRIBUTING`, code of conduct and issue templates — `blank_issues_enabled:
false` means free-form issues are refused and the report has to fit their form,
and some projects have rules about AI-assisted contributions. Check; do not
assume either way.
