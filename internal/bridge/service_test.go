package bridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "fixtures", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func mockServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	ts := httptest.NewServer(handler)
	client := NewClient(ts.URL)
	return ts, client
}

func TestClient_Health_OK(t *testing.T) {
	fixture := loadFixture(t, "health-ok.json")
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	})
	defer ts.Close()

	result := client.Health(context.Background())
	if !result.OK {
		t.Fatalf("expected OK, got error: %+v", result.Error)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result.Data)
	}
	if data["status"] != "up" {
		t.Fatalf("expected status=up, got %v", data["status"])
	}
}

func TestClient_Status_OK(t *testing.T) {
	fixture := loadFixture(t, "status-ok.json")
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	})
	defer ts.Close()

	result := client.Status(context.Background())
	if !result.OK {
		t.Fatalf("expected OK, got error: %+v", result.Error)
	}
}

func TestClient_ListTasks_OK(t *testing.T) {
	fixture := loadFixture(t, "task-list-ok.json")
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	})
	defer ts.Close()

	result := client.ListTasks(context.Background(), nil)
	if !result.OK {
		t.Fatalf("expected OK, got error: %+v", result.Error)
	}
	arr, ok := result.Data.([]any)
	if !ok {
		t.Fatalf("expected array, got %T", result.Data)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(arr))
	}
}

func TestClient_ListTasks_WithParams(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("projectId") != "proj-1" {
			t.Fatalf("expected projectId=proj-1, got %s", r.URL.Query().Get("projectId"))
		}
		if r.URL.Query().Get("includeDone") != "true" {
			t.Fatalf("expected includeDone=true, got %s", r.URL.Query().Get("includeDone"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	})
	defer ts.Close()

	result := client.ListTasks(context.Background(), map[string]string{
		"projectId":   "proj-1",
		"includeDone": "true",
	})
	if !result.OK {
		t.Fatalf("expected OK, got error: %+v", result.Error)
	}
}

func TestClient_GetTask_NotFound(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	defer ts.Close()

	result := client.GetTask(context.Background(), "nonexistent")
	if result.OK {
		t.Fatal("expected error")
	}
	if result.Error.Code != ErrTaskNotFound {
		t.Fatalf("expected TASK_NOT_FOUND, got %s", result.Error.Code)
	}
}

func TestClient_CreateTask_OK(t *testing.T) {
	fixture := loadFixture(t, "task-create-ok.json")
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/tasks" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	})
	defer ts.Close()

	result := client.CreateTask(context.Background(), map[string]any{
		"title": "Write integration tests",
	})
	if !result.OK {
		t.Fatalf("expected OK, got error: %+v", result.Error)
	}
}

func TestClient_CreateTask_Error(t *testing.T) {
	fixture := loadFixture(t, "task-create-error.json")
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write(fixture)
	})
	defer ts.Close()

	result := client.CreateTask(context.Background(), map[string]any{"title": "x"})
	if result.OK {
		t.Fatal("expected error")
	}
	if result.Error.Code != "INVALID_REQUEST_BODY" {
		t.Fatalf("expected INVALID_REQUEST_BODY, got %s", result.Error.Code)
	}
}

func TestClient_UpdateTask_OK(t *testing.T) {
	fixture := loadFixture(t, "task-update-ok.json")
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	})
	defer ts.Close()

	result := client.UpdateTask(context.Background(), "task-new789", map[string]any{"title": "updated"})
	if !result.OK {
		t.Fatalf("expected OK, got error: %+v", result.Error)
	}
}

func TestClient_ListProjects_OK(t *testing.T) {
	fixture := loadFixture(t, "project-list-ok.json")
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	})
	defer ts.Close()

	result := client.ListProjects(context.Background(), nil)
	if !result.OK {
		t.Fatalf("expected OK, got error: %+v", result.Error)
	}
	arr := result.Data.([]any)
	if len(arr) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(arr))
	}
}

func TestClient_ListTags_OK(t *testing.T) {
	fixture := loadFixture(t, "tag-list-ok.json")
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	})
	defer ts.Close()

	result := client.ListTags(context.Background(), nil)
	if !result.OK {
		t.Fatalf("expected OK, got error: %+v", result.Error)
	}
}

func TestClient_ConnectionRefused(t *testing.T) {
	// Use a port that's not listening
	client := NewClient("http://127.0.0.1:1")
	result := client.Health(context.Background())
	if result.OK {
		t.Fatal("expected error")
	}
	if result.Error.Code != ErrSPUnavailable {
		t.Fatalf("expected SP_UNAVAILABLE, got %s", result.Error.Code)
	}
}

