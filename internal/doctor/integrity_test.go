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
		projects: []map[string]any{withIDs("taskIds", "t1", "GHOST")},
	}, "doctor", "--json")
	if code != 3 {
		t.Fatalf("inconsistent store via --json must exit 3, got %d", code)
	}
}

func TestRun_JSONReturnsZeroWhenClean(t *testing.T) {
	code := runWithStore(t, storeFixture{
		active:   []map[string]any{task("t1")},
		projects: []map[string]any{withIDs("taskIds", "t1")},
	}, "doctor", "--json")
	if code != 0 {
		t.Fatalf("clean store via --json must exit 0, got %d", code)
	}
}

func TestRun_JSONReturnsOneWhenUnreachable(t *testing.T) {
	// Reserved-for-documentation port that nothing listens on.
	t.Setenv("SP_BASE_URL", "http://127.0.0.1:1")
	if code := Run([]string{"doctor", "--json"}); code != 1 {
		t.Fatalf("unreachable SP must exit 1, not be confused with 3; got %d", code)
	}
}

// A mistyped flag used to be swallowed, running a shallow check that then
// reported "All checks passed" — the user believed the integrity check ran.
func TestRun_RejectsStrayPositional(t *testing.T) {
	if code := Run([]string{"doctor", "deep"}); code != 2 {
		t.Fatalf("stray positional must exit 2, got %d", code)
	}
}

func TestRun_RejectsUnknownFlag(t *testing.T) {
	if code := Run([]string{"doctor", "--nope"}); code != 2 {
		t.Fatalf("unknown flag must exit 2, got %d", code)
	}
}

func TestRun_HelpExitsZero(t *testing.T) {
	if code := Run([]string{"doctor", "--help"}); code != 0 {
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
		projects: []map[string]any{withIDs("taskIds", "t1", "both")},
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
				enc([]map[string]any{withIDs("taskIds", "t1", "RACE")}) // transient
				return
			}
			enc([]map[string]any{withIDs("taskIds", "t1")})
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
	if !r.Clean() {
		t.Fatalf("a transient anomaly must not be reported: %+v", r)
	}
	if r.Transient != 1 {
		t.Fatalf("expected 1 transient anomaly recorded, got %d", r.Transient)
	}
}

func TestConfirmed_PersistentAnomalyIsKept(t *testing.T) {
	srv := newStoreServer(t, storeFixture{
		active:   []map[string]any{task("t1")},
		projects: []map[string]any{withIDs("taskIds", "t1", "GHOST")},
	})
	defer srv.Close()
	r, err := CheckIntegrityConfirmed(context.Background(), bridge.NewClient(srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Dangling) != 1 || r.Dangling[0] != "GHOST" {
		t.Fatalf("a persistent anomaly must survive confirmation, got %v", r.Dangling)
	}
	if r.Transient != 0 {
		t.Fatalf("nothing was transient here, got %d", r.Transient)
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
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": []map[string]any{withIDs("taskIds", "t1")}})
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
		"transient", "unconfirmed", "clean",
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
			enc([]map[string]any{withIDs("taskIds", "t1", ghost)})
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
	if !r.Clean() {
		t.Fatal("precondition: the intersection here is empty")
	}
}

func TestRun_ShiftingAnomaliesExitOneNotZero(t *testing.T) {
	srv := shiftingServer(t)
	t.Setenv("SP_BASE_URL", srv.URL)
	if code := Run([]string{"doctor", "--json"}); code != 1 {
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
			enc([]map[string]any{withIDs("taskIds", "t1", "GHOST")})
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
	t.Setenv("SP_BASE_URL", srv.URL)
	// Fresh rounds for the Run() call; the anomaly persists, the confirmation fails.
	if code := Run([]string{"doctor", "--json"}); code == 3 {
		t.Fatal("an unconfirmed report must not claim exit 3")
	}
}

// --json documents stdout as a JSON stream, so help must not land there.
func TestRun_HelpWithJSONKeepsStdoutClean(t *testing.T) {
	if code := Run([]string{"doctor", "--json", "--help"}); code != 0 {
		t.Fatalf("--help must exit 0, got %d", code)
	}
}

// The subcommand word is only legal as args[0].
func TestRun_StrayDoctorTokenRejected(t *testing.T) {
	if code := Run([]string{"doctor", "--deep", "doctor"}); code != 2 {
		t.Fatalf("a repeated subcommand word is a typo; want exit 2, got %d", code)
	}
}
