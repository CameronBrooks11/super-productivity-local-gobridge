# Operations Reference

The bridge exposes 16 operations, available via both MCP tools and CLI commands.

## Task Operations

| Operation | MCP Tool | CLI Command | Description |
|-----------|----------|-------------|-------------|
| `task.list` | `list_tasks` | `tasks list [filters]` | List tasks with optional filters |
| `task.get` | `get_task` | `tasks get <id>` | Get a task by ID |
| `task.create` | `create_task` | `tasks add <title> [flags]` | Create a new task |
| `task.update` | `update_task` | `tasks update <id> [flags]` | Update a task |
| `task.complete` | `complete_task` | `tasks complete <id>` | Mark as done |
| `task.uncomplete` | `uncomplete_task` | `tasks uncomplete <id>` | Mark as not done |
| `task.start` | `start_task` | `tasks start <id>` | Start time tracking |
| `task.stop_current` | `stop_current_task` | `tasks stop-current` | Stop current tracking |
| `task.get_current` | `get_current_task` | `tasks current` | Get currently tracked task |
| `task.set_current` | `set_current_task` | `tasks set-current <id>` | Set current task |
| `task.archive` | `archive_task` | `tasks archive <id>` | Archive a task |
| `task.restore` | `restore_task` | `tasks restore <id>` | Restore from archive |

## Other Operations

| Operation | MCP Tool | CLI Command | Description |
|-----------|----------|-------------|-------------|
| `project.list` | `list_projects` | `projects list` | List projects |
| `tag.list` | `list_tags` | `tags list` | List tags |
| `status.get` | `get_status` | `status` | Get app status |
| `bridge.health` | `health` | `health` | Check connectivity |

## Task Create Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `title` | string | yes | Task title |
| `notes` | string | no | Task notes |
| `projectId` | string \| null | no | Project to assign |
| `tagIds` | string[] | no | Tags to assign |
| `parentId` | string | no | Parent task (creates subtask) |
| `plannedAt` | string \| int | no | Planned date (ISO or epoch ms) |
| `dueDay` | string \| null | no | Due date (YYYY-MM-DD) |
| `dueWithTime` | int \| null | no | Due timestamp (epoch ms) |
| `isDone` | boolean | no | Initial completion state |
| `timeEstimate` | int ≥ 0 | no | Time estimate (ms) |
| `timeSpent` | int ≥ 0 | no | Time already spent (ms) |

::: warning
`parentId` cannot be combined with `projectId` or `tagIds` (subtasks inherit from parent).
:::

## Task Update Fields

Same as create except: `parentId` is not allowed on update.

## Task List Filters