func TestClient_AppNotReady(t *testing.T) {
	fixture := loadFixture(t, "app-not-ready-error.json")
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		w.Write(fixture)
	})
	defer ts.Close()

	result := client.Health(context.Background())
	if result.OK {
		t.Fatal("expected error")
	}
	if result.Error.Code != "APP_NOT_READY" {
		t.Fatalf("expected APP_NOT_READY, got %s", result.Error.Code)
	}
}

// Service-level tests

func TestService_UnknownOperation(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	service := NewService(client)
	result := service.Execute(context.Background(), Request{
		Operation: "nonexistent.op",
	})
	if result.OK {
		t.Fatal("expected error")
	}
	if result.Error.Code != ErrUnknownOperation {
		t.Fatalf("expected UNKNOWN_OPERATION, got %s", result.Error.Code)
	}
}

func TestService_TaskCreate_Validation_MissingTitle(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	service := NewService(client)
	result := service.Execute(context.Background(), Request{
		Operation: OpTaskCreate,
		Payload:   payload("notes", `"some notes"`),
	})
	if result.OK {
		t.Fatal("expected error")
	}
	if result.Error.Code != ErrInvalidInput {
		t.Fatalf("expected INVALID_INPUT, got %s", result.Error.Code)
	}
}

func TestService_TaskCreate_Validation_IDNotAllowed(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	service := NewService(client)
	result := service.Execute(context.Background(), Request{
		Operation: OpTaskCreate,
		Payload:   payload("title", `"test"`, "id", `"my-id"`),
	})
	if result.OK {
		t.Fatal("expected error")
	}
	if result.Error.Code != ErrInvalidInput {
		t.Fatalf("expected INVALID_INPUT, got %s", result.Error.Code)
	}
}

func TestService_TaskCreate_Validation_ParentIdWithProjectId(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	service := NewService(client)
	result := service.Execute(context.Background(), Request{
		Operation: OpTaskCreate,
		Payload:   payload("title", `"test"`, "parentId", `"p1"`, "projectId", `"proj"`),
	})
	if result.OK {
		t.Fatal("expected error")
	}
	if result.Error.Code != ErrInvalidInput {
		t.Fatalf("expected INVALID_INPUT, got %s", result.Error.Code)
	}
}

func TestService_TaskCreate_Validation_ParentIdWithTagIds(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	service := NewService(client)
	result := service.Execute(context.Background(), Request{
		Operation: OpTaskCreate,
		Payload:   payload("title", `"test"`, "parentId", `"p1"`, "tagIds", `["t1"]`),
	})
	if result.OK {
		t.Fatal("expected error")
	}
	if result.Error.Code != ErrInvalidInput {
		t.Fatalf("expected INVALID_INPUT, got %s", result.Error.Code)
	}
}

func TestService_TaskUpdate_Validation_NoFields(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	service := NewService(client)
	result := service.Execute(context.Background(), Request{
		Operation: OpTaskUpdate,
		Payload:   payload("id", `"task-1"`),
	})
	if result.OK {
		t.Fatal("expected error")
	}
	if result.Error.Code != ErrInvalidInput {
		t.Fatalf("expected INVALID_INPUT, got %s", result.Error.Code)
	}
}

func TestService_TaskUpdate_Validation_ParentIdNotAllowed(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	service := NewService(client)
	result := service.Execute(context.Background(), Request{
		Operation: OpTaskUpdate,
		Payload:   payload("id", `"task-1"`, "title", `"x"`, "parentId", `"p1"`),
	})
	if result.OK {
		t.Fatal("expected error")
	}
	if result.Error.Code != ErrInvalidInput {
		t.Fatalf("expected INVALID_INPUT, got %s", result.Error.Code)
	}
}

func TestService_TaskSetCurrent_Validation_Missing(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	service := NewService(client)
	result := service.Execute(context.Background(), Request{
		Operation: OpTaskSetCurrent,
		Payload:   nil,
	})
	if result.OK {
		t.Fatal("expected error")
	}
	if result.Error.Code != ErrInvalidInput {
		t.Fatalf("expected INVALID_INPUT, got %s", result.Error.Code)
	}
}

func TestService_TaskList_Valid(t *testing.T) {
	fixture := loadFixture(t, "task-list-ok.json")
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	})
	defer ts.Close()

	service := NewService(client)
	result := service.Execute(context.Background(), Request{
		Operation: OpTaskList,
		Payload:   payload("source", `"active"`),
	})
	if !result.OK {
		t.Fatalf("expected OK, got error: %+v", result.Error)
	}
}

