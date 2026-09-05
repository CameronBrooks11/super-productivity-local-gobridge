//go:build live

// Package-level note: these tests talk to a running Super Productivity.
//
// They are excluded from `go test ./...` by the `live` build tag and are not
// run in CI, which has no SP and could not reach one — the API port is a
// hardcoded 3876. Run them locally before a release:
//
//	make test-live
//
// Every request here is a GET. Per AGENTS.md, writes must never be aimed at a
// real store; if write coverage is added later it belongs behind a second tag
// and pointed at a throwaway `--user-data-dir` profile.
package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// fieldSpec describes one field the bridge depends on.
type fieldSpec struct {
	name     string
	jsonType string // "string", "number", "bool", "array", "object"
	optional bool   // absent on some objects, which is legal
	nullable bool   // SP may send an explicit null here
}

// jsonType names the decoded type of a JSON value the way a response describes
// it, so a mismatch reads as "string became number" rather than a Go type.
func jsonType(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case float64, json.Number:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", v)
	}
}

func liveClient(t *testing.T) *Client {
	t.Helper()
	base := DefaultBaseURL
	if env := os.Getenv("SP_BASE_URL"); env != "" {
		base = env
	}
	c := NewClient(base)
	if h := c.Health(context.Background()); !h.OK {
		t.Skipf("Super Productivity is not reachable at %s (%s); start it with the Local REST API enabled", base, h.Error.Message)
	}
	return c
}

func objects(t *testing.T, res Result, label string) []map[string]any {
	t.Helper()
	if !res.OK {
		t.Fatalf("%s: %s", label, res.Error.Message)
	}
	arr, ok := res.Data.([]any)
	if !ok {
		t.Fatalf("%s: expected a list, got %s", label, jsonType(res.Data))
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("%s: expected objects, got %s", label, jsonType(item))
		}
		out = append(out, obj)
	}
	if len(out) == 0 {
		t.Skipf("%s: the store has none, so there is nothing to check", label)
	}
	return out
}

// checkFields asserts that the fields named here are present with the type
// expected. A required field missing from even one object is a failure.
//
// These are not all read by production code — doctor --deep reads subTaskIds,
// taskIds and backlogTaskIds off a response; most of the rest are passed
// through to callers. They are checked because a caller, human or agent, is
// relying on them, and a silent rename would reach the user rather than the
// bridge.
func checkFields(t *testing.T, items []map[string]any, label string, specs []fieldSpec) {
	t.Helper()
	for _, spec := range specs {
		missing, wrong := 0, map[string]int{}
		for _, item := range items {
			v, present := item[spec.name]
			if !present {
				missing++
				continue
			}
			if got := jsonType(v); got != spec.jsonType {
				if got == "null" && spec.nullable {
					continue
				}
				// Otherwise a null is a real mismatch. Exempting it blindly hid
				// the drift that matters most: doctor --deep reads taskIds and
				// subTaskIds without checking, so "taskIds": null breaks it
				// while a blanket exemption would report the field as fine.
				wrong[got]++
			}
		}
		if missing > 0 && !spec.optional {
			t.Errorf("%s.%s: required by the bridge but missing from %d/%d objects",
				label, spec.name, missing, len(items))
		}
		if len(wrong) > 0 {
			t.Errorf("%s.%s: expected %s, saw %v across %d objects",
				label, spec.name, spec.jsonType, wrong, len(items))
		}
	}
}

func TestLive_TaskFields(t *testing.T) {
	client := liveClient(t)
	specs := []fieldSpec{
		{name: "id", jsonType: "string"},
		{name: "title", jsonType: "string"},
		{name: "isDone", jsonType: "bool"},
		// On the store this was captured against SP encodes "unset" as an absent
		// key, never an explicit null. But validate.go accepts projectId: null to
		// clear a task's project, so a null is plausible on another store or
		// version — and a false failure on a healthy store is the worse error
		// for a suite whose value is being trusted.
		{name: "projectId", jsonType: "string", nullable: true},
		{name: "tagIds", jsonType: "array"},
		{name: "subTaskIds", jsonType: "array"},
		{name: "timeSpent", jsonType: "number"},
		{name: "timeEstimate", jsonType: "number"},
		{name: "timeSpentOnDay", jsonType: "object"},
		{name: "created", jsonType: "number"},
		// Present only when set, which the bridge tolerates.
		{name: "notes", jsonType: "string", optional: true, nullable: true},
		{name: "parentId", jsonType: "string", optional: true, nullable: true},
		{name: "dueDay", jsonType: "string", optional: true, nullable: true},
	}
	for _, source := range []string{"active", "archived"} {
		t.Run(source, func(t *testing.T) {
			res := client.ListTasks(context.Background(), map[string]string{
				"source": source, "includeDone": "true",
			})
			checkFields(t, objects(t, res, "task/"+source), "task/"+source, specs)
		})
	}
}

