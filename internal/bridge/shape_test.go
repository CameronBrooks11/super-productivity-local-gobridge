package bridge

import (
	"encoding/json"
	"testing"
)

func entity(id string, extra map[string]any) map[string]any {
	e := map[string]any{"id": id, "title": "t-" + id, "isDone": false}
	for k, v := range extra {
		e[k] = v
	}
	return e
}

func listResult(items ...map[string]any) Result {
	arr := make([]any, len(items))
	for i, it := range items {
		arr[i] = it
	}
	return Success(arr)
}

func dataList(t *testing.T, r Result) []any {
	t.Helper()
	arr, ok := r.Data.([]any)
	if !ok {
		t.Fatalf("expected a list, got %T", r.Data)
	}
	return arr
}

func TestShapeList_ProjectsToCompactFields(t *testing.T) {
	in := listResult(entity("a", map[string]any{
		"timeSpentOnDay": map[string]any{"2026-01-01": 1},
		"attachments":    []any{},
		"created":        float64(1),
		"issueId":        "gh-1",
	}))
	out := shapeList(in, compactTaskFields, listOptions{})
	obj := dataList(t, out)[0].(map[string]any)

	for _, want := range []string{"id", "title", "isDone"} {
		if _, ok := obj[want]; !ok {
			t.Errorf("compact output should keep %q", want)
		}
	}
	// The per-day map grows without bound over a task's life and nothing reads
	// it; issue fields are set on a small minority and meaningless alone.
	for _, drop := range []string{"timeSpentOnDay", "attachments", "created", "issueId"} {
		if _, ok := obj[drop]; ok {
			t.Errorf("compact output should drop %q", drop)
		}
	}
}

// Absent stays absent. SP distinguishes a missing key from a null one, and so
// does the archive guard and doctor --deep.
func TestShapeList_DoesNotInventNulls(t *testing.T) {
	out := shapeList(listResult(entity("a", nil)), compactTaskFields, listOptions{})
	obj := dataList(t, out)[0].(map[string]any)
	if _, ok := obj["notes"]; ok {
		t.Fatalf("a field SP did not send must not appear, got %v", obj)
	}
}

func TestShapeList_FullReturnsEverything(t *testing.T) {
	in := listResult(entity("a", map[string]any{"timeSpentOnDay": map[string]any{"d": 1}}))
	out := shapeList(in, compactTaskFields, listOptions{full: true})
	obj := dataList(t, out)[0].(map[string]any)
	if _, ok := obj["timeSpentOnDay"]; !ok {
		t.Fatal("full must not project")
	}
}

func TestShapeList_LimitAndOffset(t *testing.T) {
	in := listResult(entity("a", nil), entity("b", nil), entity("c", nil), entity("d", nil))

	cases := []struct {
		name string
		opts listOptions
		want []string
	}{
		{"no options returns all", listOptions{}, []string{"a", "b", "c", "d"}},
		{"limit truncates", listOptions{limit: 2}, []string{"a", "b"}},
		{"offset skips", listOptions{offset: 2}, []string{"c", "d"}},
		{"offset then limit", listOptions{offset: 1, limit: 2}, []string{"b", "c"}},
		{"limit beyond length", listOptions{limit: 99}, []string{"a", "b", "c", "d"}},
		{"offset beyond length", listOptions{offset: 99}, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dataList(t, shapeList(in, compactTaskFields, tc.opts))
			if len(got) != len(tc.want) {
				t.Fatalf("got %d items, want %d", len(got), len(tc.want))
			}
			for i, id := range tc.want {
				if got[i].(map[string]any)["id"] != id {
					t.Fatalf("position %d: got %v, want %s", i, got[i], id)
				}
			}
		})
	}
}

// An offset past the end must be an empty list, not null: a consumer indexing
// the result should not have to nil-check.
func TestShapeList_EmptyResultIsAListNotNull(t *testing.T) {
	out := shapeList(listResult(entity("a", nil)), compactTaskFields, listOptions{offset: 5})
	if out.Data == nil {
		t.Fatal("expected an empty list, got nil")
	}
	if len(dataList(t, out)) != 0 {
		t.Fatal("expected no items")
	}
}

