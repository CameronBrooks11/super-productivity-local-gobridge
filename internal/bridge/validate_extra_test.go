package bridge

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Test bool-as-int rejection for every integer field.
func TestValidate_BoolAsInt_AllIntegerFields(t *testing.T) {
	intFields := []string{"timeEstimate", "timeSpent", "dueWithTime"}
	for _, field := range intFields {
		p := payload("title", `"test"`, field, `true`)
		_, r := validateTaskFields(p, taskWritableFields, nil)
		if r == nil {
			t.Errorf("expected error for bool value in %s", field)
		}
		p2 := payload("title", `"test"`, field, `false`)
		_, r2 := validateTaskFields(p2, taskWritableFields, nil)
		if r2 == nil {
			t.Errorf("expected error for false value in %s", field)
		}
	}
}

// Test integer overflow for timeEstimate and timeSpent.
func TestValidate_IntOverflow_AllIntegerFields(t *testing.T) {
	intFields := []string{"timeEstimate", "timeSpent"}
	// int64 max + 1
	for _, field := range intFields {
		p := payload("title", `"test"`, field, `9223372036854775808`)
		_, r := validateTaskFields(p, taskWritableFields, nil)
		if r == nil {
			t.Errorf("expected overflow error for %s", field)
		}
	}
}

// Test exponent notation rejected in dueWithTime.
func TestValidate_ExponentNotation_DueWithTime(t *testing.T) {
	cases := []string{`1e3`, `1E10`, `5e0`}
	for _, c := range cases {
		p := payload("title", `"test"`, "dueWithTime", c)
		_, r := validateTaskFields(p, taskWritableFields, nil)
		if r == nil {
			t.Errorf("expected error for exponent notation %s in dueWithTime", c)
		}
	}
}

// Test exponent notation in plannedAt integer path.
func TestValidate_ExponentNotation_PlannedAt(t *testing.T) {
	cases := []string{`1e3`, `1E10`}
	for _, c := range cases {
		p := payload("title", `"test"`, "plannedAt", c)
		_, r := validateTaskFields(p, taskWritableFields, nil)
		if r == nil {
			t.Errorf("expected error for exponent notation %s in plannedAt", c)
		}
	}
}

// Test multiple unknown fields in error message.
func TestValidate_MultipleUnknownFields(t *testing.T) {
	p := payload("title", `"test"`, "badField1", `"x"`, "badField2", `"y"`)
	_, r := validateTaskFields(p, taskWritableFields, nil)
	if r == nil {
		t.Fatal("expected error for unknown fields")
	}
	// Should mention at least one unknown field
	if !strings.Contains(r.Error.Message, "Unknown fields") {
		t.Errorf("expected 'Unknown fields' in message, got %q", r.Error.Message)
	}
}

// Test set_current_task with explicit null taskId (clears current).
func TestService_SetCurrent_NullTaskId(t *testing.T) {
	// set_current with null taskId should send to SP to clear current task
	// Use a mock server that accepts the request
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"data":null}`))
	})
	defer ts.Close()

	service := NewService(client)
	result := service.Execute(context.Background(), Request{
		Operation: OpTaskSetCurrent,
		Payload:   payload("taskId", `null`),
	})
	if !result.OK {
		t.Fatalf("expected OK for null taskId, got error: %+v", result.Error)
	}
}

// Test set_current_task with valid taskId.
func TestService_SetCurrent_ValidTaskId(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"data":{"id":"task-1","title":"Test"}}`))
	})
	defer ts.Close()

	service := NewService(client)
	result := service.Execute(context.Background(), Request{
		Operation: OpTaskSetCurrent,
		Payload:   payload("taskId", `"task-1"`),
	})
	if !result.OK {
		t.Fatalf("expected OK, got error: %+v", result.Error)
	}
}

// Test set_current_task missing taskId.
func TestService_SetCurrent_MissingTaskId(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	service := NewService(client)
	result := service.Execute(context.Background(), Request{
		Operation: OpTaskSetCurrent,
		Payload:   map[string]json.RawMessage{},
	})
	if result.OK {
		t.Fatal("expected error for missing taskId")
	}
	if result.Error.Code != ErrInvalidInput {
		t.Fatalf("expected INVALID_INPUT, got %s", result.Error.Code)
	}
}

// Test no-payload operations reject non-empty payloads.
func TestService_NoPayloadOps_RejectPayload(t *testing.T) {
	noPayloadOps := []string{
		OpBridgeHealth,
		OpStatusGet,
		OpTaskStopCurrent,
		OpTaskGetCurrent,
	}
	client := NewClient("http://127.0.0.1:1")
	service := NewService(client)
	for _, op := range noPayloadOps {
		result := service.Execute(context.Background(), Request{
			Operation: op,
			Payload:   payload("unexpected", `"value"`),
		})
		if result.OK {
			t.Errorf("op %s should reject non-empty payload", op)
		}
		if result.Error.Code != ErrInvalidInput {
			t.Errorf("op %s: expected INVALID_INPUT, got %s", op, result.Error.Code)
		}
	}
}

