package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/bridge"
)

// IntegrityReport summarises whether the store's indexes and task entities agree.
//
// Super Productivity keeps task entities in one collection and references them
// from project.taskIds, project.backlogTaskIds, tag.taskIds and task.subTaskIds.
// Those two views can disagree — a crashed effect in the renderer can drop
// entities while leaving every index pointing at them — and every liveness check
// still passes in that state, because each individual request answers fine.
type IntegrityReport struct {
	ActiveTasks   int
	ArchivedTasks int
	Referenced    int
	Dangling      []string // referenced by an index, no task entity exists
	Orphaned      []string // active task that no index references
	Duplicated    []string // present in both the active and archived pools

	// Unresolved holds ids flagged by exactly one pass. They are neither
	// confirmed nor dismissed: a race produces them, but so does corruption
	// appearing or clearing mid-check.
	Unresolved []string
	// Unconfirmed marks a report the two passes could not reconcile — either
	// Unresolved is non-empty, or the confirmation pass could not run.
	Unconfirmed bool
	// UnconfirmedReason carries the confirmation pass's error when it failed.
	// Without it the output blames concurrent editing for what was actually SP
	// becoming unreachable — the worst moment to misdiagnose, since a store
	// that just went down is exactly when anomalies matter.
	UnconfirmedReason string
}

// Clean reports whether the store is self-consistent.
func (r IntegrityReport) Clean() bool {
	return !r.HasConfirmedAnomalies() && len(r.Unresolved) == 0 && !r.Unconfirmed
}

// Confirmed reports whether the two passes agreed on everything.
func (r IntegrityReport) Confirmed() bool {
	return !r.Unconfirmed
}

// HasConfirmedAnomalies reports whether an anomaly survived both passes. These
// stand on their own: a race elsewhere in the store does not make them less
// real, so they are reported even when part of the run was inconclusive.
func (r IntegrityReport) HasConfirmedAnomalies() bool {
	return len(r.Dangling) > 0 || len(r.Orphaned) > 0 || len(r.Duplicated) > 0
}

// idField pulls a string id out of a decoded JSON object.
func idField(m map[string]any, key string) (string, bool) {
	v, ok := m[key].(string)
	return v, ok && v != ""
}

// collectIDs appends every string in the named array fields to dst.
//
// A missing or malformed index field is an error, not an empty list. Reading it
// as "this project references nothing" turns every task it owns into an orphan
// — a healthy store reported as corrupt, complete with advice not to restore a
// backup. That is the same failure objectsOrError guards on the entity side,
// and it is worse here: the result is deterministic, so it survives both
// confirmation passes and is reported as a *confirmed* anomaly.
//
// Audited against SP 18.10.0: all 12 projects, 3 tags, 284 active and 17
// archived tasks carried every field as a list. That is one store on one
// version, not a guarantee, so a violation degrades the check rather than
// failing it — see CheckIntegrity. An empty list stays legal: a project with no
// tasks is normal.
func collectIDs(dst map[string]struct{}, m map[string]any, endpoint string, keys ...string) error {
	// objectsOrError has already guaranteed an id, so name it: in a real
	// corruption event it is the most actionable thing we can hand the user,
	// who would otherwise be searching hundreds of entities by hand.
	owner, _ := idField(m, "id")
	for _, key := range keys {
		raw, present := m[key]
		if !present {
			return fmt.Errorf("%s: %s is missing %q; cannot judge integrity", endpoint, owner, key)
		}
		arr, ok := raw.([]any)
		if !ok {
			return fmt.Errorf("%s: %s has %q as %T, not a list; cannot judge integrity", endpoint, owner, key, raw)
		}
		for _, item := range arr {
			id, ok := item.(string)
			if !ok || id == "" {
				return fmt.Errorf("%s: %s has an entry in %q that is not an id; cannot judge integrity", endpoint, owner, key)
			}
			dst[id] = struct{}{}
		}
	}
	return nil
}