func TestLive_ProjectFields(t *testing.T) {
	client := liveClient(t)
	// taskIds and backlogTaskIds are what doctor --deep cross-references; a
	// rename there is the failure that reported a healthy store as corrupt.
	checkFields(t, objects(t, client.ListProjects(context.Background(), nil), "project"), "project", []fieldSpec{
		{name: "id", jsonType: "string"},
		{name: "title", jsonType: "string"},
		{name: "taskIds", jsonType: "array"},
		{name: "backlogTaskIds", jsonType: "array"},
	})
}

func TestLive_TagFields(t *testing.T) {
	client := liveClient(t)
	checkFields(t, objects(t, client.ListTags(context.Background(), nil), "tag"), "tag", []fieldSpec{
		{name: "id", jsonType: "string"},
		{name: "title", jsonType: "string"},
		{name: "taskIds", jsonType: "array"},
	})
}

func TestLive_StatusAndHealthFields(t *testing.T) {
	client := liveClient(t)
	status := liveObject(t, client.Status(context.Background()), "status")
	// currentTaskId is always present and null when nothing is tracked, so it is
	// required-and-nullable. Marking it optional as well made the spec vacuous:
	// absence was excused by one flag and null by the other, leaving nothing
	// that could fail if SP dropped or renamed it.
	checkFields(t, []map[string]any{status}, "status", []fieldSpec{
		{name: "taskCount", jsonType: "number"},
		{name: "currentTaskId", jsonType: "string", nullable: true},
		{name: "currentTask", jsonType: "object", nullable: true},
	})

	health := liveObject(t, client.Health(context.Background()), "health")
	checkFields(t, []map[string]any{health}, "health", []fieldSpec{
		{name: "server", jsonType: "string"},
		{name: "rendererReady", jsonType: "bool"},
	})
}

// Both a missing task and a missing route answer 404, and the bridge relies on
// the body to tell them apart. If SP ever stops distinguishing them, the
// archive existence guard silently changes meaning.
func TestLive_NotFoundCodesAreDistinct(t *testing.T) {
	client := liveClient(t)
	task := client.GetTask(context.Background(), "SP_BRIDGE_LIVE_TEST_NO_SUCH_TASK")
	if task.OK {
		t.Fatal("an invented task id must not resolve")
	}
	if task.Error.Code != ErrTaskNotFound {
		t.Errorf("a missing task should report %s, got %s (%s)", ErrTaskNotFound, task.Error.Code, task.Error.Message)
	}

	route := client.request(context.Background(), "GET", "/sp-bridge-live-test-no-such-route", nil, nil)
	if route.OK {
		t.Fatal("an invented route must not resolve")
	}
	if route.Error.Code != ErrNotFound {
		t.Errorf("a missing route should report %s, got %s (%s)", ErrNotFound, route.Error.Code, route.Error.Message)
	}
}