// Test no-payload operations accept empty payload.
func TestService_NoPayloadOps_AcceptEmpty(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"data":{"status":"up"}}`))
	})
	defer ts.Close()

	service := NewService(client)
	for _, op := range []string{OpBridgeHealth, OpStatusGet, OpTaskStopCurrent, OpTaskGetCurrent} {
		result := service.Execute(context.Background(), Request{
			Operation: op,
			Payload:   nil,
		})
		if !result.OK {
			t.Errorf("op %s should accept nil payload, got error: %+v", op, result.Error)
		}
	}
}

// Test update_task with no fields to update (only id).
func TestService_TaskUpdate_NoFieldsToUpdate(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	service := NewService(client)
	result := service.Execute(context.Background(), Request{
		Operation: OpTaskUpdate,
		Payload:   payload("id", `"task-1"`),
	})
	if result.OK {
		t.Fatal("expected error for no fields to update")
	}
	if result.Error.Code != ErrInvalidInput {
		t.Fatalf("expected INVALID_INPUT, got %s", result.Error.Code)
	}
}

// Test update_task rejects parentId.
func TestService_TaskUpdate_RejectsParentId(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	service := NewService(client)
	result := service.Execute(context.Background(), Request{
		Operation: OpTaskUpdate,
		Payload:   payload("id", `"task-1"`, "parentId", `"parent"`),
	})
	if result.OK {
		t.Fatal("expected error for parentId on update")
	}
	if result.Error.Code != ErrInvalidInput {
		t.Fatalf("expected INVALID_INPUT, got %s", result.Error.Code)
	}
}

// Test valid source values for task.list.
func TestService_TaskList_ValidSources(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	})
	defer ts.Close()

	service := NewService(client)
	for _, source := range []string{"active", "archived", "all"} {
		result := service.Execute(context.Background(), Request{
			Operation: OpTaskList,
			Payload:   payload("source", `"`+source+`"`),
		})
		if !result.OK {
			t.Errorf("source=%s should be valid, got error: %+v", source, result.Error)
		}
	}
}

// Test invalid source value rejected.
func TestService_TaskList_InvalidSource(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	service := NewService(client)
	result := service.Execute(context.Background(), Request{
		Operation: OpTaskList,
		Payload:   payload("source", `"deleted"`),
	})
	if result.OK {
		t.Fatal("expected error for invalid source")
	}
	if result.Error.Code != ErrInvalidInput {
		t.Fatalf("expected INVALID_INPUT, got %s", result.Error.Code)
	}
}

// Test all create fields accepted together.
func TestService_TaskCreate_AllFields(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"data":{"id":"new-1","title":"Full task"}}`))
	})
	defer ts.Close()

	service := NewService(client)
	result := service.Execute(context.Background(), Request{
		Operation: OpTaskCreate,
		Payload: payload(
			"title", `"Full task"`,
			"notes", `"Some notes"`,
			"projectId", `"proj-1"`,
			"tagIds", `["tag1","tag2"]`,
			"plannedAt", `"2026-06-01"`,
			"dueDay", `"2026-06-15"`,
			"dueWithTime", `1700000000000`,
			"isDone", `false`,
			"timeEstimate", `3600000`,
			"timeSpent", `1800000`,
		),
	})
	if !result.OK {
		t.Fatalf("expected OK, got error: %+v", result.Error)
	}
}

// Test notes must be string not null.
func TestValidate_Notes_Null(t *testing.T) {
	p := payload("title", `"test"`, "notes", `null`)
	_, r := validateTaskFields(p, taskWritableFields, nil)
	if r == nil {
		t.Fatal("expected error for null notes")
	}
}

// Test dueDay accepts string.
func TestValidate_DueDay_ValidString(t *testing.T) {
	p := payload("title", `"test"`, "dueDay", `"2026-01-01"`)
	body, r := validateTaskFields(p, taskWritableFields, nil)
	if r != nil {
		t.Fatalf("expected nil, got %+v", r)
	}
	if body["dueDay"] != "2026-01-01" {
		t.Fatalf("expected 2026-01-01, got %v", body["dueDay"])
	}
}

// Test dueDay null clears it.
func TestValidate_DueDay_Null(t *testing.T) {
	p := payload("title", `"test"`, "dueDay", `null`)
	body, r := validateTaskFields(p, taskWritableFields, nil)
	if r != nil {
		t.Fatalf("expected nil, got %+v", r)
	}
	if body["dueDay"] != nil {
		t.Fatalf("expected nil, got %v", body["dueDay"])
	}
}

// Test isDone rejects integer.
func TestValidate_IsDone_RejectsInt(t *testing.T) {
	p := payload("title", `"test"`, "isDone", `1`)
	_, r := validateTaskFields(p, taskWritableFields, nil)
	if r == nil {
		t.Fatal("expected error for int isDone")
	}
}

// Test tagIds rejects non-array.
func TestValidate_TagIds_RejectsString(t *testing.T) {
	p := payload("title", `"test"`, "tagIds", `"single-tag"`)
	_, r := validateTaskFields(p, taskWritableFields, nil)
	if r == nil {
		t.Fatal("expected error for string tagIds")
	}
}

// Test tagIds rejects null.
func TestValidate_TagIds_RejectsNull(t *testing.T) {
	p := payload("title", `"test"`, "tagIds", `null`)
	_, r := validateTaskFields(p, taskWritableFields, nil)
	if r == nil {
		t.Fatal("expected error for null tagIds")
	}
}

// Test task.list with empty object payload is fine.
func TestService_TaskList_EmptyPayload(t *testing.T) {
	ts, client := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	})
	defer ts.Close()

	service := NewService(client)
	result := service.Execute(context.Background(), Request{
		Operation: OpTaskList,
		Payload:   map[string]json.RawMessage{},
	})
	if !result.OK {
		t.Fatalf("expected OK, got error: %+v", result.Error)
	}
}