// objectsOrError coerces a decoded JSON array into a slice of objects.
//
// A missing or non-array payload must not be read as "zero entities". The
// client reports Success(nil) for `{"ok":true,"data":null}`, for a 2xx with an
// empty body, and for a 2xx carrying non-JSON — and silently treating any of
// those as an empty collection makes a healthy store look corrupt. An empty
// array is different, and stays legal: a store really can have no tags.
func objectsOrError(data any, endpoint string) ([]map[string]any, error) {
	arr, ok := data.([]any)
	if !ok {
		return nil, fmt.Errorf("%s returned no usable list (got %T); cannot judge integrity", endpoint, data)
	}
	out := make([]map[string]any, 0, len(arr))
	skipped := 0
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			skipped++
			continue
		}
		if _, hasID := idField(obj, "id"); !hasID {
			skipped++
			continue
		}
		out = append(out, obj)
	}
	if skipped > 0 {
		// Dropping these silently reaches the same wrong answer this function
		// exists to prevent: an entity we failed to parse is absent from the
		// known set, so every reference to it is reported as dangling.
		return nil, fmt.Errorf("%s returned %d entr(ies) without a usable id; cannot judge integrity", endpoint, skipped)
	}
	return out, nil
}

// fetchList runs one pull and names the endpoint in any error, so a failure
// says which of the four requests broke rather than just how.
func fetchList(res bridge.Result, endpoint string) ([]map[string]any, error) {
	if !res.OK {
		code := ""
		if res.Error != nil && res.Error.Code != "" {
			code = " [" + res.Error.Code + "]"
		}
		msg := ""
		if res.Error != nil {
			msg = res.Error.Message
		}
		return nil, fmt.Errorf("%s%s: %s", endpoint, code, msg)
	}
	return objectsOrError(res.Data, endpoint)
}

// CheckIntegrity fetches the whole store and cross-references entities against
// indexes.
//
// The two pools are handled differently on purpose. Archiving a task removes it
// from project.taskIds, so archived tasks are *expected* to be unreferenced and
// are not orphans. They must still be loaded, because a project or tag can
// legitimately reference a task that now lives in the archive, and counting that
// as dangling would be just as wrong.
func CheckIntegrity(ctx context.Context, client *bridge.Client) (IntegrityReport, error) {
	var report IntegrityReport

	activeTasks, err := fetchList(
		client.ListTasks(ctx, map[string]string{"source": "active", "includeDone": "true"}), "/tasks?source=active")
	if err != nil {
		return report, err
	}
	archivedTasks, err := fetchList(
		client.ListTasks(ctx, map[string]string{"source": "archived", "includeDone": "true"}), "/tasks?source=archived")
	if err != nil {
		return report, err
	}
	projectList, err := fetchList(client.ListProjects(ctx, nil), "/projects")
	if err != nil {
		return report, err
	}
	tagList, err := fetchList(client.ListTags(ctx, nil), "/tags")
	if err != nil {
		return report, err
	}

	active := make(map[string]struct{}, len(activeTasks))
	for _, t := range activeTasks {
		if id, ok := idField(t, "id"); ok {
			active[id] = struct{}{}
		}
	}
	// Count the archived pool directly rather than subtracting. A task present
	// in both pools is itself a partially-applied archive — exactly the kind of
	// inconsistency this check exists for — and subtraction would hide it.
	archived := make(map[string]struct{}, len(archivedTasks))
	for _, t := range archivedTasks {
		if id, ok := idField(t, "id"); ok {
			archived[id] = struct{}{}
		}
	}
	known := make(map[string]struct{}, len(active)+len(archived))
	for id := range active {
		known[id] = struct{}{}
	}
	for id := range archived {
		if _, both := active[id]; both {
			report.Duplicated = append(report.Duplicated, id)
		}
		known[id] = struct{}{}
	}
	report.ActiveTasks = len(active)
	report.ArchivedTasks = len(archived)

	referenced := make(map[string]struct{})
	var indexErr error
	note := func(err error) {
		if err != nil && indexErr == nil {
			indexErr = err
		}
	}
	for _, p := range projectList {
		note(collectIDs(referenced, p, "/projects", "taskIds", "backlogTaskIds"))
	}
	for _, tag := range tagList {
		note(collectIDs(referenced, tag, "/tags", "taskIds"))
	}
	// Subtasks are referenced by their parent in either pool.
	for _, t := range activeTasks {
		note(collectIDs(referenced, t, "/tasks?source=active", "subTaskIds"))
	}
	for _, t := range archivedTasks {
		note(collectIDs(referenced, t, "/tasks?source=archived", "subTaskIds"))
	}
	report.Referenced = len(referenced)

	// An unreadable index makes the reference set untrustworthy, so the
	// reference-derived verdicts are withheld. Everything derived from the task
	// pools alone stays valid, though — Duplicated needs no index — and
	// discarding it would throw away the most actionable signal in exactly the
	// corruption event this check exists for. So the run degrades to unconfirmed
	// rather than failing outright.
	if indexErr != nil {
		report.Unconfirmed = true
		report.UnconfirmedReason = indexErr.Error()
		return report, nil
	}

	for id := range referenced {
		if _, ok := known[id]; !ok {
			report.Dangling = append(report.Dangling, id)
		}
	}
	for id := range active {
		if _, ok := referenced[id]; !ok {
			report.Orphaned = append(report.Orphaned, id)
		}
	}
	sort.Strings(report.Dangling)
	sort.Strings(report.Orphaned)
	sort.Strings(report.Duplicated)
	return report, nil
}

