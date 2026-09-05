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

// SP answers both a missing task and a missing route with HTTP 404, and
// distinguishes them only in the body. The client used to short-circuit on the
// status before reading it, so every 404 became TASK_NOT_FOUND — a mistyped or
// removed route reported "task not found" and sent whoever was debugging it
// looking for a task that was never the problem.
func TestClient_GetTask_NotFound(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantCode   string
		wantStatus int
	}{
		{
			name:       "missing task carries SP's TASK_NOT_FOUND",
			body:       `{"ok":false,"error":{"code":"TASK_NOT_FOUND","message":"Task not found"}}`,
			wantCode:   ErrTaskNotFound,
			wantStatus: 404,
		},
		{
			name:       "missing route carries SP's NOT_FOUND",
			body:       `{"ok":false,"error":{"code":"NOT_FOUND","message":"Route not found"}}`,
			wantCode:   ErrNotFound,
			wantStatus: 404,
		},
		{
			// Not something SP does, but a proxy might. We cannot tell what was
			// missing, so claiming the task was is a guess.
			name:       "bodyless 404 is not claimed as a missing task",
			body:       "",
			wantCode:   ErrSPError,
			wantStatus: 404,
		},
		{
			name:       "non-JSON 404 is not claimed as a missing task",
			body:       "<html>404</html>",
			wantCode:   ErrSPError,
			wantStatus: 404,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(404)
				if tc.body != "" {
					w.Write([]byte(tc.body))
				}
			})
			defer ts.Close()

			result := client.GetTask(context.Background(), "nonexistent")
			if result.OK {
				t.Fatal("expected error")
			}
			if result.Error.Code != tc.wantCode {
				t.Fatalf("expected %s, got %s (%s)", tc.wantCode, result.Error.Code, result.Error.Message)
			}
			if got := result.Error.Details["status_code"]; got != tc.wantStatus {
				t.Fatalf("expected status_code %d, got %v", tc.wantStatus, got)
			}
		})
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
// SP's archive route reports success for ids that never existed, while
// get/update/start/restore all return TASK_NOT_FOUND. This was the one
// operation where a mistaken or invented id produced a confident success, so
// nothing signalled that the archive had not happened.

