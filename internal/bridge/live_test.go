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

// checkFields asserts that every field the bridge relies on is present with the
// type it expects. A required field missing from even one object is a failure:
// the code does not check before reading it.
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
				// A null in a nullable slot is not a type change.
				if got == "null" {
					continue
				}
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
		{name: "projectId", jsonType: "string"},
		{name: "tagIds", jsonType: "array"},
		{name: "subTaskIds", jsonType: "array"},
		{name: "timeSpent", jsonType: "number"},
		{name: "timeEstimate", jsonType: "number"},
		{name: "timeSpentOnDay", jsonType: "object"},
		{name: "created", jsonType: "number"},
		// Present only when set, which the bridge tolerates.
		{name: "notes", jsonType: "string", optional: true},
		{name: "parentId", jsonType: "string", optional: true},
		{name: "dueDay", jsonType: "string", optional: true},
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
	status, ok := client.Status(context.Background()).Data.(map[string]any)
	if !ok {
		t.Fatal("status did not return an object")
	}
	checkFields(t, []map[string]any{status}, "status", []fieldSpec{
		{name: "taskCount", jsonType: "number"},
		{name: "currentTaskId", jsonType: "string", optional: true},
	})

	health, ok := client.Health(context.Background()).Data.(map[string]any)
	if !ok {
		t.Fatal("health did not return an object")
	}
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
func TestLive_FixturesDoNotInventFields(t *testing.T) {
	client := liveClient(t)
	ctx := context.Background()

	cases := []struct {
		fixture string
		live    []map[string]any
	}{
		{"task-list-ok.json", objects(t, client.ListTasks(ctx,
			map[string]string{"source": "active", "includeDone": "true"}), "task")},
		{"project-list-ok.json", objects(t, client.ListProjects(ctx, nil), "project")},
		{"tag-list-ok.json", objects(t, client.ListTags(ctx, nil), "tag")},
		{"status-ok.json", []map[string]any{liveObject(t, client.Status(ctx), "status")}},
		{"health-ok.json", []map[string]any{liveObject(t, client.Health(ctx), "health")}},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			for _, obj := range fixtureObjects(t, tc.fixture) {
				compareToLive(t, obj, tc.live)
			}
		})
	}
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
	var asList []map[string]any
	if err := json.Unmarshal(raw, &asList); err == nil {
		return asList
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("parsing fixture: %v", err)
	}
	if envelope.Data == nil {
		t.Fatalf("fixture %s has no data object", name)
	}
	return []map[string]any{envelope.Data}
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
			t.Errorf("fixture claims field %q, which SP never returns — the offline tests are asserting fiction", k)
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