func TestService_TaskCreate_Valid(t *testing.T) {
	fixture := loadFixture(t, "task-create-ok.json")
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify the request body
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["title"] != "Write integration tests" {
			t.Fatalf("expected title, got %v", body["title"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	})
	defer ts.Close()

	service := NewService(client)
	result := service.Execute(context.Background(), Request{
		Operation: OpTaskCreate,
		Payload:   payload("title", `"Write integration tests"`),
	})
	if !result.OK {
		t.Fatalf("expected OK, got error: %+v", result.Error)
	}
}

func TestService_BridgeHealth_OK(t *testing.T) {
	healthFixture := loadFixture(t, "health-ok.json")
	statusFixture := loadFixture(t, "status-ok.json")
	callCount := 0
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if callCount == 0 {
			w.Write(healthFixture)
		} else {
			w.Write(statusFixture)
		}
		callCount++
	})
	defer ts.Close()

	service := NewService(client)
	result := service.Execute(context.Background(), Request{
		Operation: OpBridgeHealth,
	})
	if !result.OK {
		t.Fatalf("expected OK, got error: %+v", result.Error)
	}
	data := result.Data.(map[string]any)
	if data["health"] == nil {
		t.Fatal("expected health key")
	}
	if data["status"] == nil {
		t.Fatal("expected status key")
	}
}

// --- task.archive existence check (#27) ---
//
// SP's archive route answers {"ok":true,"archived":true} for ids that never
// existed, while get/update/start/restore all return TASK_NOT_FOUND. Agents
// invent plausible-looking ids, so this was the one operation where an invented
// one produced a confident success and the model had no signal to self-correct.

// archiveServer records which paths were hit, so a test can assert that the
// archive POST was never sent.
func archiveServer(t *testing.T, taskExists bool) (*httptest.Server, *Client, *[]string) {
	t.Helper()
	var hits []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/tasks/t1":
			if !taskExists {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Write([]byte(`{"ok":true,"data":{"id":"t1","title":"t1"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/tasks/t1/archive":
			// What SP does even for ids that do not exist.
			w.Write([]byte(`{"ok":true,"data":{"id":"t1","archived":true}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(ts.Close)
	return ts, NewClient(ts.URL), &hits
}

func archive(t *testing.T, client *Client, id string) Result {
	t.Helper()
	return NewService(client).Execute(context.Background(), Request{
		Operation: OpTaskArchive,
		Payload:   map[string]json.RawMessage{"id": json.RawMessage(`"` + id + `"`)},
	})
}

func TestService_TaskArchive_MissingTaskReportsNotFound(t *testing.T) {
	_, client, hits := archiveServer(t, false)
	result := archive(t, client, "t1")
	if result.OK {
		t.Fatal("archiving a task that does not exist must not report success")
	}
	if result.Error.Code != ErrTaskNotFound {
		t.Fatalf("expected %s, got %s", ErrTaskNotFound, result.Error.Code)
	}
	// The message must say nothing was archived; "Resource not found." left the
	// caller guessing whether anything had happened.
	if !strings.Contains(result.Error.Message, "nothing was archived") {
		t.Fatalf("message should say nothing happened, got: %s", result.Error.Message)
	}
	for _, hit := range *hits {
		if strings.Contains(hit, "/archive") {
			t.Fatalf("the archive POST must not be sent for a missing task, hits: %v", *hits)
		}
	}
}

func TestService_TaskArchive_ExistingTaskStillArchives(t *testing.T) {
	_, client, hits := archiveServer(t, true)
	result := archive(t, client, "t1")
	if !result.OK {
		t.Fatalf("archiving an existing task must still work, got %+v", result.Error)
	}
	var sawArchive bool
	for _, hit := range *hits {
		if hit == "POST /tasks/t1/archive" {
			sawArchive = true
		}
	}
	if !sawArchive {
		t.Fatalf("expected the archive POST, hits: %v", *hits)
	}
}

// A transport failure is not a missing task. Recasting it would tell the user
// their task does not exist when SP is simply down.
func TestService_TaskArchive_TransportErrorIsNotRecast(t *testing.T) {
	client := NewClient("http://127.0.0.1:1") // nothing listening
	result := archive(t, client, "t1")
	if result.OK {
		t.Fatal("expected failure")
	}
	if result.Error.Code == ErrTaskNotFound {
		t.Fatalf("an unreachable SP must not be reported as a missing task, got %s", result.Error.Code)
	}
	if result.Error.Code != ErrSPUnavailable {
		t.Fatalf("expected %s, got %s", ErrSPUnavailable, result.Error.Code)
	}
}

// The guard must not weaken the existing payload validation.
func TestService_TaskArchive_RejectsExtraFields(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	result := NewService(client).Execute(context.Background(), Request{
		Operation: OpTaskArchive,
		Payload: map[string]json.RawMessage{
			"id":    json.RawMessage(`"t1"`),
			"title": json.RawMessage(`"nope"`),
		},
	})
	if result.OK || result.Error.Code != ErrInvalidInput {
		t.Fatalf("expected %s, got %+v", ErrInvalidInput, result.Error)
	}
}