// --- fixtures ---
//
// The fixtures in testdata/ are replayed by the offline tests, so if they
// describe a response SP does not send, those tests pass while the client is
// wrong. That has happened: the tag fixture used "name" where SP sends "title".
//
// This checks the fixtures against reality in the one direction that matters —
// every field a fixture claims must exist live, with the same type. It does not
// require fixtures to be exhaustive; they are deliberately small.
//
// There is no --update mode on purpose. Fixtures are committed to a public
// repository and live responses carry real task titles, project names and
// notes, so they are written by hand from the shapes this test reports rather
// than copied from a store.
// knownOptional names fields SP returns only when the user has set them. On a
// store where none are set they are absent everywhere, and requiring every
// fixture field to appear live would then report them as fiction — with the
// documented remedy being to delete a real field from the fixture.
//
// Listing one here is a claim that SP has the field, checked whenever a store
// does contain it: presence is excused, type is not.
var knownOptional = map[string]bool{
	"notes":            true,
	"parentId":         true,
	"dueDay":           true,
	"doneOn":           true,
	"modified":         true,
	"subTasks":         true,
	"issueId":          true,
	"issueType":        true,
	"issueProviderId":  true,
	"issueLastUpdated": true,
}

func TestLive_FixturesDoNotInventFields(t *testing.T) {
	client := liveClient(t)
	ctx := context.Background()

	// Each pool is fetched inside its own subtest. Fetching them all up front
	// meant one empty collection — a store with no tags — skipped every other
	// fixture check and still exited 0, so the guard against fiction silently
	// did nothing.
	// Each fetch takes the subtest's own *testing.T. Passing the parent's would
	// route a skip or failure to the parent, which is how one empty collection
	// came to swallow every other check.
	cases := []struct {
		fixture string
		live    func(t *testing.T) []map[string]any
	}{
		{"task-list-ok.json", func(t *testing.T) []map[string]any { return liveTasks(t, client) }},
		{"task-create-ok.json", func(t *testing.T) []map[string]any { return liveTasks(t, client) }},
		{"task-update-ok.json", func(t *testing.T) []map[string]any { return liveTasks(t, client) }},
		{"project-list-ok.json", func(t *testing.T) []map[string]any {
			return objects(t, client.ListProjects(ctx, nil), "project")
		}},
		{"tag-list-ok.json", func(t *testing.T) []map[string]any {
			return objects(t, client.ListTags(ctx, nil), "tag")
		}},
		{"status-ok.json", func(t *testing.T) []map[string]any {
			return []map[string]any{liveObject(t, client.Status(ctx), "status")}
		}},
		{"health-ok.json", func(t *testing.T) []map[string]any {
			return []map[string]any{liveObject(t, client.Health(ctx), "health")}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			live := tc.live(t)
			for _, obj := range requireObjects(t, tc.fixture, fixtureObjects(t, tc.fixture)) {
				compareToLive(t, obj, live)
			}
		})
	}
}

// liveTasks unions both pools. Some fields only ever appear on archived tasks
// (subTasks) and some only on active ones, so checking a fixture against one
// pool alone invents drift that is not there.
func liveTasks(t *testing.T, client *Client) []map[string]any {
	t.Helper()
	ctx := context.Background()
	var all []map[string]any
	for _, source := range []string{"active", "archived"} {
		res := client.ListTasks(ctx, map[string]string{"source": source, "includeDone": "true"})
		if !res.OK {
			t.Fatalf("task/%s: %s", source, res.Error.Message)
		}
		arr, ok := res.Data.([]any)
		if !ok {
			t.Fatalf("task/%s: expected a list, got %s", source, jsonType(res.Data))
		}
		for _, item := range arr {
			obj, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("task/%s: expected objects, got %s", source, jsonType(item))
			}
			all = append(all, obj)
		}
	}
	// Skip only if the union is empty. Skipping on an empty active pool alone
	// meant a store whose tasks had all been archived checked nothing, which is
	// the same silent no-op this suite exists to prevent.
	if len(all) == 0 {
		t.Skip("the store has no tasks in either pool, so there is nothing to check")
	}
	return all
}

// liveObject unwraps a single-object response.
func liveObject(t *testing.T, res Result, label string) map[string]any {
	t.Helper()
	if !res.OK {
		t.Fatalf("%s: %s", label, res.Error.Message)
	}
	obj, ok := res.Data.(map[string]any)
	if !ok {
		t.Fatalf("%s: expected an object, got %s", label, jsonType(res.Data))
	}
	return obj
}

