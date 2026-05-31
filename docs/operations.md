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
`source=all` expands the pool to include archived tasks but does **not** automatically show completed tasks. Most archived tasks are done, so combine with `includeDone=true` to see them. The `taskCount` in `get_status` reflects the active pool including done tasks (not all tasks across all sources).
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
| `PROJECT_NOT_FOUND` | Project ID not found |
| `SP_ERROR` | SP returned an error |
| `INTERNAL_ERROR` | Unexpected bridge error |

## Intentionally Excluded

- **`task.delete`** — Destructive operation excluded by design. Use archive/restore instead.