// Errors and non-list payloads pass through untouched. Reshaping them would
// turn a failure into something that looks like a successful empty list.
func TestShapeList_PassesThroughNonLists(t *testing.T) {
	failure := Failure(ErrSPUnavailable, "down")
	if got := shapeList(failure, compactTaskFields, listOptions{limit: 1}); got.OK {
		t.Fatal("an error must not become a success")
	}
	obj := Success(map[string]any{"id": "x"})
	if got := shapeList(obj, compactTaskFields, listOptions{}); got.Data == nil {
		t.Fatal("a single object must survive")
	}
}

// A surprise in the response should be visible, not quietly dropped.
func TestShapeList_KeepsNonEntityItems(t *testing.T) {
	out := shapeList(Success([]any{"unexpected"}), compactTaskFields, listOptions{})
	if got := dataList(t, out); len(got) != 1 || got[0] != "unexpected" {
		t.Fatalf("non-entity items must pass through, got %v", got)
	}
}

func TestCompactFieldSets(t *testing.T) {
	// taskIds is the bulk of a project payload and a caller listing projects is
	// choosing between them, not enumerating contents.
	for _, f := range compactProjectFields {
		if f == "taskIds" || f == "backlogTaskIds" {
			t.Errorf("compact project output should not carry %q", f)
		}
	}
	if len(compactTagFields) == 0 || compactTagFields[0] != "id" {
		t.Error("tags need at least an id")
	}
}

// --- listOptions validation ---

func rawPayload(pairs map[string]string) map[string]json.RawMessage {
	p := make(map[string]json.RawMessage, len(pairs))
	for k, v := range pairs {
		p[k] = json.RawMessage(v)
	}
	return p
}

func TestValidateListOptions(t *testing.T) {
	cases := []struct {
		name    string
		payload map[string]string
		want    listOptions
		wantErr bool
	}{
		{"empty", nil, listOptions{}, false},
		{"limit", map[string]string{"limit": "20"}, listOptions{limit: 20}, false},
		{"offset", map[string]string{"offset": "5"}, listOptions{offset: 5}, false},
		{"full true", map[string]string{"full": "true"}, listOptions{full: true}, false},
		{"full false", map[string]string{"full": "false"}, listOptions{}, false},
		{"zero limit means no limit", map[string]string{"limit": "0"}, listOptions{}, false},
		{"negative limit", map[string]string{"limit": "-1"}, listOptions{}, true},
		{"limit as string", map[string]string{"limit": `"20"`}, listOptions{}, true},
		{"limit as float", map[string]string{"limit": "1.5"}, listOptions{}, true},
		{"full as string", map[string]string{"full": `"yes"`}, listOptions{}, true},
		{"limit above cap", map[string]string{"limit": "100001"}, listOptions{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, errResult := validateListOptions(rawPayload(tc.payload))
			if tc.wantErr {
				if errResult == nil {
					t.Fatalf("expected an error, got %+v", got)
				}
				if errResult.Error.Code != ErrInvalidInput {
					t.Fatalf("expected %s, got %s", ErrInvalidInput, errResult.Error.Code)
				}
				return
			}
			if errResult != nil {
				t.Fatalf("unexpected error: %s", errResult.Error.Message)
			}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// The shaping options are bridge-side. Forwarding them would be a lie about
// what was asked of the app, since SP ignores them entirely.
func TestValidateTaskListFilters_DoesNotForwardShapingOptions(t *testing.T) {
	params, errResult := validateTaskListFilters(rawPayload(map[string]string{
		"limit": "5", "offset": "1", "full": "true", "query": `"x"`,
	}))
	if errResult != nil {
		t.Fatalf("unexpected error: %s", errResult.Error.Message)
	}
	for _, forbidden := range []string{"limit", "offset", "full"} {
		if _, ok := params[forbidden]; ok {
			t.Errorf("%q must not be sent to SP, got params %v", forbidden, params)
		}
	}
	if params["query"] != "x" {
		t.Errorf("real filters must still be forwarded, got %v", params)
	}
}