// fixtureObjects reads a fixture, accepting both shapes committed in testdata:
// a bare array of objects, and an envelope whose data is one object.
func fixtureObjects(t *testing.T, name string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	body := raw
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Data != nil {
		body = envelope.Data
	}
	// data may be a list or a single object; both are real SP shapes.
	var asList []map[string]any
	if err := json.Unmarshal(body, &asList); err == nil {
		return asList
	}
	var one map[string]any
	if err := json.Unmarshal(body, &one); err != nil {
		t.Fatalf("parsing fixture %s: %v", name, err)
	}
	return []map[string]any{one}
}

// requireObjects fails when a fixture decodes to nothing. `null` and `[]` both
// unmarshal cleanly into an empty slice, so the fixture would be reported as
// covered while asserting nothing.
func requireObjects(t *testing.T, name string, objs []map[string]any) []map[string]any {
	t.Helper()
	if len(objs) == 0 {
		t.Fatalf("fixture %s decoded to no objects, so it verifies nothing", name)
	}
	return objs
}

// compareToLive fails for any field the fixture claims that SP does not return,
// or returns with a different type. It does not require fixtures to be
// exhaustive — they are deliberately small — only that they are not fiction.
func compareToLive(t *testing.T, fixture map[string]any, live []map[string]any) {
	t.Helper()
	// Union of types seen live, so a field present on only some objects still
	// counts as real.
	liveTypes := map[string]map[string]bool{}
	for _, obj := range live {
		for k, v := range obj {
			if liveTypes[k] == nil {
				liveTypes[k] = map[string]bool{}
			}
			liveTypes[k][jsonType(v)] = true
		}
	}
	for k, v := range fixture {
		types, exists := liveTypes[k]
		if !exists {
			if knownOptional[k] {
				t.Logf("field %q is not set anywhere in this store, so its type could not be checked", k)
				continue
			}
			t.Errorf("fixture claims field %q, which SP never returns — the offline tests are asserting fiction", k)
			continue
		}
		// SP encodes "unset" two ways — an absent key, and an explicit null —
		// and only the first was handled. A field present but null everywhere
		// reached the type comparison and was reported as fiction, whose
		// documented remedy is to delete a real field from the fixture.
		if len(types) == 1 && types["null"] {
			t.Logf("field %q is null everywhere in this store, so its type could not be checked", k)
			continue
		}
		got := jsonType(v)
		if got == "null" {
			continue // a null placeholder asserts nothing about type
		}
		if !types[got] {
			t.Errorf("fixture has %q as %s; SP returns it as %s", k, got, strings.Join(sortedTypeNames(types), "|"))
		}
	}
}

func sortedTypeNames(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// The suite skips rather than fails when SP is unreachable or a collection is
// empty, so a run that asserted nothing still exits 0 and reads as "verified"
// before a release. This makes that state loud.
func TestLive_StoreHasSomethingToCheck(t *testing.T) {
	// Deliberately not liveClient: that skips when SP is unreachable, which is
	// the louder half of what this test exists to report. Skipping there would
	// have left `go test -tags live` exiting 0 having asserted nothing.
	base := DefaultBaseURL
	if env := os.Getenv("SP_BASE_URL"); env != "" {
		base = env
	}
	client := NewClient(base)
	ctx := context.Background()

	if h := client.Health(ctx); !h.OK {
		t.Fatalf("Super Productivity is not reachable at %s (%s), so this run verified nothing",
			base, h.Error.Message)
	}
	// Report a failed call as a failed call. Reading Data through a discarded
	// error turned "SP returned an error" into "your store is empty", which
	// sends the reader to the wrong problem.
	tasksRes := client.ListTasks(ctx, map[string]string{"source": "all", "includeDone": "true"})
	if !tasksRes.OK {
		t.Fatalf("listing tasks failed: %s", tasksRes.Error.Message)
	}
	projectsRes := client.ListProjects(ctx, nil)
	if !projectsRes.OK {
		t.Fatalf("listing projects failed: %s", projectsRes.Error.Message)
	}
	tasks, _ := tasksRes.Data.([]any)
	projects, _ := projectsRes.Data.([]any)
	if len(tasks) == 0 && len(projects) == 0 {
		t.Fatal("the store has no tasks and no projects, so this run verified nothing; " +
			"point SP_BASE_URL at a populated store before treating a pass as meaningful")
	}
}
