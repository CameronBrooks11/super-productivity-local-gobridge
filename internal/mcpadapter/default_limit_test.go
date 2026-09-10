package mcpadapter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/bridge"
)

// The default exists because omitting limit used to return every matching item,
// which on a real store is a tool result large enough that hosts start spilling
// it to disk or refusing it. These tests pin the three things that makes true:
// it applies to list operations, it never overrides a caller, and it never
// touches anything else.

func TestDefaultLimit_AppliedWhenOmitted(t *testing.T) {
	for _, op := range []string{bridge.OpTaskList, bridge.OpProjectList, bridge.OpTagList} {
		t.Run(op, func(t *testing.T) {
			got := applyDefaultLimit(op, map[string]json.RawMessage{})
			raw, ok := got["limit"]
			if !ok {
				t.Fatalf("no limit was supplied for %s", op)
			}
			if string(raw) != "20" {
				t.Errorf("limit = %s, want 20", raw)
			}
		})
	}
}

func TestDefaultLimit_AppliedToTheNoArgumentsCall(t *testing.T) {
	// A tool called with no arguments arrives as a nil map, which is exactly the
	// call this defends against — a model asking for "my tasks" and getting the
	// whole store.
	got := applyDefaultLimit(bridge.OpTaskList, nil)
	if got == nil {
		t.Fatal("a nil payload was left nil, so the default never applied")
	}
	if string(got["limit"]) != "20" {
		t.Errorf("limit = %s, want 20", got["limit"])
	}
}

func TestDefaultLimit_NeverOverridesTheCaller(t *testing.T) {
	// Including the opt-out: asking for MaxListLimit is how a caller says it
	// really does want everything, so it must survive untouched.
	for _, tc := range []struct{ name, limit string }{
		{"smaller", "5"},
		{"larger", "500"},
		{"the opt-out", "100000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := applyDefaultLimit(bridge.OpTaskList, map[string]json.RawMessage{
				"limit": json.RawMessage(tc.limit),
			})
			if string(got["limit"]) != tc.limit {
				t.Errorf("limit = %s, want the caller's %s", got["limit"], tc.limit)
			}
		})
	}
}

func TestDefaultLimit_LeavesNonListOperationsAlone(t *testing.T) {
	// task.create takes a title, not a limit. Injecting one would turn a valid
	// create into an invalid-input error.
	for _, op := range []string{bridge.OpTaskCreate, bridge.OpTaskGet, bridge.OpStatusGet, bridge.OpBridgeHealth} {
		t.Run(op, func(t *testing.T) {
			payload := map[string]json.RawMessage{"title": json.RawMessage(`"x"`)}
			got := applyDefaultLimit(op, payload)
			if _, ok := got["limit"]; ok {
				t.Errorf("a limit was injected into %s", op)
			}
		})
	}
}

func TestDefaultLimit_LeavesOtherFiltersIntact(t *testing.T) {
	payload := map[string]json.RawMessage{
		"query":       json.RawMessage(`"report"`),
		"includeDone": json.RawMessage("true"),
	}
	got := applyDefaultLimit(bridge.OpTaskList, payload)
	if string(got["query"]) != `"report"` || string(got["includeDone"]) != "true" {
		t.Errorf("the caller's filters were altered: %v", got)
	}
	if string(got["limit"]) != "20" {
		t.Errorf("limit = %s, want 20", got["limit"])
	}
}

func TestDefaultLimit_IsActuallyWiredIntoTheHandler(t *testing.T) {
	// The helper being correct proves nothing if nothing calls it. This drives a
	// real tools/call through the server against a stub SP returning 50 tasks,
	// and asserts what comes back is capped — the one assertion that fails if
	// the call site is ever removed.
	tasks := make([]map[string]any, 50)
	for i := range tasks {
		tasks[i] = map[string]any{
			"id":    fmt.Sprintf("t%d", i),
			"title": fmt.Sprintf("Task %d", i),
		}
	}
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": tasks})
	})

	resp := sendRPC(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_tasks","arguments":{}}}`)
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result in response: %v", resp)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("no content in result: %v", result)
	}
	first := content[0].(map[string]any)
	var got []any
	if err := json.Unmarshal([]byte(first["text"].(string)), &got); err != nil {
		t.Fatalf("first content block was not a JSON list: %v", err)
	}
	if len(got) != DefaultListLimit {
		t.Errorf("got %d tasks, want %d — the default is not wired into the tool call path", len(got), DefaultListLimit)
	}
	// And the model must be told, or a capped list reads as the whole store.
	if len(content) < 2 {
		t.Fatal("no truncation note accompanied the capped list")
	}
	note := content[1].(map[string]any)["text"].(string)
	if !strings.Contains(note, "truncated") {
		t.Errorf("second content block is not a truncation note: %q", note)
	}
}

func TestDefaultLimit_IsTheValueTheDescriptionsPromise(t *testing.T) {
	// The limit descriptions tell the model the default is 20. A default that
	// disagreed with them would be worse than none: the model would plan against
	// a number the server does not honour.
	s := NewServer(nil)
	seen := 0
	for _, tool := range s.tools {
		if !listOperations[s.toolMap[tool.Name]] {
			continue
		}
		seen++
		props, ok := tool.InputSchema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s has no schema properties", tool.Name)
		}
		limit, ok := props["limit"].(map[string]any)
		if !ok {
			t.Fatalf("%s exposes no limit parameter", tool.Name)
		}
		desc, _ := limit["description"].(string)
		if !strings.Contains(desc, "Defaults to 20") {
			t.Errorf("%s limit description does not state the default: %q", tool.Name, desc)
		}
	}
	if seen != 3 {
		t.Fatalf("checked %d list tools, want 3", seen)
	}
}