// intersect returns the ids present in both slices, preserving sorted order.
func intersect(a, b []string) []string {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	in := make(map[string]struct{}, len(b))
	for _, id := range b {
		in[id] = struct{}{}
	}
	var out []string
	for _, id := range a {
		if _, ok := in[id]; ok {
			out = append(out, id)
		}
	}
	return out
}

// CheckIntegrityConfirmed runs the check and, when it finds anomalies, repeats
// it and reports only those seen in both passes.
//
// The four pulls are not an atomic snapshot of a live app. A task added in the
// UI between the task pull and the project pull lands in project.taskIds but
// not in the task set, and would be reported as dangling; deleting a project in
// that same window leaves its tasks unreferenced and looking orphaned. Both are
// normal use, and both would otherwise print "the store is inconsistent" along
// with advice not to restore a backup.
//
// A genuine inconsistency persists across passes; a race does not. Anything the
// two passes disagree about is reported as unconfirmed rather than as either a
// clean store or a confirmed inconsistency — reporting clean would hide real
// corruption that the second pass saw.
func CheckIntegrityConfirmed(ctx context.Context, client *bridge.Client) (IntegrityReport, error) {
	first, err := CheckIntegrity(ctx, client)
	if err != nil || first.Clean() {
		return first, err
	}

	second, err := CheckIntegrity(ctx, client)
	if err != nil {
		// The confirmation pull failed, so every anomaly was seen exactly once
		// and may just be a race. Move them out of the confirmed sets: nothing
		// here survived two passes, and leaving them in would let the caller
		// treat a single observation as a verdict.
		unresolved := IntegrityReport{
			ActiveTasks:       first.ActiveTasks,
			ArchivedTasks:     first.ArchivedTasks,
			Referenced:        first.Referenced,
			Unconfirmed:       true,
			UnconfirmedReason: err.Error(),
		}
		// Orphaned and Duplicated are not mutually exclusive — a task in both
		// pools that nothing references lands in both — so concatenating raw
		// would report one id twice and inflate the count.
		seen := make(map[string]struct{})
		for _, group := range [][]string{first.Dangling, first.Orphaned, first.Duplicated} {
			for _, id := range group {
				if _, dup := seen[id]; dup {
					continue
				}
				seen[id] = struct{}{}
				unresolved.Unresolved = append(unresolved.Unresolved, id)
			}
		}
		sort.Strings(unresolved.Unresolved)
		return unresolved, nil
	}

	confirmed := second
	confirmed.Dangling = intersect(first.Dangling, second.Dangling)
	confirmed.Orphaned = intersect(first.Orphaned, second.Orphaned)
	confirmed.Duplicated = intersect(first.Duplicated, second.Duplicated)

	// Everything either pass flagged that the other did not was seen once. It is
	// recorded rather than dropped — dropping it would let a store the second
	// pass found broken be reported clean — but it does not gate the anomalies
	// that did survive both passes.
	agreed := make(map[string]struct{})
	for _, group := range [][]string{confirmed.Dangling, confirmed.Orphaned, confirmed.Duplicated} {
		for _, id := range group {
			agreed[id] = struct{}{}
		}
	}
	seenOnce := make(map[string]struct{})
	for _, pass := range []IntegrityReport{first, second} {
		for _, group := range [][]string{pass.Dangling, pass.Orphaned, pass.Duplicated} {
			for _, id := range group {
				if _, ok := agreed[id]; !ok {
					seenOnce[id] = struct{}{}
				}
			}
		}
	}
	for id := range seenOnce {
		confirmed.Unresolved = append(confirmed.Unresolved, id)
	}
	sort.Strings(confirmed.Unresolved)
	// Preserve a reason carried up from the pass itself (an unreadable index),
	// which is independent of whether the two passes disagreed.
	confirmed.Unconfirmed = len(confirmed.Unresolved) > 0 || confirmed.UnconfirmedReason != ""
	return confirmed, nil
}

