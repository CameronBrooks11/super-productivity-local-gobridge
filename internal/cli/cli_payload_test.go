package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// captureCreatePayload starts an httptest server that captures the POST /tasks body
// and returns a successful response. Returns the server and a pointer to the captured body.
func captureCreatePayload(t *testing.T) (*httptest.Server, *map[string]any) {
	t.Helper()
	var captured map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/tasks" {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}
			if err := json.Unmarshal(body, &captured); err != nil {
				t.Fatalf("failed to unmarshal request body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"test-id-123","title":"test"}`))
			return
		}
		// Default: return empty success for any other endpoint
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	return ts, &captured
}

func TestTasksAdd_PayloadContainsProjectID(t *testing.T) {
	ts, captured := captureCreatePayload(t)
	defer ts.Close()
	t.Setenv("SP_BASE_URL", ts.URL)

	code := Run([]string{"tasks", "add", "My Task", "--project-id", "proj-abc123"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if (*captured)["title"] != "My Task" {
		t.Errorf("expected title 'My Task', got %v", (*captured)["title"])
	}
	if (*captured)["projectId"] != "proj-abc123" {
		t.Errorf("expected projectId 'proj-abc123', got %v", (*captured)["projectId"])
	}
}

func TestTasksAdd_PayloadContainsTagIDs(t *testing.T) {
	ts, captured := captureCreatePayload(t)
	defer ts.Close()
	t.Setenv("SP_BASE_URL", ts.URL)

	code := Run([]string{"tasks", "add", "Tagged Task", "--tag-id", "tag-xyz"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if (*captured)["title"] != "Tagged Task" {
		t.Errorf("expected title 'Tagged Task', got %v", (*captured)["title"])
	}
	tagIds, ok := (*captured)["tagIds"].([]any)
	if !ok {
		t.Fatalf("expected tagIds to be array, got %T: %v", (*captured)["tagIds"], (*captured)["tagIds"])
	}
	if len(tagIds) != 1 || tagIds[0] != "tag-xyz" {
		t.Errorf("expected tagIds=['tag-xyz'], got %v", tagIds)
	}
}

func TestTasksAdd_PayloadContainsAllFields(t *testing.T) {
	ts, captured := captureCreatePayload(t)
	defer ts.Close()
	t.Setenv("SP_BASE_URL", ts.URL)

	code := Run([]string{"tasks", "add", "Full Task",
		"--project-id", "proj-1",
		"--tag-id", "tag-2",
		"--notes", "Some notes here",
		"--due-day", "2026-07-01",
		"--time-estimate", "3600000",
	})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	payload := *captured
	if payload["title"] != "Full Task" {
		t.Errorf("title: got %v", payload["title"])
	}
	if payload["projectId"] != "proj-1" {
		t.Errorf("projectId: got %v", payload["projectId"])
	}
	tagIds, ok := payload["tagIds"].([]any)
	if !ok || len(tagIds) != 1 || tagIds[0] != "tag-2" {
		t.Errorf("tagIds: got %v", payload["tagIds"])
	}
	if payload["notes"] != "Some notes here" {
		t.Errorf("notes: got %v", payload["notes"])
	}
	if payload["dueDay"] != "2026-07-01" {
		t.Errorf("dueDay: got %v", payload["dueDay"])
	}
	// timeEstimate is sent as a raw number
	te, ok := payload["timeEstimate"].(float64)
	if !ok || te != 3600000 {
		t.Errorf("timeEstimate: got %v (%T)", payload["timeEstimate"], payload["timeEstimate"])
	}
}

func TestTasksAdd_TitleNotPollutedByFlags(t *testing.T) {
	ts, captured := captureCreatePayload(t)
	defer ts.Close()
	t.Setenv("SP_BASE_URL", ts.URL)

	code := Run([]string{"tasks", "add", "Clean Title", "--project-id", "proj-1", "--notes", "My notes"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	title := (*captured)["title"]
	if title != "Clean Title" {
		t.Errorf("expected title 'Clean Title', got '%v' — flags leaked into title", title)
	}
}
