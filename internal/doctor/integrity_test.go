package doctor

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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

// anyIDs mirrors how a decoded JSON array of ids arrives.
func anyIDs(ids ...string) []any {
	arr := make([]any, len(ids))
	for i, s := range ids {
		arr[i] = s
	}
	return arr
}

// The fixtures below mirror what SP 18.10.0 actually returns: every index field
// is present as a list on every object, empty rather than omitted. Fixtures that
// omitted them were not just unrealistic, they hid the guard this file tests.

func task(id string, subTaskIDs ...string) map[string]any {
	return map[string]any{"id": id, "title": id, "subTaskIds": anyIDs(subTaskIDs...)}
}

func project(taskIDs ...string) map[string]any {
	return map[string]any{
		"id": "p1", "title": "p1",
		"taskIds":        anyIDs(taskIDs...),
		"backlogTaskIds": anyIDs(),
	}
}

func tag(taskIDs ...string) map[string]any {
	return map[string]any{"id": "g1", "title": "g1", "taskIds": anyIDs(taskIDs...)}
}

func newStoreServer(t *testing.T, f storeFixture) *httptest.Server {
	t.Helper()
	// A nil slice boxed into any is not == nil, so it would encode as JSON null
	// and the checker would (correctly) reject it as an unusable list. Fixtures
	// that omit a collection mean "empty", so normalise to an empty slice.
	write := func(w http.ResponseWriter, v []map[string]any) {
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
		projects: []map[string]any{project("t1", "t2")},
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
		projects: []map[string]any{project("t1")},
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
		projects: []map[string]any{project("t1", "a1")},
	})
	if len(r.Dangling) != 0 {
		t.Fatalf("expected no dangling, got %v", r.Dangling)
	}
}

// The failure mode this check exists for: entities dropped, indexes intact.
func TestIntegrity_DetectsDanglingReferences(t *testing.T) {
	r := check(t, storeFixture{
		active:   []map[string]any{task("t1")},
		projects: []map[string]any{project("t1", "gone1", "gone2")},
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
		projects: []map[string]any{project("t1")},
	})
	if len(r.Orphaned) != 1 || r.Orphaned[0] != "stray" {
		t.Fatalf("expected orphan 'stray', got %v", r.Orphaned)
	}
}

func TestIntegrity_SubtasksAndTagsCountAsReferences(t *testing.T) {
	r := check(t, storeFixture{
		active:   []map[string]any{task("parent", "sub1"), task("sub1"), task("tagged")},
		projects: []map[string]any{project("parent")},
		tags:     []map[string]any{tag("tagged")},
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

// --- Run() exit-code contract ---
//
// The point of a distinct code 3 is that a script can tell "cannot reach SP"
// from "SP answered and its data is broken". --json is the documented scripting
// mode, so it is the path where collapsing them matters most.

func runWithStore(t *testing.T, f storeFixture, args ...string) int {
	t.Helper()
	srv := newStoreServer(t, f)
	defer srv.Close()
	t.Setenv("SP_BASE_URL", srv.URL)
	return Run(args)
}

func TestRun_JSONReturnsThreeWhenStoreInconsistent(t *testing.T) {
	code := runWithStore(t, storeFixture{
		active:   []map[string]any{task("t1")},
		projects: []map[string]any{project("t1", "GHOST")},
	}, "--json")
	if code != 3 {
		t.Fatalf("inconsistent store via --json must exit 3, got %d", code)
	}
}

func TestRun_JSONReturnsZeroWhenClean(t *testing.T) {
	code := runWithStore(t, storeFixture{
		active:   []map[string]any{task("t1")},
		projects: []map[string]any{project("t1")},
	}, "--json")
	if code != 0 {
		t.Fatalf("clean store via --json must exit 0, got %d", code)
	}
}

func TestRun_JSONReturnsOneWhenUnreachable(t *testing.T) {
	// Reserved-for-documentation port that nothing listens on.
	t.Setenv("SP_BASE_URL", "http://127.0.0.1:1")
	if code := Run([]string{"--json"}); code != 1 {
		t.Fatalf("unreachable SP must exit 1, not be confused with 3; got %d", code)
	}
}

// A mistyped flag used to be swallowed, running a shallow check that then
// reported "All checks passed" — the user believed the integrity check ran.
func TestRun_RejectsStrayPositional(t *testing.T) {
	if code := Run([]string{"deep"}); code != 2 {
		t.Fatalf("stray positional must exit 2, got %d", code)
	}
}

func TestRun_RejectsUnknownFlag(t *testing.T) {
	if code := Run([]string{"--nope"}); code != 2 {
		t.Fatalf("unknown flag must exit 2, got %d", code)
	}
}

func TestRun_HelpExitsZero(t *testing.T) {
	if code := Run([]string{"--help"}); code != 0 {
		t.Fatalf("--help must exit 0, got %d", code)
	}
}

// --- Degenerate-but-successful responses ---
//
// The client reports Success(nil) for `{"ok":true,"data":null}`, for a 2xx with
// an empty body, and for a 2xx carrying non-JSON. Reading any of those as "zero
// entities" made a healthy store look corrupt — and the warning tells the user
// not to import a backup, which is precisely the wrong advice when the store is
// fine.

func rawServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestIntegrity_NullIndexPayloadIsAnError(t *testing.T) {
	srv := rawServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/projects" || r.URL.Path == "/tags" {
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": nil})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{task("t1")}})
	})
	_, err := CheckIntegrity(context.Background(), bridge.NewClient(srv.URL))
	if err == nil {
		t.Fatal("a null index payload must be an error, not zero references")
	}
	if !strings.Contains(err.Error(), "/projects") {
		t.Fatalf("error should name the endpoint that failed, got: %v", err)
	}
}

func TestIntegrity_EmptyBodyIsAnError(t *testing.T) {
	srv := rawServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tasks" && r.URL.Query().Get("source") == "active" {
			w.WriteHeader(http.StatusOK) // 200, no body
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{}})
	})
	if _, err := CheckIntegrity(context.Background(), bridge.NewClient(srv.URL)); err == nil {
		t.Fatal("a 200 with an empty body must be an error, not an empty task list")
	}
}

