package bridge

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// Test every route path and method.

func TestClient_Routes_Health(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Health: expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/health" {
			t.Errorf("Health: expected /health, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"data":{"status":"up"}}`))
	})
	defer ts.Close()
	result := client.Health(context.Background())
	if !result.OK {
		t.Fatal("expected OK")
	}
}

func TestClient_Routes_Status(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Status: expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/status" {
			t.Errorf("Status: expected /status, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"data":{"currentTask":null}}`))
	})
	defer ts.Close()
	result := client.Status(context.Background())
	if !result.OK {
		t.Fatal("expected OK")
	}
}

func TestClient_Routes_ListTasks(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/tasks" {
			t.Errorf("expected /tasks, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	})
	defer ts.Close()
	result := client.ListTasks(context.Background(), nil)
	if !result.OK {
		t.Fatal("expected OK")
	}
}

func TestClient_Routes_GetTask(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/tasks/abc-123" {
			t.Errorf("expected /tasks/abc-123, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"data":{"id":"abc-123","title":"Test"}}`))
	})
	defer ts.Close()
	result := client.GetTask(context.Background(), "abc-123")
	if !result.OK {
		t.Fatal("expected OK")
	}
}

func TestClient_Routes_CreateTask(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/tasks" {
			t.Errorf("expected /tasks, got %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json")
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["title"] != "New task" {
			t.Errorf("expected title 'New task', got %v", body["title"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"data":{"id":"new-1","title":"New task"}}`))
	})
	defer ts.Close()
	result := client.CreateTask(context.Background(), map[string]any{"title": "New task"})
	if !result.OK {
		t.Fatal("expected OK")
	}
}

func TestClient_Routes_UpdateTask(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/tasks/task-1" {
			t.Errorf("expected /tasks/task-1, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"data":{"id":"task-1","title":"Updated"}}`))
	})
	defer ts.Close()
	result := client.UpdateTask(context.Background(), "task-1", map[string]any{"title": "Updated"})
	if !result.OK {
		t.Fatal("expected OK")
	}
}

func TestClient_Routes_StartTask(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/tasks/task-1/start" {
			t.Errorf("expected /tasks/task-1/start, got %s", r.URL.Path)
		}
		w.WriteHeader(204)
	})
	defer ts.Close()
	result := client.StartTask(context.Background(), "task-1")
	if !result.OK {
		t.Fatal("expected OK")
	}
}

func TestClient_Routes_StopCurrentTask(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/task-control/stop" {
			t.Errorf("expected /task-control/stop, got %s", r.URL.Path)
		}
		w.WriteHeader(204)
	})
	defer ts.Close()
	result := client.StopCurrentTask(context.Background())
	if !result.OK {
		t.Fatal("expected OK")
	}
}

func TestClient_Routes_GetCurrentTask(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/task-control/current" {
			t.Errorf("expected /task-control/current, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"data":null}`))
	})
	defer ts.Close()
	result := client.GetCurrentTask(context.Background())
	if !result.OK {
		t.Fatal("expected OK")
	}
}

func TestClient_Routes_SetCurrentTask(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/task-control/current" {
			t.Errorf("expected /task-control/current, got %s", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["taskId"] != "task-1" {
			t.Errorf("expected taskId=task-1, got %v", body["taskId"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"data":{"id":"task-1"}}`))
	})
	defer ts.Close()
	taskID := "task-1"
	result := client.SetCurrentTask(context.Background(), &taskID)
	if !result.OK {
		t.Fatal("expected OK")
	}
}

func TestClient_Routes_SetCurrentTask_Clear(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["taskId"] != nil {
			t.Errorf("expected taskId=null, got %v", body["taskId"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"data":null}`))
	})
	defer ts.Close()
	result := client.SetCurrentTask(context.Background(), nil)
	if !result.OK {
		t.Fatal("expected OK")
	}
}

func TestClient_Routes_ArchiveTask(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/tasks/task-1/archive" {
			t.Errorf("expected /tasks/task-1/archive, got %s", r.URL.Path)
		}
		w.WriteHeader(204)
	})
	defer ts.Close()
	result := client.ArchiveTask(context.Background(), "task-1")
	if !result.OK {
		t.Fatal("expected OK")
	}
}

func TestClient_Routes_RestoreTask(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/tasks/task-1/restore" {
			t.Errorf("expected /tasks/task-1/restore, got %s", r.URL.Path)
		}
		w.WriteHeader(204)
	})
	defer ts.Close()
	result := client.RestoreTask(context.Background(), "task-1")
	if !result.OK {
		t.Fatal("expected OK")
	}
}