// archiveServer records which paths were hit, so a test can assert that the
// archive POST was never sent.
func archiveServer(t *testing.T, taskExists bool) (*Client, *[]string) {
	t.Helper()
	var hits []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/tasks/t1":
			if !taskExists {
				// SP's real 404 body. A bare status with no body is not what it
				// sends, and now that the client reads the envelope to tell a
				// missing task from a missing route, the difference matters.
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"ok":false,"error":{"code":"TASK_NOT_FOUND","message":"Task not found"}}`))
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
	return NewClient(ts.URL), &hits
}

func archive(t *testing.T, client *Client, id string) Result {
	t.Helper()
	return NewService(client).Execute(context.Background(), Request{
		Operation: OpTaskArchive,
		Payload:   map[string]json.RawMessage{"id": json.RawMessage(`"` + id + `"`)},
	})
}

func TestService_TaskArchive_MissingTaskReportsNotFound(t *testing.T) {
	client, hits := archiveServer(t, false)
	result := archive(t, client, "t1")
	if result.OK {
		t.Fatal("archiving a task that does not exist must not report success")
	}
	if result.Error.Code != ErrTaskNotFound {
		t.Fatalf("expected %s, got %s", ErrTaskNotFound, result.Error.Code)
	}
	// The message must say why: SP's own "Task not found" says what is missing
	// but not what the bridge was doing, or whether anything changed.
	if !strings.Contains(result.Error.Message, "not in the active list") {
		t.Fatalf("message should say why, got: %s", result.Error.Message)
	}
	for _, hit := range *hits {
		if strings.Contains(hit, "/archive") {
			t.Fatalf("the archive POST must not be sent for a missing task, hits: %v", *hits)
		}
	}
}

func TestService_TaskArchive_ExistingTaskStillArchives(t *testing.T) {
	client, hits := archiveServer(t, true)
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

// Every other TASK_NOT_FOUND carries status_code in details; archive must not
// be the one route whose envelope omits it, since the MCP adapter serializes
// details unconditionally and hosts branch on it.
func TestService_TaskArchive_NotFoundCarriesDetails(t *testing.T) {
	client, _ := archiveServer(t, false)
	result := archive(t, client, "t1")
	if result.Error == nil || result.Error.Details == nil {
		t.Fatalf("expected details on the error, got %+v", result.Error)
	}
	if got := result.Error.Details["status_code"]; got != 404 {
		t.Fatalf("expected status_code 404, got %v", got)
	}
}

// The details are forwarded from the underlying read rather than fabricated, so
// an envelope-derived not-found keeps its own status code and sp_details
// instead of being relabelled 404.
func TestService_TaskArchive_ForwardsOriginalErrorDetails(t *testing.T) {
	var unexpected string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			// A 200 carrying SP's error envelope, not a bare 404.
			w.Write([]byte(`{"ok":false,"error":{"code":"TASK_NOT_FOUND","message":"Task not found","details":{"why":"gone"}}}`))
			return
		}
		// t.Fatalf here would only Goexit this handler goroutine, dropping the
		// connection and surfacing downstream as a confusing SP_UNAVAILABLE.
		// Record it and assert on the main goroutine instead.
		unexpected = r.Method + " " + r.URL.Path
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	result := archive(t, NewClient(ts.URL), "t1")
	if unexpected != "" {
		t.Fatalf("archive POST must not be sent, got %s", unexpected)
	}
	if result.OK || result.Error.Code != ErrTaskNotFound {
		t.Fatalf("expected %s, got %+v", ErrTaskNotFound, result.Error)
	}
	if got := result.Error.Details["status_code"]; got != 200 {
		t.Fatalf("status code must come from the response, not be fabricated as 404; got %v", got)
	}
	if _, ok := result.Error.Details["sp_details"]; !ok {
		t.Fatalf("sp_details must survive, got %v", result.Error.Details)
	}
}

// The reason #37 mattered: the archive guard keys on TASK_NOT_FOUND, so while
// every 404 was flattened to that code, a probe route that stopped resolving
// would have been reported as a missing task — turning a working archive into a
// confidently wrong error.
func TestService_TaskArchive_RouteNotFoundIsNotReportedAsMissingTask(t *testing.T) {
	var archived bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			// The GET route itself is gone, not the task.
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"ok":false,"error":{"code":"NOT_FOUND","message":"Route not found"}}`))
			return
		}
		archived = true
		w.Write([]byte(`{"ok":true,"data":{"id":"t1","archived":true}}`))
	}))
	defer ts.Close()

	result := archive(t, NewClient(ts.URL), "t1")
	if result.OK {
		t.Fatal("expected failure")
	}
	if result.Error.Code == ErrTaskNotFound {
		t.Fatal("a missing route must not be reported as a missing task")
	}
	if result.Error.Code != ErrNotFound {
		t.Fatalf("expected %s, got %s", ErrNotFound, result.Error.Code)
	}
	if !strings.Contains(result.Error.Message, "Route not found") {
		t.Fatalf("SP's own message should survive, got: %s", result.Error.Message)
	}
	// Still fails closed: the dangerous POST is not sent on an unclear read.
	if archived {
		t.Fatal("the archive POST must not be sent when the existence check is inconclusive")
	}
}

// An HTTP error status carrying ok:true is contradictory. Believing the body
// would report a failed request as a success — and for task.archive, which
// reads a task to decide whether it exists before writing, that means treating
// a 404 as "it exists" and sending the call that crashed SP's renderer.
func TestClient_SuccessEnvelopeOnErrorStatusIsAnError(t *testing.T) {
	for _, status := range []int{404, 400, 500} {
		ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			w.Write([]byte(`{"ok":true,"data":{"id":"t1"}}`))
		})
		result := client.GetTask(context.Background(), "t1")
		ts.Close()
		if result.OK {
			t.Errorf("status %d with ok:true must not be a success", status)
			continue
		}
		if result.Error.Code != ErrSPError {
			t.Errorf("status %d: expected %s, got %s", status, ErrSPError, result.Error.Code)
		}
		if got := result.Error.Details["status_code"]; got != status {
			t.Errorf("status %d: details should carry the status, got %v", status, got)
		}
	}
}