// An empty array is a legitimate answer and must stay legal.
func TestIntegrity_EmptyArraysAreValid(t *testing.T) {
	r := check(t, storeFixture{})
	if !r.Clean() {
		t.Fatalf("an empty store is consistent, got %+v", r)
	}
}

func TestIntegrity_ErrorNamesEndpointAndCode(t *testing.T) {
	srv := rawServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/tags" {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]any{
				"ok": false, "error": map[string]any{"code": "SP_ERROR", "message": "boom"},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{}})
	})
	_, err := CheckIntegrity(context.Background(), bridge.NewClient(srv.URL))
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"/tags", "SP_ERROR", "boom"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got: %v", want, err)
		}
	}
}

// A task in both pools is a partially-applied archive — the kind of
// inconsistency this check exists for. Subtracting pool sizes hid it.
func TestIntegrity_DetectsTaskInBothPools(t *testing.T) {
	r := check(t, storeFixture{
		active:   []map[string]any{task("t1"), task("both")},
		archived: []map[string]any{task("both")},
		projects: []map[string]any{project("t1", "both")},
	})
	if len(r.Duplicated) != 1 || r.Duplicated[0] != "both" {
		t.Fatalf("expected 'both' flagged as duplicated, got %v", r.Duplicated)
	}
	if r.Clean() {
		t.Fatal("a task in both pools must not report clean")
	}
	if r.ArchivedTasks != 1 {
		t.Fatalf("archived count must be counted directly, got %d", r.ArchivedTasks)
	}
}

// --- exit-code precedence ---

func TestExitCode_FailureOutranksInconsistency(t *testing.T) {
	cases := []struct {
		failures     int
		inconsistent bool
		want         int
	}{
		{0, false, 0},
		{0, true, 3},
		{1, false, 1},
		{1, true, 1}, // a failed request makes the integrity verdict untrustworthy
	}
	for _, c := range cases {
		if got := exitCode(c.failures, c.inconsistent); got != c.want {
			t.Errorf("exitCode(%d, %v) = %d, want %d", c.failures, c.inconsistent, got, c.want)
		}
	}
}

func TestUsage_WritesToProvidedWriter(t *testing.T) {
	var buf strings.Builder
	usage(&buf)
	if !strings.Contains(buf.String(), "--deep") {
		t.Fatalf("usage should describe --deep, got: %s", buf.String())
	}
}

// --- Confirmation pass ---
//
// The four pulls are not an atomic snapshot. A task added in the UI between the
// task pull and the project pull looks exactly like a dangling reference, and
// that false verdict tells the user not to restore a backup. A real
// inconsistency survives a second pass; a race does not.

// racingServer reports an extra project reference on the first round only,
// simulating a task added mid-check.
func racingServer(t *testing.T) *httptest.Server {
	t.Helper()
	round := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		enc := func(v any) { json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": v}) }
		switch {
		case r.URL.Path == "/tasks" && r.URL.Query().Get("source") == "archived":
			enc([]map[string]any{})
		case r.URL.Path == "/tasks":
			enc([]map[string]any{task("t1")})
		case r.URL.Path == "/projects":
			round++
			if round == 1 {
				enc([]map[string]any{project("t1", "RACE")}) // transient
				return
			}
			enc([]map[string]any{project("t1")})
		default:
			enc([]map[string]any{})
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestConfirmed_TransientAnomalyIsDropped(t *testing.T) {
	srv := racingServer(t)
	r, err := CheckIntegrityConfirmed(context.Background(), bridge.NewClient(srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.HasConfirmedAnomalies() {
		t.Fatalf("a transient anomaly must not be confirmed: %+v", r)
	}
	if r.Confirmed() {
		t.Fatal("a one-pass-only anomaly leaves the run unconfirmed, not clean")
	}
	if len(r.Unresolved) != 1 || r.Unresolved[0] != "RACE" {
		t.Fatalf("the transient id must be recorded as unresolved, got %v", r.Unresolved)
	}
}

func TestConfirmed_PersistentAnomalyIsKept(t *testing.T) {
	srv := newStoreServer(t, storeFixture{
		active:   []map[string]any{task("t1")},
		projects: []map[string]any{project("t1", "GHOST")},
	})
	defer srv.Close()
	r, err := CheckIntegrityConfirmed(context.Background(), bridge.NewClient(srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Dangling) != 1 || r.Dangling[0] != "GHOST" {
		t.Fatalf("a persistent anomaly must survive confirmation, got %v", r.Dangling)
	}
	if len(r.Unresolved) != 0 {
		t.Fatalf("nothing was unresolved here, got %v", r.Unresolved)
	}
}

func TestConfirmed_CleanStoreSkipsSecondPass(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/projects" {
			calls++
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/projects" {
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{project("t1")}})
			return
		}
		if r.URL.Path == "/tasks" && r.URL.Query().Get("source") != "archived" {
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{task("t1")}})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{}})
	}))
	defer srv.Close()
	if _, err := CheckIntegrityConfirmed(context.Background(), bridge.NewClient(srv.URL)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("a clean store must not pay for a second pass, got %d /projects calls", calls)
	}
}

func TestIntersect(t *testing.T) {
	if got := intersect([]string{"a", "b", "c"}, []string{"b", "c", "d"}); len(got) != 2 || got[0] != "b" || got[1] != "c" {
		t.Fatalf("got %v", got)
	}
	if got := intersect(nil, []string{"a"}); got != nil {
		t.Fatalf("empty input must yield nil, got %v", got)
	}
}

// Silently dropping a malformed element reaches the same wrong answer the
// null-payload guard exists to prevent, via a different door.
func TestIntegrity_MalformedElementIsAnError(t *testing.T) {
	srv := rawServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/tasks" && r.URL.Query().Get("source") != "archived" {
			w.Write([]byte(`{"ok":true,"data":[{"id":"t1"},null,{"id":"t2"}]}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{}})
	})
	_, err := CheckIntegrity(context.Background(), bridge.NewClient(srv.URL))
	if err == nil {
		t.Fatal("a null array element must be an error, not a silently dropped task")
	}
	if !strings.Contains(err.Error(), "usable id") {
		t.Fatalf("error should explain the cause, got: %v", err)
	}
}

func TestIntegrity_TaskWithoutIDIsAnError(t *testing.T) {
	srv := rawServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/tasks" && r.URL.Query().Get("source") != "archived" {
			w.Write([]byte(`{"ok":true,"data":[{"id":"t1"},{"title":"no id"}]}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{}})
	})
	if _, err := CheckIntegrity(context.Background(), bridge.NewClient(srv.URL)); err == nil {
		t.Fatal("a task without an id must be an error")
	}
}

