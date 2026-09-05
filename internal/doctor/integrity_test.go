package doctor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/bridge"
)

// storeFixture describes a fake SP store for the integrity checker.
type storeFixture struct {
	active   []map[string]any
	archived []map[string]any
	projects []map[string]any
	tags     []map[string]any
}

func task(id string, subTaskIDs ...string) map[string]any {
	t := map[string]any{"id": id, "title": id}
	if len(subTaskIDs) > 0 {
		ids := make([]any, len(subTaskIDs))
		for i, s := range subTaskIDs {
			ids[i] = s
		}
		t["subTaskIds"] = ids
	}
	return t
}

func withIDs(key string, ids ...string) map[string]any {
	arr := make([]any, len(ids))
	for i, s := range ids {
		arr[i] = s
	}
	return map[string]any{"id": "x", "title": "x", key: arr}
}

func newStoreServer(t *testing.T, f storeFixture) *httptest.Server {
	t.Helper()
	write := func(w http.ResponseWriter, v any) {
		if v == nil {
			v = []map[string]any{}
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": v})
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/tasks" && r.URL.Query().Get("source") == "archived":
			write(w, f.archived)
		case r.URL.Path == "/tasks":
			write(w, f.active)
		case r.URL.Path == "/projects":
			write(w, f.projects)
		case r.URL.Path == "/tags":
			write(w, f.tags)
		default:
			http.NotFound(w, r)
		}
	}))
}

func check(t *testing.T, f storeFixture) IntegrityReport {
	t.Helper()
	srv := newStoreServer(t, f)
	defer srv.Close()
	report, err := CheckIntegrity(context.Background(), bridge.NewClient(srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return report
}

func TestIntegrity_CleanStore(t *testing.T) {
	r := check(t, storeFixture{
		active:   []map[string]any{task("t1"), task("t2")},
		projects: []map[string]any{withIDs("taskIds", "t1", "t2")},
	})
	if !r.Clean() {
		t.Fatalf("expected clean, got dangling=%v orphaned=%v", r.Dangling, r.Orphaned)
	}
	if r.ActiveTasks != 2 || r.Referenced != 2 {
		t.Fatalf("counts wrong: %+v", r)
	}
}

// Archiving removes a task from project.taskIds, so archived tasks are expected
// to be unreferenced. Counting them as orphans made the check warn on a
// perfectly healthy store.
func TestIntegrity_ArchivedTasksAreNotOrphans(t *testing.T) {
	r := check(t, storeFixture{
		active:   []map[string]any{task("t1")},
		archived: []map[string]any{task("a1"), task("a2")},
		projects: []map[string]any{withIDs("taskIds", "t1")},
	})
	if !r.Clean() {
		t.Fatalf("archived tasks must not be orphans: dangling=%v orphaned=%v", r.Dangling, r.Orphaned)
	}
	if r.ActiveTasks != 1 || r.ArchivedTasks != 2 {
		t.Fatalf("pool counts wrong: %+v", r)
	}
}

// A project may legitimately reference a task that now lives in the archive.
func TestIntegrity_ReferenceIntoArchiveIsNotDangling(t *testing.T) {
	r := check(t, storeFixture{
		active:   []map[string]any{task("t1")},
		archived: []map[string]any{task("a1")},
		projects: []map[string]any{withIDs("taskIds", "t1", "a1")},
	})
	if len(r.Dangling) != 0 {
		t.Fatalf("expected no dangling, got %v", r.Dangling)
	}
}

// The failure mode this check exists for: entities dropped, indexes intact.
func TestIntegrity_DetectsDanglingReferences(t *testing.T) {
	r := check(t, storeFixture{
		active:   []map[string]any{task("t1")},
		projects: []map[string]any{withIDs("taskIds", "t1", "gone1", "gone2")},
	})
	if len(r.Dangling) != 2 {
		t.Fatalf("expected 2 dangling, got %v", r.Dangling)
	}
	if r.Clean() {
		t.Fatal("store with dangling refs must not be clean")
	}
	if r.Dangling[0] != "gone1" || r.Dangling[1] != "gone2" {
		t.Fatalf("dangling should be sorted: %v", r.Dangling)
	}
}

func TestIntegrity_DetectsOrphanedActiveTask(t *testing.T) {
	r := check(t, storeFixture{
		active:   []map[string]any{task("t1"), task("stray")},
		projects: []map[string]any{withIDs("taskIds", "t1")},
	})
	if len(r.Orphaned) != 1 || r.Orphaned[0] != "stray" {
		t.Fatalf("expected orphan 'stray', got %v", r.Orphaned)
	}
}

func TestIntegrity_SubtasksAndTagsCountAsReferences(t *testing.T) {
	r := check(t, storeFixture{
		active:   []map[string]any{task("parent", "sub1"), task("sub1"), task("tagged")},
		projects: []map[string]any{withIDs("taskIds", "parent")},
		tags:     []map[string]any{withIDs("taskIds", "tagged")},
	})
	if !r.Clean() {
		t.Fatalf("expected clean, got dangling=%v orphaned=%v", r.Dangling, r.Orphaned)
	}
}

func TestIntegrity_PropagatesFetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]any{
			"ok": false, "error": map[string]any{"code": "SP_ERROR", "message": "boom"},
		})
	}))
	defer srv.Close()
	if _, err := CheckIntegrity(context.Background(), bridge.NewClient(srv.URL)); err == nil {
		t.Fatal("expected an error when the store cannot be read")
	}
}

func TestIntegrityJSON_EmptyListsNotNull(t *testing.T) {
	out := integrityJSON(IntegrityReport{ActiveTasks: 1, Referenced: 1})
	if strings.Contains(out, "null") {
		t.Fatalf("empty id lists must render as [], got:\n%s", out)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["clean"] != true {
		t.Fatalf("expected clean=true, got %v", parsed["clean"])
	}
}

func TestSample_TruncatesLongLists(t *testing.T) {
	got := sample([]string{"a", "b", "c", "d", "e"}, 3)
	if !strings.Contains(got, "+2 more") {
		t.Fatalf("expected truncation marker, got %q", got)
	}
	if full := sample([]string{"a"}, 3); strings.Contains(full, "more") {
		t.Fatalf("short list must not be truncated, got %q", full)
	}
}