// sample renders at most n ids for display, so a store with hundreds of broken
// references does not flood the terminal.
func sample(ids []string, n int) string {
	if len(ids) <= n {
		return fmt.Sprintf("%v", ids)
	}
	return fmt.Sprintf("%v (+%d more)", ids[:n], len(ids)-n)
}

// printIntegrity renders the report. Returns true when the store is clean.
func printIntegrity(report IntegrityReport) bool {
	switch {
	case report.HasConfirmedAnomalies():
		fmt.Println("WARN")
	case report.Unconfirmed:
		fmt.Println("UNCONFIRMED")
	default:
		fmt.Println("OK")
	}

	fmt.Printf("  active tasks        : %d\n", report.ActiveTasks)
	fmt.Printf("  archived tasks      : %d\n", report.ArchivedTasks)
	fmt.Printf("  referenced by index : %d\n", report.Referenced)

	if len(report.Dangling) > 0 {
		fmt.Printf("  dangling references : %d  %s\n", len(report.Dangling), sample(report.Dangling, 3))
		fmt.Println("    Projects or tags point at tasks that do not exist.")
	}
	if len(report.Orphaned) > 0 {
		fmt.Printf("  orphaned tasks      : %d  %s\n", len(report.Orphaned), sample(report.Orphaned, 3))
		fmt.Println("    Active tasks that nothing references; they may be invisible in the UI.")
	}
	if len(report.Duplicated) > 0 {
		fmt.Printf("  in both pools       : %d  %s\n", len(report.Duplicated), sample(report.Duplicated, 3))
		fmt.Println("    Tasks listed as both active and archived; an archive or restore")
		fmt.Println("    was only partially applied.")
	}
	if len(report.Unresolved) > 0 {
		fmt.Printf("  seen in one pass    : %d  %s\n", len(report.Unresolved), sample(report.Unresolved, 3))
		if report.UnconfirmedReason != "" {
			fmt.Println("    Seen only once, because the run could not be completed. They are")
			fmt.Println("    not necessarily transient.")
		} else {
			fmt.Println("    Flagged by only one of the two passes, so most likely the store")
			fmt.Println("    being edited while the check ran.")
		}
	}
	if report.UnconfirmedReason != "" {
		fmt.Printf("  reason              : %s\n", report.UnconfirmedReason)
	}

	if report.HasConfirmedAnomalies() {
		fmt.Println()
		fmt.Println("  The store is inconsistent. Restart Super Productivity and re-run.")
		fmt.Println("  Do NOT import a backup taken while this warning is showing — backups are")
		fmt.Println("  written from the same in-memory state and will capture the inconsistency.")
		return false
	}
	if report.Unconfirmed {
		fmt.Println()
		if report.UnconfirmedReason != "" {
			// The reason is self-describing — an unreadable index names the
			// entity and field, a failed pull names the endpoint and code — so
			// it carries the diagnosis rather than a guess at the cause.
			fmt.Println("  No verdict: see the reason above. Re-run once it is resolved.")
		} else {
			fmt.Println("  No verdict: the two passes disagreed and nothing was seen twice.")
			fmt.Println("  Re-run with the app idle.")
		}
		return false
	}
	return true
}

// integrityJSON renders the report as JSON for scripting.
func integrityJSON(report IntegrityReport) string {
	// Empty id lists must render as [] rather than null, so consumers can index
	// them without a nil check.
	ids := func(v []string) []string {
		if v == nil {
			return []string{}
		}
		return v
	}
	payload := map[string]any{
		"activeTasks":       report.ActiveTasks,
		"archivedTasks":     report.ArchivedTasks,
		"referenced":        report.Referenced,
		"dangling":          ids(report.Dangling),
		"orphaned":          ids(report.Orphaned),
		"duplicated":        ids(report.Duplicated),
		"unresolved":        ids(report.Unresolved),
		"unconfirmed":       report.Unconfirmed,
		"unconfirmedReason": report.UnconfirmedReason,
		"clean":             report.Clean(),
	}
	out, _ := json.MarshalIndent(payload, "", "  ")
	return string(out)
}