func TestClient_Routes_ListProjects(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/projects" {
			t.Errorf("expected /projects, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	})
	defer ts.Close()
	result := client.ListProjects(context.Background(), nil)
	if !result.OK {
		t.Fatal("expected OK")
	}
}

func TestClient_Routes_ListTags(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/tags" {
			t.Errorf("expected /tags, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	})
	defer ts.Close()
	result := client.ListTags(context.Background(), nil)
	if !result.OK {
		t.Fatal("expected OK")
	}
}

// Test query parameter encoding.
func TestClient_QueryParams(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("query") != "hello world" {
			t.Errorf("expected 'hello world', got %q", q.Get("query"))
		}
		if q.Get("tagId") != "TODAY" {
			t.Errorf("expected 'TODAY', got %q", q.Get("tagId"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	})
	defer ts.Close()
	result := client.ListTasks(context.Background(), map[string]string{
		"query": "hello world",
		"tagId": "TODAY",
	})
	if !result.OK {
		t.Fatal("expected OK")
	}
}

// Test that no Origin header is sent.
func TestClient_NoOriginHeader(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			t.Errorf("expected no Origin header, got %q", origin)
		}
		if referer := r.Header.Get("Referer"); referer != "" {
			t.Errorf("expected no Referer header, got %q", referer)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"data":{"status":"up"}}`))
	})
	defer ts.Close()
	client.Health(context.Background())
}

// Test structured error envelope with details.
func TestClient_ErrorEnvelope_WithDetails(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write([]byte(`{"ok":false,"error":{"code":"INVALID_REQUEST_BODY","message":"Validation failed","details":{"field":"title","reason":"required"}}}`))
	})
	defer ts.Close()
	result := client.Health(context.Background())
	if result.OK {
		t.Fatal("expected error")
	}
	if result.Error.Code != "INVALID_REQUEST_BODY" {
		t.Errorf("expected INVALID_REQUEST_BODY, got %s", result.Error.Code)
	}
	if result.Error.Message != "Validation failed" {
		t.Errorf("expected 'Validation failed', got %q", result.Error.Message)
	}
	if result.Error.Details == nil {
		t.Fatal("expected details map")
	}
	if result.Error.Details["sp_details"] == nil {
		t.Error("expected sp_details in error details")
	}
}

// Test 503 with error envelope.
func TestClient_ErrorEnvelope_503(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		w.Write([]byte(`{"ok":false,"error":{"code":"APP_NOT_READY","message":"App is loading"}}`))
	})
	defer ts.Close()
	result := client.Health(context.Background())
	if result.OK {
		t.Fatal("expected error")
	}
	if result.Error.Code != "APP_NOT_READY" {
		t.Errorf("expected APP_NOT_READY, got %s", result.Error.Code)
	}
}

// Test non-JSON error response.
func TestClient_NonJSONError(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("Internal Server Error"))
	})
	defer ts.Close()
	result := client.Health(context.Background())
	if result.OK {
		t.Fatal("expected error")
	}
	if result.Error.Code != ErrSPError {
		t.Errorf("expected SP_ERROR, got %s", result.Error.Code)
	}
}

// Test empty 204 response (success, no body).
func TestClient_Empty204(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	})
	defer ts.Close()
	result := client.StopCurrentTask(context.Background())
	if !result.OK {
		t.Fatalf("expected OK, got error: %+v", result.Error)
	}
}

// Test non-envelope JSON array response (e.g., /tasks returning []).
func TestClient_ArrayResponse(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":"t1","title":"Task 1"}]`))
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
	if len(arr) != 1 {
		t.Errorf("expected 1 task, got %d", len(arr))
	}
}

// http.Client.Timeout caps every request independently of the context deadline,
// so a caller needing longer than the default must be able to raise it.
func TestNewClientWithTimeout(t *testing.T) {
	c := NewClientWithTimeout("http://example.invalid", 45*time.Second)
	if c.httpClient.Timeout != 45*time.Second {
		t.Fatalf("timeout not applied: got %v", c.httpClient.Timeout)
	}
	if d := NewClientWithTimeout("http://example.invalid", 0); d.httpClient.Timeout != defaultTimeout {
		t.Fatalf("non-positive timeout should fall back to default, got %v", d.httpClient.Timeout)
	}
	if n := NewClient("http://example.invalid"); n.httpClient.Timeout != defaultTimeout {
		t.Fatalf("NewClient must keep the default timeout, got %v", n.httpClient.Timeout)
	}
}
