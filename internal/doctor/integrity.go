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
}

// Clean reports whether the store is self-consistent.
func (r IntegrityReport) Clean() bool {
	return len(r.Dangling) == 0 && len(r.Orphaned) == 0
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

// asObjects coerces a decoded JSON array into a slice of objects, skipping
// anything that is not one.
func asObjects(data any) []map[string]any {
	arr, ok := data.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if obj, ok := item.(map[string]any); ok {
			out = append(out, obj)
		}
	}
	return out
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

	activeRes := client.ListTasks(ctx, map[string]string{"source": "active", "includeDone": "true"})
	if !activeRes.OK {
		return report, fmt.Errorf("%s", activeRes.Error.Message)
	}
	archivedRes := client.ListTasks(ctx, map[string]string{"source": "archived", "includeDone": "true"})
	if !archivedRes.OK {
		return report, fmt.Errorf("%s", archivedRes.Error.Message)
	}
	projects := client.ListProjects(ctx, nil)
	if !projects.OK {
		return report, fmt.Errorf("%s", projects.Error.Message)
	}
	tags := client.ListTags(ctx, nil)
	if !tags.OK {
		return report, fmt.Errorf("%s", tags.Error.Message)
	}

	activeTasks := asObjects(activeRes.Data)
	archivedTasks := asObjects(archivedRes.Data)

	active := make(map[string]struct{}, len(activeTasks))
	for _, t := range activeTasks {
		if id, ok := idField(t, "id"); ok {
			active[id] = struct{}{}
		}
	}
	known := make(map[string]struct{}, len(active)+len(archivedTasks))
	for id := range active {
		known[id] = struct{}{}
	}
	for _, t := range archivedTasks {
		if id, ok := idField(t, "id"); ok {
			known[id] = struct{}{}
		}
	}
	report.ActiveTasks = len(active)
	report.ArchivedTasks = len(known) - len(active)

	referenced := make(map[string]struct{})
	for _, p := range asObjects(projects.Data) {
		collectIDs(referenced, p, "taskIds", "backlogTaskIds")
	}
	for _, tag := range asObjects(tags.Data) {
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
	return report, nil
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
	fmt.Println()
	fmt.Println("  The store is inconsistent. Restart Super Productivity and re-run.")
	fmt.Println("  Do NOT import a backup taken while this warning is showing — backups are")
	fmt.Println("  written from the same in-memory state and will capture the inconsistency.")
	return false
}

// integrityJSON renders the report as JSON for scripting.
func integrityJSON(report IntegrityReport) string {
	payload := map[string]any{
		"activeTasks":   report.ActiveTasks,
		"archivedTasks": report.ArchivedTasks,
		"referenced":    report.Referenced,
		"dangling":      report.Dangling,
		"orphaned":      report.Orphaned,
		"clean":         report.Clean(),
	}
	if report.Dangling == nil {
		payload["dangling"] = []string{}
	}
	if report.Orphaned == nil {
		payload["orphaned"] = []string{}
	}
	out, _ := json.MarshalIndent(payload, "", "  ")
	return string(out)
}