// The documented JSON contract must actually contain every documented key.
func TestIntegrityJSON_ContainsDocumentedKeys(t *testing.T) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(integrityJSON(IntegrityReport{})), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, key := range []string{
		"activeTasks", "archivedTasks", "referenced",
		"dangling", "orphaned", "duplicated",
		"unresolved", "unconfirmed", "clean",
	} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("documented key %q missing from --json output", key)
		}
	}
}

// --- Unconfirmed reports are not verdicts ---
//
// Exit 3 promises "SP answered and its data is broken". Claiming that from a
// single observation is exactly the false positive the confirmation pass was
// added to prevent, and reporting clean when only the second pass saw anomalies
// would hide real corruption.

// shiftingServer reports a different anomaly on each pass, so the intersection
// is empty even though both passes saw one.
func shiftingServer(t *testing.T) *httptest.Server {
	t.Helper()
	round := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		enc := func(v any) { json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": v}) }
		switch {
		case r.URL.Path == "/tasks" && r.URL.Query().Get("source") == "archived":
			enc([]map[string]any{})
		case r.URL.Path == "/tasks":
			enc([]map[string]any{task("t1")})
		case r.URL.Path == "/projects":
			round++
			ghost := "GHOST2"
			if round == 1 {
				ghost = "GHOST1"
			}
			enc([]map[string]any{project("t1", ghost)})
		default:
			enc([]map[string]any{})
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestConfirmed_ShiftingAnomaliesAreNotReportedClean(t *testing.T) {
	srv := shiftingServer(t)
	r, err := CheckIntegrityConfirmed(context.Background(), bridge.NewClient(srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Confirmed() {
		t.Fatal("passes disagreed; the report must be marked unconfirmed")
	}
	if r.HasConfirmedAnomalies() {
		t.Fatal("precondition: the intersection here is empty")
	}
}

func TestRun_ShiftingAnomaliesExitOneNotZero(t *testing.T) {
	srv := shiftingServer(t)
	t.Setenv("SP_BASE_URL", srv.URL)
	if code := Run([]string{"--json"}); code != 1 {
		t.Fatalf("an unconfirmed report must exit 1, got %d", code)
	}
}

// The confirmation pass failing means every anomaly was seen once.
func TestConfirmed_FailedSecondPassIsUnconfirmed(t *testing.T) {
	round := 0
	srv := rawServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/projects" {
			round++
			if round > 1 {
				w.WriteHeader(500)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		enc := func(v any) { json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": v}) }
		switch {
		case r.URL.Path == "/tasks" && r.URL.Query().Get("source") == "archived":
			enc([]map[string]any{})
		case r.URL.Path == "/tasks":
			enc([]map[string]any{task("t1")})
		case r.URL.Path == "/projects":
			enc([]map[string]any{project("t1", "GHOST")})
		default:
			enc([]map[string]any{})
		}
	})
	r, err := CheckIntegrityConfirmed(context.Background(), bridge.NewClient(srv.URL))
	if err != nil {
		t.Fatalf("a failed confirmation is not a hard error: %v", err)
	}
	if r.Confirmed() {
		t.Fatal("a failed confirmation pass must mark the report unconfirmed")
	}
	// round is captured by the handler closure and is already past its first
	// value, so without resetting it Run's *first* pass would take the 500 and
	// return via the hard-error path, never exercising the unconfirmed case.
	round = 0
	t.Setenv("SP_BASE_URL", srv.URL)
	if code := Run([]string{"--json"}); code != 1 {
		t.Fatalf("an unconfirmed report must exit 1, got %d", code)
	}
}

// --json documents stdout as a JSON stream, so help must not land there.
func TestRun_HelpWithJSONKeepsStdoutClean(t *testing.T) {
	if code := Run([]string{"--json", "--help"}); code != 0 {
		t.Fatalf("--help must exit 0, got %d", code)
	}
}

// main.go strips the subcommand word before calling Run, so seeing it here at
// any position is a typo. It used to be tolerated at index 0, which meant the
// multicall alias silently accepted `sp-local-bridge-doctor doctor` and ran a
// shallow check while printing "All checks passed".
func TestRun_SubcommandWordIsAlwaysRejected(t *testing.T) {
	for _, args := range [][]string{{"doctor"}, {"--deep", "doctor"}, {"doctor", "--deep"}} {
		if code := Run(args); code != 2 {
			t.Errorf("Run(%v) must exit 2, got %d", args, code)
		}
	}
}

// A confirmed anomaly stands on its own. A race elsewhere in the store does not
// make it less real, and gating it behind "unconfirmed" meant one transient id
// could permanently mask corruption on a live app — which is the only kind of
// app --deep is ever run against.
func mixedServer(t *testing.T) *httptest.Server {
	t.Helper()
	round := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		enc := func(v any) { json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": v}) }
		switch {
		case r.URL.Path == "/tasks" && r.URL.Query().Get("source") == "archived":
			enc([]map[string]any{})
		case r.URL.Path == "/tasks":
			enc([]map[string]any{task("t1")})
		case r.URL.Path == "/projects":
			round++
			ids := []string{"t1", "GHOST"} // GHOST persists
			if round == 2 {
				ids = append(ids, "LATE") // LATE appears once
			}
			enc([]map[string]any{project(ids...)})
		default:
			enc([]map[string]any{})
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestConfirmed_PersistentAnomalySurvivesATransientOne(t *testing.T) {
	r, err := CheckIntegrityConfirmed(context.Background(), bridge.NewClient(mixedServer(t).URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.HasConfirmedAnomalies() {
		t.Fatal("GHOST was seen twice and must be confirmed")
	}
	if len(r.Dangling) != 1 || r.Dangling[0] != "GHOST" {
		t.Fatalf("confirmed set should hold only GHOST, got %v", r.Dangling)
	}
	if len(r.Unresolved) != 1 || r.Unresolved[0] != "LATE" {
		t.Fatalf("LATE was seen once and belongs in Unresolved, got %v", r.Unresolved)
	}
}

func TestRun_ConfirmedAnomalyStillExitsThree(t *testing.T) {
	t.Setenv("SP_BASE_URL", mixedServer(t).URL)
	if code := Run([]string{"--json"}); code != 3 {
		t.Fatalf("a twice-seen anomaly must exit 3 even alongside a transient one, got %d", code)
	}
}

// clean must never be true when either pass saw an anomaly: a script doing
// `doctor --json | jq -e .clean` would otherwise read true for a store both
// passes found broken.
func TestIntegrityJSON_NotCleanWhenAnythingWasSeen(t *testing.T) {
	for name, report := range map[string]IntegrityReport{
		"confirmed":   {Dangling: []string{"GHOST"}},
		"unresolved":  {Unresolved: []string{"G1", "G2"}, Unconfirmed: true},
		"unconfirmed": {Unconfirmed: true},
	} {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(integrityJSON(report)), &parsed); err != nil {
			t.Fatalf("%s: invalid JSON: %v", name, err)
		}
		if parsed["clean"] == true {
			t.Errorf("%s: clean must not be true, got %s", name, integrityJSON(report))
		}
	}
}

// When the confirmation pass cannot run, nothing was seen twice, so the
// confirmed sets must be empty rather than holding the first pass's findings.
func TestConfirmed_FailedSecondPassConfirmsNothing(t *testing.T) {
	round := 0
	srv := rawServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/projects" {
			round++
			if round > 1 {
				w.WriteHeader(500)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		enc := func(v any) { json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": v}) }
		switch {
		case r.URL.Path == "/tasks" && r.URL.Query().Get("source") == "archived":
			enc([]map[string]any{})
		case r.URL.Path == "/tasks":
			enc([]map[string]any{task("t1")})
		case r.URL.Path == "/projects":
			enc([]map[string]any{project("t1", "GHOST")})
		default:
			enc([]map[string]any{})
		}
	})
	r, err := CheckIntegrityConfirmed(context.Background(), bridge.NewClient(srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.HasConfirmedAnomalies() {
		t.Fatalf("nothing survived two passes; confirmed sets must be empty, got %+v", r)
	}
	if len(r.Unresolved) != 1 || r.Unresolved[0] != "GHOST" {
		t.Fatalf("the single observation belongs in Unresolved, got %v", r.Unresolved)
	}
}

// When the confirmation pass errors, the report must say so. Blaming concurrent
// editing hides the real cause at the worst moment: a store whose app just went
// down is exactly when its anomalies matter.
func TestConfirmed_FailedSecondPassCarriesTheReason(t *testing.T) {
	round := 0
	srv := rawServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/projects" {
			round++
			if round > 1 {
				w.WriteHeader(500)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		enc := func(v any) { json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": v}) }
		switch {
		case r.URL.Path == "/tasks" && r.URL.Query().Get("source") == "archived":
			enc([]map[string]any{})
		case r.URL.Path == "/tasks":
			enc([]map[string]any{task("t1")})
		case r.URL.Path == "/projects":
			enc([]map[string]any{project("t1", "GHOST")})
		default:
			enc([]map[string]any{})
		}
	})
	r, err := CheckIntegrityConfirmed(context.Background(), bridge.NewClient(srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.UnconfirmedReason == "" {
		t.Fatal("a failed confirmation pass must record why")
	}
	if !strings.Contains(r.UnconfirmedReason, "/projects") {
		t.Fatalf("the reason should name the failing endpoint, got %q", r.UnconfirmedReason)
	}
}

// Disagreement between two successful passes is a different situation and must
// not carry a failure reason.
func TestConfirmed_DisagreementHasNoFailureReason(t *testing.T) {
	r, err := CheckIntegrityConfirmed(context.Background(), bridge.NewClient(shiftingServer(t).URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.Unconfirmed {
		t.Fatal("precondition: passes disagreed")
	}
	if r.UnconfirmedReason != "" {
		t.Fatalf("no request failed here; reason should be empty, got %q", r.UnconfirmedReason)
	}
}

// Orphaned and Duplicated overlap: a task in both pools that nothing references
// appears in both. Concatenating them raw reported one id as two.
func TestConfirmed_FailedSecondPassDeduplicatesUnresolved(t *testing.T) {
	round := 0
	srv := rawServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/projects" {
			round++
			if round > 1 {
				w.WriteHeader(500)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		enc := func(v any) { json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": v}) }
		switch {
		case r.URL.Path == "/tasks" && r.URL.Query().Get("source") == "archived":
			enc([]map[string]any{task("both")}) // in both pools
		case r.URL.Path == "/tasks":
			enc([]map[string]any{task("both")}) // and unreferenced
		default:
			enc([]map[string]any{})
		}
	})
	r, err := CheckIntegrityConfirmed(context.Background(), bridge.NewClient(srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Unresolved) != 1 || r.Unresolved[0] != "both" {
		t.Fatalf("an id appearing in two categories must be listed once, got %v", r.Unresolved)
	}
}

// --- Index-side degenerate payloads (#33) ---
//
// Reading a missing index field as "this project references nothing" turns
// every task it owns into an orphan. That is a healthy store reported as
// corrupt, with advice not to restore a backup — and unlike a race it is
// deterministic, so it survives both passes and is reported as *confirmed*.

func serverWithProjects(t *testing.T, projectsJSON string) *httptest.Server {
	t.Helper()
	return rawServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/projects":
			w.Write([]byte(`{"ok":true,"data":` + projectsJSON + `}`))
		case r.URL.Path == "/tasks" && r.URL.Query().Get("source") == "archived":
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{}})
		case r.URL.Path == "/tasks":
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{task("t1"), task("t2")}})
		default:
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{}})
		}
	})
}

// assertIndexUnreadable pins the contract for an unreadable index: no
// reference-derived verdict is emitted (that was the false-orphans bug), the
// run is marked unconfirmed, and the reason names what broke.
func assertIndexUnreadable(t *testing.T, srv *httptest.Server, want ...string) IntegrityReport {
	t.Helper()
	r, err := CheckIntegrity(context.Background(), bridge.NewClient(srv.URL))
	if err != nil {
		t.Fatalf("an unreadable index degrades the run, it does not fail it: %v", err)
	}
	if !r.Unconfirmed || r.UnconfirmedReason == "" {
		t.Fatalf("expected an unconfirmed report with a reason, got %+v", r)
	}
	if len(r.Dangling) > 0 || len(r.Orphaned) > 0 {
		t.Fatalf("reference-derived verdicts must be withheld, got dangling=%v orphaned=%v", r.Dangling, r.Orphaned)
	}
	if r.Clean() {
		t.Fatal("an unreadable index is not a clean result")
	}
	for _, w := range want {
		if !strings.Contains(r.UnconfirmedReason, w) {
			t.Errorf("reason should mention %q, got: %s", w, r.UnconfirmedReason)
		}
	}
	return r
}

func TestIntegrity_MissingIndexFieldIsAnError(t *testing.T) {
	// The reviewer's exact repro: a project with no taskIds at all. Before the
	// guard this reported both tasks as orphaned and exited 3.
	srv := serverWithProjects(t, `[{"id":"p1","title":"Inbox"}]`)
	assertIndexUnreadable(t, srv, "/projects", "taskIds", "p1")
}

func TestIntegrity_NullIndexFieldIsAnError(t *testing.T) {
	srv := serverWithProjects(t, `[{"id":"p1","taskIds":null,"backlogTaskIds":[]}]`)
	assertIndexUnreadable(t, srv, "taskIds")
}

func TestIntegrity_NonStringIndexEntryIsAnError(t *testing.T) {
	srv := serverWithProjects(t, `[{"id":"p1","taskIds":["t1",42],"backlogTaskIds":[]}]`)
	assertIndexUnreadable(t, srv, "taskIds")
}

// An empty index is legal: a project with no tasks is normal.
func TestIntegrity_EmptyIndexFieldIsValid(t *testing.T) {
	srv := rawServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/projects" {
			w.Write([]byte(`{"ok":true,"data":[{"id":"p1","taskIds":[],"backlogTaskIds":[]}]}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{}})
	})
	r, err := CheckIntegrity(context.Background(), bridge.NewClient(srv.URL))
	if err != nil {
		t.Fatalf("an empty index is normal, got: %v", err)
	}
	if !r.Clean() {
		t.Fatalf("an empty store is consistent, got %+v", r)
	}
}

// backlogTaskIds is required too — a rename there would otherwise hide every
// backlog task from the reference set.
func TestIntegrity_MissingBacklogFieldIsAnError(t *testing.T) {
	srv := serverWithProjects(t, `[{"id":"p1","taskIds":["t1","t2"]}]`)
	assertIndexUnreadable(t, srv, "backlogTaskIds")
}

// Asserting only the exit code proves nothing here: last-wins returns 2 as
// well. The message is the behaviour under test — naming the last bad argument
// sends the user round the loop once per typo.
func TestRun_ReportsFirstBadArgument(t *testing.T) {
	stderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	code := Run([]string{"--nope", "--alsonope"})
	w.Close()
	os.Stderr = stderr

	out, _ := io.ReadAll(r)
	if code != 2 {
		t.Fatalf("want exit 2, got %d", code)
	}
	msg := string(out)
	if !strings.Contains(msg, "--nope") {
		t.Fatalf("should name the first bad argument, got: %s", msg)
	}
	if strings.Contains(msg, "--alsonope") {
		t.Fatalf("should not name the later one, got: %s", msg)
	}
}

// --- The other three collectIDs call sites ---
//
// The degenerate-payload tests above all go through /projects. Each remaining
// call site is a separate guard that nothing else pins: dropping the
// archivedTasks loop, for instance, would not fail any other test.

func serverWithRaw(t *testing.T, path, query, body string) *httptest.Server {
	t.Helper()
	return rawServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == path && (query == "" || r.URL.Query().Get("source") == query) {
			w.Write([]byte(`{"ok":true,"data":` + body + `}`))
			return
		}
		switch {
		case r.URL.Path == "/projects":
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{project()}})
		case r.URL.Path == "/tasks":
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{}})
		default:
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{}})
		}
	})
}

func TestIntegrity_TagMissingTaskIDsIsAnError(t *testing.T) {
	srv := serverWithRaw(t, "/tags", "", `[{"id":"g1","title":"Today"}]`)
	assertIndexUnreadable(t, srv, "/tags", "g1")
}

func TestIntegrity_ActiveTaskMissingSubTaskIDsIsAnError(t *testing.T) {
	srv := serverWithRaw(t, "/tasks", "active", `[{"id":"t1","title":"t1"}]`)
	assertIndexUnreadable(t, srv, "t1")
}

// The archived payload's shape differs from the active one (archived objects
// carry a subTasks key that active objects lack), so this call site is the one
// most likely to drift.
func TestIntegrity_ArchivedTaskMissingSubTaskIDsIsAnError(t *testing.T) {
	// The archived payload's shape differs from the active one (archived objects
	// carry a subTasks key that active objects lack), so this call site is the
	// one most likely to drift.
	srv := serverWithRaw(t, "/tasks", "archived", `[{"id":"a1","title":"a1","subTasks":[]}]`)
	assertIndexUnreadable(t, srv, "archived", "a1")
}

// The entity id is the most actionable thing in a real corruption event.
func TestIntegrity_ErrorNamesTheOffendingEntity(t *testing.T) {
	srv := serverWithProjects(t, `[{"id":"p_broken","title":"Inbox"}]`)
	assertIndexUnreadable(t, srv, "p_broken")
}

// Duplicated is derived from the two task pools alone; no index can invalidate
// it. Aborting on an unreadable index used to discard it along with the
// reference-derived verdicts, throwing away the most actionable signal in
// exactly the corruption event this check exists for.
func TestIntegrity_PoolFindingsSurviveAnUnreadableIndex(t *testing.T) {
	srv := rawServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/projects":
			w.Write([]byte(`{"ok":true,"data":[{"id":"p_broken","title":"Inbox"}]}`))
		case r.URL.Path == "/tasks" && r.URL.Query().Get("source") == "archived":
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{task("both")}})
		case r.URL.Path == "/tasks":
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{task("both")}})
		default:
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{}})
		}
	})
	r, err := CheckIntegrity(context.Background(), bridge.NewClient(srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Duplicated) != 1 || r.Duplicated[0] != "both" {
		t.Fatalf("pool-derived findings must survive, got %v", r.Duplicated)
	}
	if r.ActiveTasks != 1 || r.ArchivedTasks != 1 {
		t.Fatalf("pool counts must survive, got active=%d archived=%d", r.ActiveTasks, r.ArchivedTasks)
	}
	if !r.Unconfirmed {
		t.Fatal("the run is still unconfirmed: the reference set could not be read")
	}
}

// The confirmation pass must not clear a reason that came from the pass itself.
func TestConfirmed_IndexReasonSurvivesConfirmation(t *testing.T) {
	srv := serverWithProjects(t, `[{"id":"p1","title":"Inbox"}]`)
	r, err := CheckIntegrityConfirmed(context.Background(), bridge.NewClient(srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.Unconfirmed || r.UnconfirmedReason == "" {
		t.Fatalf("an unreadable index must stay unconfirmed through confirmation, got %+v", r)
	}
}

// bad doubled as both value and flag, so an argument that *is* the empty string
// was unreportable: `doctor "$UNSET_VAR"` ran a silently shallow check.
func TestRun_EmptyStringArgumentIsRejected(t *testing.T) {
	if code := Run([]string{""}); code != 2 {
		t.Fatalf("an empty-string argument must exit 2, got %d", code)
	}
}

// The degraded early return sits above the sort at the end of CheckIntegrity,
// so Duplicated kept raw map iteration order and --json's output varied run to
// run. Many ids and repeated runs, since one duplicate cannot show ordering.
func TestIntegrity_DuplicatedIsSortedOnTheDegradedPath(t *testing.T) {
	ids := []string{"d5", "d1", "d4", "d2", "d6", "d3"}
	pool := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		pool = append(pool, task(id))
	}
	srv := rawServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/projects":
			w.Write([]byte(`{"ok":true,"data":[{"id":"p_broken","title":"Inbox"}]}`))
		case r.URL.Path == "/tasks":
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": pool})
		default:
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{}})
		}
	})
	want := []string{"d1", "d2", "d3", "d4", "d5", "d6"}
	for i := 0; i < 12; i++ {
		r, err := CheckIntegrity(context.Background(), bridge.NewClient(srv.URL))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(r.Duplicated) != len(want) {
			t.Fatalf("run %d: got %v", i, r.Duplicated)
		}
		for j, id := range want {
			if r.Duplicated[j] != id {
				t.Fatalf("run %d: Duplicated must be sorted, got %v", i, r.Duplicated)
			}
		}
	}
}

// A partial reference count read as authoritative invites the wrong conclusion.
func TestIntegrity_ReferencedIsZeroedWhenIndexUnreadable(t *testing.T) {
	srv := serverWithProjects(t, `[{"id":"p_ok","taskIds":["t1","t2"],"backlogTaskIds":[]},{"id":"p_broken","title":"Inbox"}]`)
	r, err := CheckIntegrity(context.Background(), bridge.NewClient(srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Referenced != 0 {
		t.Fatalf("a partial reference count must not be reported, got %d", r.Referenced)
	}
}

// A deterministic index error will fail identically on a re-pull, so the second
// full store pull is pure cost.
func TestConfirmed_IndexErrorSkipsTheSecondPass(t *testing.T) {
	calls := 0
	srv := rawServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/projects" {
			calls++
			w.Write([]byte(`{"ok":true,"data":[{"id":"p1","title":"Inbox"}]}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{}})
	})
	if _, err := CheckIntegrityConfirmed(context.Background(), bridge.NewClient(srv.URL)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("a deterministic index error must not trigger a re-pull, got %d /projects calls", calls)
	}
}

// A task can be orphaned and duplicated at the same time, so reconciling by id
// alone let the confirmed "orphaned" verdict swallow a "duplicated" observation
// the two passes disagreed about. The id was still surfaced, so the user was
// told the right task to look at — what vanished was the second symptom, which
// is the hint about *how* it broke: a partially applied archive reads very
// differently from a stray reference. Worse, Confirmed() then claimed the passes
// agreed on everything when they had not.
func TestConfirmed_CategoriesAreReconciledIndependently(t *testing.T) {
	round := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		enc := func(v []map[string]any) {
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": v})
		}
		switch {
		case r.URL.Path == "/tasks" && r.URL.Query().Get("source") == "archived":
			// "both" is in the archive on the first pass only: duplicated once.
			round++
			if round == 1 {
				enc([]map[string]any{task("both")})
				return
			}
			enc([]map[string]any{})
		case r.URL.Path == "/tasks":
			// "both" is active and referenced by nothing: orphaned every pass.
			enc([]map[string]any{task("t1"), task("both")})
		case r.URL.Path == "/projects":
			enc([]map[string]any{project("t1")})
		default:
			enc([]map[string]any{})
		}
	}))
	defer srv.Close()

	r, err := CheckIntegrityConfirmed(context.Background(), bridge.NewClient(srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The orphan survived both passes and is still a verdict.
	if len(r.Orphaned) != 1 || r.Orphaned[0] != "both" {
		t.Errorf("expected orphaned [both], got %v", r.Orphaned)
	}
	// The duplication was seen once, so it is not a verdict...
	if len(r.Duplicated) != 0 {
		t.Errorf("a once-seen duplication must not be confirmed, got %v", r.Duplicated)
	}
	// ...but it must not disappear either.
	if len(r.Unresolved) != 1 || r.Unresolved[0] != "both" {
		t.Errorf("expected the once-seen observation recorded as unresolved, got %v", r.Unresolved)
	}
	if r.Confirmed() {
		t.Error("the passes disagreed about the duplicated category, so Confirmed() must be false")
	}
}

// Unresolved is a list of ids, so an id the passes disagree about in two
// categories at once is still reported once.
//
// The pass is counted on the *active* request because CheckIntegrity fetches
// active before archived. Counting it on the archived request made an earlier
// version of this test vacuous: the active branch read the counter before it had
// been incremented, "ghost" never entered the active pool, pass one came back
// clean, and CheckIntegrityConfirmed short-circuited before any of the
// assertions below could mean anything.
func TestConfirmed_UnresolvedIDsAreNotDuplicatedAcrossCategories(t *testing.T) {
	pass := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		enc := func(v []map[string]any) {
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": v})
		}
		switch {
		case r.URL.Path == "/tasks" && r.URL.Query().Get("source") == "archived":
			if pass == 1 {
				enc([]map[string]any{task("ghost")})
				return
			}
			enc([]map[string]any{})
		case r.URL.Path == "/tasks":
			pass++
			if pass == 1 {
				// First pass only. "ghost" is active and unreferenced, so it is
				// orphaned, and it is in the archive too, so it is duplicated:
				// one id, two categories, both seen exactly once.
				enc([]map[string]any{task("t1"), task("ghost")})
				return
			}
			enc([]map[string]any{task("t1")})
		case r.URL.Path == "/projects":
			enc([]map[string]any{project("t1")})
		default:
			enc([]map[string]any{})
		}
	}))
	defer srv.Close()

	r, err := CheckIntegrityConfirmed(context.Background(), bridge.NewClient(srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Guard the guard: if the fixture stops producing a disagreement, the dedup
	// assertion below would pass on an empty list and prove nothing.
	if len(r.Unresolved) == 0 {
		t.Fatal("fixture produced no disagreement, so this test would verify nothing")
	}
	seen := map[string]int{}
	for _, id := range r.Unresolved {
		seen[id]++
	}
	if seen["ghost"] != 1 {
		t.Errorf("ghost was seen once in two categories, so it must be listed once; got %v", r.Unresolved)
	}
}

// --- The human --deep rendering ---
//
// printIntegrity had no test at all, yet its return value is what Run() turns
// into the difference between exit 1 and exit 3: a false return with confirmed
// anomalies means "inconsistent", a false return without them means "a check did
// not complete". exitCode() was pinned at the unit level; the value fed into it
// was not.

// captureIntegrity runs printIntegrity with stdout redirected, returning what it
// printed and what it returned.
func captureIntegrity(t *testing.T, report IntegrityReport) (string, bool) {
	t.Helper()
	stdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	// Restored with defer: a panic in printIntegrity would otherwise leave every
	// later test in this package writing into a closed pipe.
	defer func() { os.Stdout = stdout }()
	os.Stdout = w

	clean := printIntegrity(report)
	w.Close()

	out, _ := io.ReadAll(r)
	return string(out), clean
}

func TestPrintIntegrity_CleanStore(t *testing.T) {
	out, clean := captureIntegrity(t, IntegrityReport{ActiveTasks: 3, Referenced: 3})
	if !clean {
		t.Error("a clean report must return true")
	}
	if !strings.Contains(out, "OK") {
		t.Errorf("expected OK, got:\n%s", out)
	}
	if strings.Contains(out, "WARN") || strings.Contains(out, "UNCONFIRMED") {
		t.Errorf("a clean report must not warn, got:\n%s", out)
	}
}

func TestPrintIntegrity_ConfirmedAnomalyWarnsAndBlocksBackupImport(t *testing.T) {
	out, clean := captureIntegrity(t, IntegrityReport{
		ActiveTasks: 1, Dangling: []string{"GHOST"},
	})
	if clean {
		t.Error("a confirmed anomaly must return false")
	}
	if !strings.Contains(out, "WARN") {
		t.Errorf("expected WARN, got:\n%s", out)
	}
	if !strings.Contains(out, "GHOST") {
		t.Errorf("expected the offending id, got:\n%s", out)
	}
	// The backup advice is the load-bearing line: a backup taken now captures
	// the same in-memory inconsistency.
	if !strings.Contains(out, "Do NOT import a backup") {
		t.Errorf("expected the backup warning, got:\n%s", out)
	}
}

// An unconfirmed report is not a verdict, so it must not render as one — but it
// must not render as OK either.
func TestPrintIntegrity_UnconfirmedIsNeitherWarnNorOK(t *testing.T) {
	out, clean := captureIntegrity(t, IntegrityReport{
		ActiveTasks: 1, Unresolved: []string{"MAYBE"}, Unconfirmed: true,
	})
	if clean {
		t.Error("an unconfirmed report must return false")
	}
	if !strings.Contains(out, "UNCONFIRMED") {
		t.Errorf("expected UNCONFIRMED, got:\n%s", out)
	}
	if strings.Contains(out, "WARN") {
		t.Errorf("a once-seen observation is not a verdict, got:\n%s", out)
	}
	// The name says "neither", so check both halves: the status line must not
	// read OK either, or the caller is told a store it could not reconcile is
	// fine. Matched on the line itself, since "OK" appears inside UNCONFIRMED.
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "OK" {
			t.Errorf("an unconfirmed report must not read OK, got:\n%s", out)
		}
	}
	if !strings.Contains(out, "MAYBE") {
		t.Errorf("expected the once-seen id, got:\n%s", out)
	}
}

// A failed confirmation pass and a disagreement between two good passes both
// leave ids in Unresolved, but they mean different things and the wording says
// so. Getting this backwards tells the user their store is being edited when SP
// actually went down.
func TestPrintIntegrity_UnconfirmedReasonChangesTheExplanation(t *testing.T) {
	withReason, _ := captureIntegrity(t, IntegrityReport{
		Unresolved: []string{"X"}, Unconfirmed: true, UnconfirmedReason: "connection refused",
	})
	if !strings.Contains(withReason, "not necessarily transient") {
		t.Errorf("a failed pass must not be blamed on concurrent editing, got:\n%s", withReason)
	}
	if !strings.Contains(withReason, "connection refused") {
		t.Errorf("expected the carried reason, got:\n%s", withReason)
	}

	noReason, _ := captureIntegrity(t, IntegrityReport{
		Unresolved: []string{"X"}, Unconfirmed: true,
	})
	if !strings.Contains(noReason, "being edited while the check ran") {
		t.Errorf("a disagreement between two good passes should say so, got:\n%s", noReason)
	}
}

// The #32 case as the user sees it: the confirmed orphan is a verdict and the
// once-seen duplication is reported alongside it, rather than being swallowed.
func TestPrintIntegrity_ConfirmedAndUnresolvedAreBothShown(t *testing.T) {
	out, clean := captureIntegrity(t, IntegrityReport{
		ActiveTasks: 2,
		Orphaned:    []string{"both"},
		Unresolved:  []string{"both"},
		Unconfirmed: true,
	})
	if clean {
		t.Error("a confirmed anomaly must return false")
	}
	if !strings.Contains(out, "orphaned tasks") {
		t.Errorf("expected the confirmed orphan line, got:\n%s", out)
	}
	if !strings.Contains(out, "seen in one pass") {
		t.Errorf("expected the once-seen line, got:\n%s", out)
	}
}

// twoPassServer serves one fixture on the first pass and another on the second.
//
// The pass flips when an endpoint is requested a second time, rather than on a
// nominated endpoint or a request count. Tying it to one endpoint is a trap: the
// switch happens when that endpoint is reached, so every endpoint served earlier
// in the same pass still answers from the previous fixture. The damage is then
// category-dependent — a pair of fixtures differing only in `active` breaks
// while pairs differing in `projects` or `archived` keep passing — which is a
// partially green table hiding exactly one category, the same shape of bug this
// file exists to catch. Counting requests instead would hardcode how many
// requests a pass makes. Detecting the repeat depends on neither.
func twoPassServer(t *testing.T, p1, p2 storeFixture) *httptest.Server {
	t.Helper()
	cur := p1
	served := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		enc := func(v []map[string]any) {
			if v == nil {
				v = []map[string]any{}
			}
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": v})
		}

		key := r.URL.Path + "?source=" + r.URL.Query().Get("source")
		if served[key] {
			// This endpoint was already answered, so a new pass has begun.
			cur = p2
			served = map[string]bool{}
		}
		served[key] = true

		switch {
		case r.URL.Path == "/tasks" && r.URL.Query().Get("source") == "archived":
			enc(cur.archived)
		case r.URL.Path == "/tasks":
			enc(cur.active)
		case r.URL.Path == "/projects":
			enc(cur.projects)
		case r.URL.Path == "/tags":
			enc(cur.tags)
		default:
			enc(nil)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Every category must be filtered by the confirmation pass, and the coverage has
// to be symmetric across them. It was not: deleting `orphaned` from
// categoryRefs left the whole suite green, while deleting `dangling` failed 13
// tests and `duplicated` failed 2. An unfiltered category does not come out
// empty — `confirmed` starts as a copy of the second pass, so it comes out
// holding that pass's findings, is treated as agreed, and swallows the once-seen
// observation. That reports one observation as confirmed and exits 3.
//
// Two constraints make these fixtures look fussier than they are:
//   - the once-seen anomaly must be in the SECOND pass. In the first, the
//     unfiltered copy of pass two would be empty anyway and the bug stays hidden.
//   - the first pass needs an anomaly of its own, or CheckIntegrityConfirmed
//     short-circuits on first.Clean() and never runs a second pass. Every case
//     below uses a persistent dangling reference to "ghost" for that.
func TestConfirmed_EachCategorySeenOnceIsUnresolved(t *testing.T) {
	for _, tc := range []struct {
		category string
		p1, p2   storeFixture
	}{
		{
			category: "dangling",
			p1: storeFixture{active: []map[string]any{task("t1")},
				projects: []map[string]any{project("t1", "ghost")}},
			p2: storeFixture{active: []map[string]any{task("t1")},
				projects: []map[string]any{project("t1", "ghost", "late")}},
		},
		{
			category: "orphaned",
			p1: storeFixture{active: []map[string]any{task("t1")},
				projects: []map[string]any{project("t1", "ghost")}},
			p2: storeFixture{active: []map[string]any{task("t1"), task("late")},
				projects: []map[string]any{project("t1", "ghost")}},
		},
		{
			category: "duplicated",
			p1: storeFixture{active: []map[string]any{task("t1"), task("late")},
				projects: []map[string]any{project("t1", "ghost", "late")}},
			p2: storeFixture{active: []map[string]any{task("t1"), task("late")},
				archived: []map[string]any{task("late")},
				projects: []map[string]any{project("t1", "ghost", "late")}},
		},
	} {
		t.Run(tc.category, func(t *testing.T) {
			srv := twoPassServer(t, tc.p1, tc.p2)
			r, err := CheckIntegrityConfirmed(context.Background(), bridge.NewClient(srv.URL))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// The fixture must actually have run two passes and found the
			// persistent anomaly, or nothing below means anything.
			if !containsID(r.Dangling, "ghost") {
				t.Fatalf("fixture did not produce the persistent anomaly, got %+v", r)
			}

			var confirmedForCategory []string
			for _, group := range r.byCategory() {
				if group.category == tc.category {
					confirmedForCategory = group.ids
				}
			}
			if containsID(confirmedForCategory, "late") {
				t.Errorf("%s: a once-seen observation must not be confirmed, got %v",
					tc.category, confirmedForCategory)
			}
			if !containsID(r.Unresolved, "late") {
				t.Errorf("%s: a once-seen observation must be recorded as unresolved, got %v",
					tc.category, r.Unresolved)
			}
			if !r.Unconfirmed {
				t.Errorf("%s: the passes disagreed, so Unconfirmed must be true", tc.category)
			}
		})
	}
}

// containsID reports whether an id list holds want. Distinct from doctor_test's
// contains, which is a substring check on a string.
func containsID(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// The render paths name each category by hand, which categoryRefs' comment
// discloses. This turns "remember to edit integrityJSON too" into a test
// failure: a category with no JSON key emits "clean": false with every list
// empty, which tells a consumer nothing about what was wrong.
func TestIntegrityJSON_HasAKeyForEveryCategory(t *testing.T) {
	report := IntegrityReport{}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(integrityJSON(report)), &decoded); err != nil {
		t.Fatalf("integrityJSON is not valid JSON: %v", err)
	}
	for _, group := range report.byCategory() {
		if _, ok := decoded[group.category]; !ok {
			t.Errorf("category %q has no key in the --json output; integrityJSON needs updating",
				group.category)
		}
	}
}