| Filter | Type | Description |
|--------|------|-------------|
| `query` | string | Title substring (case-insensitive) |
| `projectId` | string | Filter by project |
| `tagId` | string | Filter by tag (`TODAY` for today's tasks) |
| `includeDone` | boolean | Include completed tasks |
| `source` | `active` \| `archived` \| `all` | Task pool (default: `active`) |

::: tip
`source=all` expands the pool to include archived tasks but does **not** automatically show completed tasks.

**Both `source=archived` and `source=all` require `includeDone=true`.** Super Productivity applies the done filter to the archived pool regardless of a task's own `isDone` value, so a task archived while still open is invisible to `source=archived` on its own:

```console
$ sp-local-bridge tasks archive <id>
{ "archived": true, "id": "<id>" }

$ sp-local-bridge tasks list --source archived
[]

$ sp-local-bridge tasks list --source archived --include-done
# the task is here
```

An empty result from `--source archived` therefore does not mean the archive failed. This is Super Productivity's own filtering — the bridge passes `source` and `includeDone` through to the REST API unchanged.

The `taskCount` in `get_status` reflects the active pool including done tasks (not all tasks across all sources).
:::

## Error Codes

| Code | Meaning |
|------|---------|
| `SP_UNAVAILABLE` | Cannot connect to SP Local REST API |
| `TIMEOUT` | Request timed out |
| `UNKNOWN_OPERATION` | Operation not recognized |
| `UNSUPPORTED_OPERATION` | Operation exists but is not implemented |
| `INVALID_INPUT` | Payload validation failed |
| `TASK_NOT_FOUND` | Task ID not found |
| `NOT_FOUND` | The route does not exist — not the same as a missing task |
| `PROJECT_NOT_FOUND` | Project ID not found |
| `SP_ERROR` | SP returned an error |
| `INTERNAL_ERROR` | Unexpected bridge error |

### Two kinds of 404

Super Productivity answers both a missing task and a missing route with HTTP
404, distinguishing them only in the body:

```console
$ curl -s http://127.0.0.1:3876/tasks/GHOST_ID
{"ok":false,"error":{"code":"TASK_NOT_FOUND","message":"Task not found"}}

$ curl -s http://127.0.0.1:3876/definitely-not-a-route
{"ok":false,"error":{"code":"NOT_FOUND","message":"Route not found"}}
```

The bridge reads the body and passes SP's own code through, so `TASK_NOT_FOUND`
means the task is genuinely absent. Earlier versions reported every 404 as
`TASK_NOT_FOUND`, which sent anyone debugging a mistyped or removed route
looking for a task that was never the problem.

A 404 whose body is empty or not JSON — something a proxy might produce, but not
SP — reports `SP_ERROR` rather than guessing which was missing.

## Intentionally Excluded

- **`task.delete`** — Destructive operation excluded by design. Use archive/restore instead.

## Store integrity (`doctor --deep`)

`doctor` on its own checks that the bridge can reach Super Productivity. It does
not check that the data coming back is coherent — a corrupt store answers every
liveness check perfectly.

`doctor --deep` additionally cross-references task entities against the indexes
that point at them (`project.taskIds`, `project.backlogTaskIds`, `tag.taskIds`,
`task.subTaskIds`) and reports three conditions:

| Condition | Meaning |
|---|---|
| **dangling** | A project or tag references a task that does not exist in any pool. |
| **orphaned** | An active task exists that nothing references; it may be invisible in the UI. |
| **duplicated** | A task appears in both the active and archived pools — an archive or restore was only partially applied. |

Archived tasks are exempt from the orphan check: archiving removes a task from
`project.taskIds`, so an unreferenced archived task is normal. They are still
loaded, because a project may legitimately reference a task that now lives in
the archive.

```console
$ sp-local-bridge doctor --deep
...
Store integrity... OK
  active tasks        : 277
  archived tasks      : 17
  referenced by index : 277
```

`--json` prints only the report (and implies `--deep`), for scripting:

```console
$ sp-local-bridge doctor --json
{
  "activeTasks": 277,
  "archivedTasks": 17,
  "clean": true,
  "dangling": [],
  "duplicated": [],
  "orphaned": [],
  "referenced": 277,
  "unconfirmed": false,
  "unconfirmedReason": "",
  "unresolved": []
}
```

### Exit codes

| Code | Meaning |
|---|---|
| 0 | All checks passed. |
| 1 | A check failed (for example, SP is unreachable), or no verdict could be reached. **Takes precedence over 3**, so an unrelated failure later in the run (the MCP self-check, say) masks a confirmed anomaly in the summary line. |
| 2 | Bad usage (unknown flag or unexpected argument). |
| 3 | An anomaly survived both passes. `unconfirmed` may still be true if the passes disagreed about *other* ids — a race elsewhere does not make a twice-seen anomaly less real. |

Because 1 outranks 3, a monitoring script should key on `doctor --json`, which
runs only the integrity check and cannot be masked by an unrelated failure. The
`--deep` exit code is for interactive use, where the printed report is visible
alongside it.

`--json` follows the same table, so a script can distinguish a corrupt store
(3) from a connection failure (1) without parsing output. On bad usage it
writes the error and help to stderr, leaving stdout empty rather than
contaminating the JSON stream.

The index fields are checked the same way. `project.taskIds`,
`project.backlogTaskIds`, `tag.taskIds` and `task.subTaskIds` must each be
present and hold a list of ids; a missing or malformed one is an error naming
the field, not an empty index. Reading a missing index as "this project
references nothing" would turn every task it owns into an orphan — a healthy
store reported as corrupt — and because that result is deterministic it would
survive both confirmation passes and be reported as confirmed. An empty list is
legal: a project with no tasks is normal.

An unreadable index **degrades the run rather than failing it**. The
reference-derived verdicts (`dangling`, `orphaned`) are withheld, because a
reference set built from a broken index cannot be trusted — every task in that
project would look orphaned. Everything derived from the task pools alone stays
valid and is still reported: the pool counts, and `duplicated`, which needs no
index at all. That matters, because in a real corruption event those are the
signals worth keeping.

The report comes back `unconfirmed` with the reason naming the entity and field,
and exits 1:

```console
Store integrity... UNCONFIRMED
  active tasks        : 2
  archived tasks      : 0
  referenced by index : 0
  reason              : /projects: p_broken is missing "taskIds"; cannot judge integrity

  No verdict: see the reason above. Re-run once it is resolved.
```

`referenced by index` reads 0 on this path rather than a partial count: the
loops keep going past the first bad entity, so any figure would cover only the
entities that happened to parse.

**Monitoring should treat any non-zero exit as needing attention** rather than
keying on 3 alone, since this class of problem reports as 1.

If any of the four pulls returns something that is not a list — `data: null`, an
empty body, a non-JSON payload — the check reports an error naming that endpoint
and exits 1 rather than reading it as "zero entities". Treating a degenerate
response as an empty collection would make a healthy store look corrupt, and the
warning it prints tells the user not to restore a backup.

3 is distinct on purpose: it separates "cannot reach SP" from "SP answered, and
its data is broken", which need different responses.

### Anomalies are confirmed before being reported

The four requests are not an atomic snapshot of a live app. Adding a task in the
UI between the task pull and the project pull would leave the new id in
`project.taskIds` but not in the task set, which looks exactly like a dangling
reference; deleting a project in that window makes its tasks look orphaned.

So when the first pass finds anomalies, the check runs again and sorts what it
saw into two groups:

- **`dangling` / `orphaned` / `duplicated`** — seen in **both** passes. These are
  confirmed and reported as an inconsistency (exit 3). A race elsewhere in the
  store does not make them less real.
- **`unresolved`** — seen by only one pass. Most often the store being edited
  mid-check, but it could also be corruption appearing or clearing, so these are
  recorded rather than discarded.

`unconfirmed` is true when `unresolved` is non-empty, or when the confirmation
pass could not run at all. `unconfirmedReason` carries the second pass's error
in that case, so "SP became unreachable" is not misreported as concurrent
editing. If the confirmation pass fails, nothing was seen
twice, so every anomaly moves to `unresolved` and none are reported as
confirmed.

`clean` is true only when neither pass saw anything. A report that is
unconfirmed with no confirmed anomalies reaches no verdict and exits 1 — re-run
with the app idle.

### If it warns

Restart Super Productivity and re-run — an inconsistency confined to the
renderer's in-memory store clears on reload, since the persisted database is
usually intact.

**Do not import a backup while this warning is showing.** Backups are written
from the same in-memory state, so a backup taken during an inconsistency
captures it, and restoring one can turn a recoverable glitch into real data
loss.

## Archiving and existence

`archive_task` / `tasks archive` checks that the task is in the active list
before archiving it, and returns `TASK_NOT_FOUND` when it is not:

```console
$ sp-local-bridge tasks archive GHOST_ID
Error [TASK_NOT_FOUND]: Task is not in the active list, so it was not archived now. It may already be archived, or never have existed.
$ echo $?
1
```

This is a bridge-side guard. Super Productivity's own endpoint answers
`{"ok":true,"data":{"id":"...","archived":true}}` for ids that never existed, unlike `get`,
`update`, `start` and `restore`, which all return `TASK_NOT_FOUND`.

::: danger Do not reproduce this against a real store
Posting to the archive route with an id that does not exist is what crashed
Super Productivity's renderer and left its in-memory store inconsistent — see
the note at the end of this section. There is deliberately no copy-pasteable
command here.

To confirm the upstream behaviour, use a throwaway profile:

```bash
superproductivity --user-data-dir=/tmp/sp-scratch
```

The API port is a hardcoded 3876 and Super Productivity takes a single-instance
lock, so the real app must be closed first.
:::

The guard matters most for automation, where a mistaken or invented id would
otherwise produce a confident success and be reported as a completed archive —
the one operation that gave no signal to correct.

Note that an **already-archived** task also reports `TASK_NOT_FOUND`, since it
is no longer in the active list. `GET /tasks/:id` resolves the active pool only
— verified against SP 18.10.0, where a task confirmed present in the archive
returns 404 from that route. Use `--source archived --include-done` to check
whether it is already there before treating this as a failure.

The guard narrows the problem but does not eliminate it. If a task is archived
or deleted by someone else between the check and the archive call, the call
still goes out against an id no longer in the active pool. That case is not
harmless — it is the one that crashed Super Productivity's renderer and left its
in-memory store inconsistent — so closing the window needs a fix upstream. See
the reports in the project's issue tracker.

A transport failure is never recast as a missing task: if Super Productivity is
unreachable, the error is `SP_UNAVAILABLE`.
