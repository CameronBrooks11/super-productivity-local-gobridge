package bridge

// Response shaping: what a list operation hands back.
//
// SP returns whole entities, and the whole entity is rarely what a caller
// wants. A single list_tasks with includeDone on a real store is ~94KB — around
// 24k tokens — and most of it is fields nothing consumes: a per-day time map
// that grows without bound, theme colours, worklog export column lists. Asking
// "what am I working on?" spent most of the answer before the reader reached
// it.
//
// SP has no server-side projection or paging — limit, offset, page and fields
// are all ignored by the API — so this happens here.

// compactTaskFields is what a caller almost always wants from a task.
//
// Deliberately excluded: timeSpentOnDay (a map with an entry per day worked,
// unbounded over a task's life), attachments, created/modified, and the four
// issue-integration fields, which are set on a small minority of tasks and
// meaningless without the provider.
//
// doneOn and dueDay stay: "what did I finish today" and "what is due" are
// ordinary questions, and both are single scalars.
// plannedAt and dueWithTime are included because add_task and update_task can
// set them: a caller that cannot read back what it just wrote cannot confirm
// the write, or find the tasks it needs to clear. Neither appears on the store
// this was measured against, so they cost nothing there; project() only emits
// fields the response actually carried.
var compactTaskFields = []string{
	"id", "title", "isDone", "projectId", "tagIds", "parentId",
	"subTaskIds", "timeSpent", "timeEstimate", "notes",
	"dueDay", "dueWithTime", "plannedAt", "doneOn",
}

// compactProjectFields keeps what identifies a project. taskIds and
// backlogTaskIds are excluded because they are the bulk of the payload and a
// caller listing projects is choosing between them, not enumerating their
// contents — list_tasks with projectId does that, and does it filtered.
var compactProjectFields = []string{"id", "title", "isArchived"}

// compactTagFields: a tag is an id and a name. taskIds is excluded for the same
// reason as on a project — it is the bulk of the payload, and the TODAY tag
// carries an entry per planned task. A caller listing tags is choosing between
// them; list_tasks with tagId enumerates one, filtered.
var compactTagFields = []string{"id", "title"}

// listOptions are applied by the bridge after SP responds. They are never sent
// upstream, which is why they are stripped from the payload during validation.
type listOptions struct {
	limit  int // 0 means no limit
	offset int
	full   bool // return entities unprojected
}

// project narrows an entity to the named fields, keeping only those present.
// Absent fields stay absent rather than becoming null: SP distinguishes the two
// and so does the rest of the bridge.
func project(entity map[string]any, fields []string) map[string]any {
	out := make(map[string]any, len(fields))
	for _, f := range fields {
		if v, ok := entity[f]; ok {
			out[f] = v
		}
	}
	return out
}

// shapeList applies projection, then offset, then limit, to a list response.
//
// A non-list payload is returned untouched. That is not laziness: an error or a
// degenerate response should reach the caller as it is, not be reshaped into
// something that looks like a successful empty list.
func shapeList(result Result, fields []string, opts listOptions) Result {
	if !result.OK {
		return result
	}
	items, ok := result.Data.([]any)
	if !ok {
		return result
	}

	// Slice before projecting: building 284 maps to keep 20 undercuts the point
	// of asking for 20.
	matched := len(items)
	selected := items
	if opts.offset > 0 {
		if opts.offset >= len(selected) {
			selected = nil
		} else {
			selected = selected[opts.offset:]
		}
	}
	truncated := false
	if opts.limit > 0 && len(selected) > opts.limit {
		selected = selected[:opts.limit]
		truncated = true
	}

	shaped := make([]any, 0, len(selected))
	for _, item := range selected {
		obj, ok := item.(map[string]any)
		if !ok {
			// Not an entity; pass it through rather than dropping it, so a
			// surprise in the response is visible instead of silently gone.
			shaped = append(shaped, item)
			continue
		}
		if opts.full {
			shaped = append(shaped, obj)
			continue
		}
		shaped = append(shaped, project(obj, fields))
	}

	if shaped == nil {
		shaped = []any{}
	}
	out := Success(shaped)
	// Say when the list was cut. Without this a caller asked "how many tasks
	// are in project X?", passed the limit the tool description recommends, got
	// exactly that many back, and could not tell a complete answer from a
	// truncated one. get_status.taskCount is not a substitute: it counts the
	// whole active pool and cannot answer a filtered question.
	if truncated {
		out.Meta = map[string]any{
			"truncated": true,
			"returned":  len(shaped),
			"matched":   matched,
		}
	}
	return out
}
