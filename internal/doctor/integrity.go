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

	// Transient counts anomalies from the first pass that did not survive
	// confirmation — the signature of the store changing mid-check.
	Transient int
	// Unconfirmed marks a report whose confirmation pass could not be run.
	Unconfirmed bool
}

// Clean reports whether the store is self-consistent.
func (r IntegrityReport) Clean() bool {
	return len(r.Dangling) == 0 && len(r.Orphaned) == 0 && len(r.Duplicated) == 0
}

// idField pulls a string id out of a decoded JSON object.
func idField(m map[string]any, key string) (string, bool) {
	v, ok := m[key].(string)
	return v, ok && v != ""
}

// collectIDs appends every string in the named array field to dst.
func collectIDs(dst map[string]struct{}, m map[string]any, keys ...string) {
	for _, key := range keys {
		arr, ok := m[key].([]any)
		if !ok {
			continue
		}
		for _, item := range arr {
			if s, ok := item.(string); ok && s != "" {
				dst[s] = struct{}{}
			}
		}
	}
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
	for _, p := range projectList {
		collectIDs(referenced, p, "taskIds", "backlogTaskIds")
	}
	for _, tag := range tagList {
		collectIDs(referenced, tag, "taskIds")
	}
	// Subtasks are referenced by their parent in either pool.
	for _, t := range activeTasks {
		collectIDs(referenced, t, "subTaskIds")
	}
	for _, t := range archivedTasks {
		collectIDs(referenced, t, "subTaskIds")
	}
	report.Referenced = len(referenced)

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
// it and keeps only those that appear in both passes.
//
// The four pulls are not an atomic snapshot of a live app. A task added in the
// UI between the task pull and the project pull lands in project.taskIds but
// not in the task set, and would be reported as dangling; deleting a project in
// that same window leaves its tasks unreferenced and looking orphaned. Both are
// normal use, and both would otherwise print "the store is inconsistent" along
// with advice not to restore a backup.
//
// A genuine inconsistency persists across passes; a race does not.
func CheckIntegrityConfirmed(ctx context.Context, client *bridge.Client) (IntegrityReport, error) {
	first, err := CheckIntegrity(ctx, client)
	if err != nil || first.Clean() {
		return first, err
	}

	second, err := CheckIntegrity(ctx, client)
	if err != nil {
		// The confirmation pull failed; report the first pass rather than
		// claiming a verdict we could not confirm.
		first.Unconfirmed = true
		return first, nil
	}

	confirmed := second
	confirmed.Dangling = intersect(first.Dangling, second.Dangling)
	confirmed.Orphaned = intersect(first.Orphaned, second.Orphaned)
	confirmed.Duplicated = intersect(first.Duplicated, second.Duplicated)
	confirmed.Transient = (len(first.Dangling) - len(confirmed.Dangling)) +
		(len(first.Orphaned) - len(confirmed.Orphaned)) +
		(len(first.Duplicated) - len(confirmed.Duplicated))
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
	if report.Clean() {
		fmt.Println("OK")
		if report.Transient > 0 {
			fmt.Printf("  (store changed during the check; %d transient anomal(ies) ignored)\n", report.Transient)
		}
		fmt.Printf("  active tasks        : %d\n", report.ActiveTasks)
		fmt.Printf("  archived tasks      : %d\n", report.ArchivedTasks)
		fmt.Printf("  referenced by index : %d\n", report.Referenced)
		return true
	}

	fmt.Println("WARN")
	fmt.Printf("  active tasks        : %d\n", report.ActiveTasks)
	fmt.Printf("  archived tasks      : %d\n", report.ArchivedTasks)
	fmt.Printf("  referenced by index : %d\n", report.Referenced)
	if len(report.Dangling) > 0 {
		fmt.Printf("  dangling references : %d  %s\n", len(report.Dangling), sample(report.Dangling, 3))
		fmt.Println("    Projects or tags point at tasks that no longer exist.")
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
	if report.Transient > 0 {
		fmt.Printf("  (%d anomal(ies) from the first pass did not recur and were ignored)\n", report.Transient)
	}
	if report.Unconfirmed {
		fmt.Println("  NOTE: the confirmation pass could not run, so these are unconfirmed.")
	}
	fmt.Println()
	fmt.Println("  The store is inconsistent. Restart Super Productivity and re-run.")
	fmt.Println("  Do NOT import a backup taken while this warning is showing — backups are")
	fmt.Println("  written from the same in-memory state and will capture the inconsistency.")
	return false
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
		"activeTasks":   report.ActiveTasks,
		"archivedTasks": report.ArchivedTasks,
		"referenced":    report.Referenced,
		"dangling":      ids(report.Dangling),
		"orphaned":      ids(report.Orphaned),
		"duplicated":    ids(report.Duplicated),
		"transient":     report.Transient,
		"unconfirmed":   report.Unconfirmed,
		"clean":         report.Clean(),
	}
	out, _ := json.MarshalIndent(payload, "", "  ")
	return string(out)
}