// The guard must fail closed on a contradictory probe response: no archive POST.
func TestService_TaskArchive_ContradictoryProbeDoesNotArchive(t *testing.T) {
	var posted bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"ok":true,"data":{"id":"t1"}}`)) // 404 but claims success
			return
		}
		posted = true
		w.Write([]byte(`{"ok":true,"data":{"id":"t1","archived":true}}`))
	}))
	defer ts.Close()

	result := archive(t, NewClient(ts.URL), "t1")
	if posted {
		t.Fatal("a contradictory existence probe must not lead to an archive POST")
	}
	if result.OK {
		t.Fatal("expected failure")
	}
}

// A 2xx carrying ok:true is the normal path and must keep working.
func TestClient_SuccessEnvelopeOnOKStatusStillSucceeds(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"data":{"id":"t1","title":"t1"}}`))
	})
	defer ts.Close()
	if result := client.GetTask(context.Background(), "t1"); !result.OK {
		t.Fatalf("a 200 with ok:true must succeed, got %+v", result.Error)
	}
}

// A successful status does not confirm a task. An empty body, a non-JSON body
// and {"ok":true,"data":null} all translate to Success(nil), and archiving on
// the strength of one would send the POST for an id never confirmed — the same
// call the guard exists to prevent.
func TestService_TaskArchive_UnconfirmedProbeDoesNotArchive(t *testing.T) {
	cases := map[string]string{
		"empty body":         "",
		"non-JSON body":      "<html>ok</html>",
		"data is null":       `{"ok":true,"data":null}`,
		"wrong task id":      `{"ok":true,"data":{"id":"someone-else","title":"x"}}`,
		"data not an object": `{"ok":true,"data":[{"id":"t1"}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			var posted bool
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					w.WriteHeader(http.StatusOK)
					if body != "" {
						w.Write([]byte(body))
					}
					return
				}
				posted = true
				w.Write([]byte(`{"ok":true,"data":{"id":"t1","archived":true}}`))
			}))
			defer ts.Close()

			result := archive(t, NewClient(ts.URL), "t1")
			if posted {
				t.Fatal("the archive POST must not be sent when the probe did not confirm the task")
			}
			if result.OK {
				t.Fatal("expected failure")
			}
			if !strings.Contains(result.Error.Message, "Could not confirm") {
				t.Fatalf("message should say the task was unconfirmed, got: %s", result.Error.Message)
			}
			// Collapsing every shape to one contentless error made a proxy's
			// HTML interstitial and SP returning a different task's id look
			// identical, so neither could be diagnosed without a manual repro.
			desc, _ := result.Error.Details["probe_returned"].(string)
			if desc == "" {
				t.Fatal("the error should record what the probe returned")
			}
			if name == "wrong task id" && !strings.Contains(desc, "someone-else") {
				t.Fatalf("a mismatched id is the signal worth surfacing, got %q", desc)
			}
		})
	}
}

// The normal path: the probe returns the task, so the archive proceeds.
func TestService_TaskArchive_ConfirmedProbeArchives(t *testing.T) {
	var posted bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			w.Write([]byte(`{"ok":true,"data":{"id":"t1","title":"real"}}`))
			return
		}
		posted = true
		w.Write([]byte(`{"ok":true,"data":{"id":"t1","archived":true}}`))
	}))
	defer ts.Close()

	if result := archive(t, NewClient(ts.URL), "t1"); !result.OK {
		t.Fatalf("a confirmed task must still archive, got %+v", result.Error)
	}
	if !posted {
		t.Fatal("expected the archive POST")
	}
}
