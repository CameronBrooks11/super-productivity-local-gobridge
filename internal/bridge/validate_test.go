package bridge

import (
	"encoding/json"
	"testing"
)

func raw(s string) json.RawMessage {
	return json.RawMessage(s)
}

func payload(pairs ...any) map[string]json.RawMessage {
	m := make(map[string]json.RawMessage)
	for i := 0; i < len(pairs); i += 2 {
		k := pairs[i].(string)
		v := pairs[i+1].(string)
		m[k] = raw(v)
	}
	return m
}

func TestValidateNoPayload_Empty(t *testing.T) {
	r := validateNoPayload(nil)
	if r != nil {
		t.Fatalf("expected nil, got %+v", r)
	}
}

func TestValidateNoPayload_WithFields(t *testing.T) {
	r := validateNoPayload(payload("foo", `"bar"`))
	if r == nil {
		t.Fatal("expected error")
	}
	if r.Error.Code != ErrInvalidInput {
		t.Fatalf("expected INVALID_INPUT, got %s", r.Error.Code)
	}
}

func TestValidateIDOnly_Valid(t *testing.T) {
	p := payload("id", `"task-1"`)
	r := validateIDOnly(p)
	if r != nil {
		t.Fatalf("expected nil, got %+v", r)
	}
}

func TestValidateIDOnly_Extra(t *testing.T) {
	p := payload("id", `"task-1"`, "extra", `"val"`)
	r := validateIDOnly(p)
	if r == nil {
		t.Fatal("expected error")
	}
	if r.Error.Code != ErrInvalidInput {
		t.Fatalf("expected INVALID_INPUT, got %s", r.Error.Code)
	}
}

func TestExtractID_Valid(t *testing.T) {
	p := payload("id", `"abc"`)
	id, r := extractID(p)
	if r != nil {
		t.Fatalf("expected nil error, got %+v", r)
	}
	if id != "abc" {
		t.Fatalf("expected 'abc', got '%s'", id)
	}
}

func TestExtractID_Missing(t *testing.T) {
	p := map[string]json.RawMessage{}
	_, r := extractID(p)
	if r == nil {
		t.Fatal("expected error")
	}
	if r.Error.Code != ErrInvalidInput {
		t.Fatalf("expected INVALID_INPUT, got %s", r.Error.Code)
	}
}

func TestExtractID_Empty(t *testing.T) {
	p := payload("id", `""`)
	_, r := extractID(p)
	if r == nil {
		t.Fatal("expected error")
	}
}

func TestExtractID_Null(t *testing.T) {
	p := payload("id", `null`)
	_, r := extractID(p)
	if r == nil {
		t.Fatal("expected error")
	}
}

func TestGetInt_Valid(t *testing.T) {
	p := payload("n", `42`)
	v, isNull, present, err := getInt(p, "n")
	if err != nil {
		t.Fatal(err)
	}
	if !present || isNull {
		t.Fatal("expected present, not null")
	}
	if v != 42 {
		t.Fatalf("expected 42, got %d", v)
	}
}

func TestGetInt_RejectsBool(t *testing.T) {
	p := payload("n", `true`)
	_, _, _, err := getInt(p, "n")
	if err == nil {
		t.Fatal("expected error for bool")
	}
}

func TestGetInt_RejectsFraction(t *testing.T) {
	p := payload("n", `3.14`)
	_, _, _, err := getInt(p, "n")
	if err == nil {
		t.Fatal("expected error for float")
	}
}

func TestGetInt_LargeInteger(t *testing.T) {
	// 2^53 + 1 would lose precision in float64
	p := payload("n", `9007199254740993`)
	v, _, present, err := getInt(p, "n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !present {
		t.Fatal("expected present")
	}
	if v != 9007199254740993 {
		t.Fatalf("precision loss: expected 9007199254740993, got %d", v)
	}
}

func TestGetInt_Negative(t *testing.T) {
	p := payload("n", `-500`)
	v, _, present, err := getInt(p, "n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !present {
		t.Fatal("expected present")
	}
	if v != -500 {
		t.Fatalf("expected -500, got %d", v)
	}
}

func TestGetInt_ExponentNotation(t *testing.T) {
	p := payload("n", `1e3`)
	v, _, present, err := getInt(p, "n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !present {
		t.Fatal("expected present")
	}
	if v != 1000 {
		t.Fatalf("expected 1000, got %d", v)
	}
}

func TestGetInt_RejectsString(t *testing.T) {
	p := payload("n", `"42"`)
	_, _, _, err := getInt(p, "n")
	if err == nil {
		t.Fatal("expected error for string")
	}
}

func TestGetInt_Missing(t *testing.T) {
	p := map[string]json.RawMessage{}
	_, _, present, err := getInt(p, "n")
	if err != nil {
		t.Fatal(err)
	}
	if present {
		t.Fatal("expected not present")
	}
}

func TestGetInt_Null(t *testing.T) {
	p := payload("n", `null`)
	_, isNull, present, err := getInt(p, "n")
	if err != nil {
		t.Fatal(err)
	}
	if !present || !isNull {
		t.Fatal("expected present and null")
	}
}

func TestGetNonNegativeInt_Negative(t *testing.T) {
	p := payload("t", `-100`)
	_, _, err := getNonNegativeInt(p, "t")
	if err == nil {
		t.Fatal("expected error for negative")
	}
}

func TestGetNonNegativeInt_Null(t *testing.T) {
	p := payload("t", `null`)
	_, _, err := getNonNegativeInt(p, "t")
	if err == nil {
		t.Fatal("expected error for null time field")
	}
}

func TestGetBool_Valid(t *testing.T) {
	p := payload("b", `true`)
	v, present, err := getBool(p, "b")
	if err != nil {
		t.Fatal(err)
	}
	if !present || !v {
		t.Fatal("expected true")
	}
}

func TestGetBool_Null(t *testing.T) {
	p := payload("b", `null`)
	_, _, err := getBool(p, "b")
	if err == nil {
		t.Fatal("expected error for null bool")
	}
}

func TestGetStringArray_Valid(t *testing.T) {
	p := payload("arr", `["a","b"]`)
	arr, present, err := getStringArray(p, "arr")
	if err != nil {
		t.Fatal(err)
	}
	if !present {
		t.Fatal("expected present")
	}
	if len(arr) != 2 || arr[0] != "a" || arr[1] != "b" {
		t.Fatalf("unexpected: %v", arr)
	}
}

func TestGetStringArray_Null(t *testing.T) {
	p := payload("arr", `null`)
	_, _, err := getStringArray(p, "arr")
	if err == nil {
		t.Fatal("expected error for null")
	}
}

func TestValidateTaskListFilters_Valid(t *testing.T) {
	p := payload("query", `"test"`, "includeDone", `true`, "source", `"all"`)
	params, r := validateTaskListFilters(p)
	if r != nil {
		t.Fatalf("expected nil, got %+v", r)
	}
	if params["query"] != "test" {
		t.Fatalf("expected query=test, got %s", params["query"])
	}
	if params["includeDone"] != "true" {
		t.Fatalf("expected includeDone=true, got %s", params["includeDone"])
	}
	if params["source"] != "all" {
		t.Fatalf("expected source=all, got %s", params["source"])
	}
}

func TestValidateTaskListFilters_InvalidSource(t *testing.T) {
	p := payload("source", `"invalid"`)
	_, r := validateTaskListFilters(p)
	if r == nil {
		t.Fatal("expected error")
	}
}

func TestValidateTaskListFilters_UnknownField(t *testing.T) {
	p := payload("unknown", `"val"`)
	_, r := validateTaskListFilters(p)
	if r == nil {
		t.Fatal("expected error")
	}
}

func TestValidateTaskFields_Create_Valid(t *testing.T) {
	p := payload("title", `"test"`, "notes", `"some notes"`, "projectId", `"proj-1"`)
	body, r := validateTaskFields(p, taskWritableFields, nil)
	if r != nil {
		t.Fatalf("expected nil, got %+v", r)
	}
	if body["title"] != "test" {
		t.Fatalf("expected title=test, got %v", body["title"])
	}
	if body["notes"] != "some notes" {
		t.Fatalf("expected notes, got %v", body["notes"])
	}
	if body["projectId"] != "proj-1" {
		t.Fatalf("expected projectId=proj-1, got %v", body["projectId"])
	}
}

func TestValidateTaskFields_NullProjectId(t *testing.T) {
	p := payload("title", `"test"`, "projectId", `null`)
	body, r := validateTaskFields(p, taskWritableFields, nil)
	if r != nil {
		t.Fatalf("expected nil, got %+v", r)
	}
	if body["projectId"] != nil {
		t.Fatalf("expected nil, got %v", body["projectId"])
	}
}

func TestValidateTaskFields_TimeEstimate_Valid(t *testing.T) {
	p := payload("title", `"test"`, "timeEstimate", `3600000`)
	body, r := validateTaskFields(p, taskWritableFields, nil)
	if r != nil {
		t.Fatalf("expected nil, got %+v", r)
	}
	if body["timeEstimate"] != int64(3600000) {
		t.Fatalf("expected 3600000, got %v", body["timeEstimate"])
	}
}

func TestValidateTaskFields_TimeEstimate_Negative(t *testing.T) {
	p := payload("title", `"test"`, "timeEstimate", `-1`)
	_, r := validateTaskFields(p, taskWritableFields, nil)
	if r == nil {
		t.Fatal("expected error for negative timeEstimate")
	}
}

func TestValidateTaskFields_UnknownField(t *testing.T) {
	p := payload("title", `"test"`, "badField", `"val"`)
	_, r := validateTaskFields(p, taskWritableFields, nil)
	if r == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestValidateTaskFields_TagIds_Valid(t *testing.T) {
	p := payload("title", `"test"`, "tagIds", `["tag1","tag2"]`)
	body, r := validateTaskFields(p, taskWritableFields, nil)
	if r != nil {
		t.Fatalf("expected nil, got %+v", r)
	}
	tags := body["tagIds"].([]string)
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
}

func TestValidateTaskFields_DueWithTime_Null(t *testing.T) {
	p := payload("title", `"test"`, "dueWithTime", `null`)
	body, r := validateTaskFields(p, taskWritableFields, nil)
	if r != nil {
		t.Fatalf("expected nil, got %+v", r)
	}
	if body["dueWithTime"] != nil {
		t.Fatalf("expected nil, got %v", body["dueWithTime"])
	}
}

func TestValidateTaskFields_DueWithTime_Bool(t *testing.T) {
	p := payload("title", `"test"`, "dueWithTime", `true`)
	_, r := validateTaskFields(p, taskWritableFields, nil)
	if r == nil {
		t.Fatal("expected error for bool dueWithTime")
	}
}

func TestValidateTaskFields_PlannedAt_String(t *testing.T) {
	p := payload("title", `"test"`, "plannedAt", `"2025-01-15"`)
	body, r := validateTaskFields(p, taskWritableFields, nil)
	if r != nil {
		t.Fatalf("expected nil, got %+v", r)
	}
	if body["plannedAt"] != "2025-01-15" {
		t.Fatalf("expected date string, got %v", body["plannedAt"])
	}
}

func TestValidateTaskFields_PlannedAt_Int(t *testing.T) {
	p := payload("title", `"test"`, "plannedAt", `1700000000000`)
	body, r := validateTaskFields(p, taskWritableFields, nil)
	if r != nil {
		t.Fatalf("expected nil, got %+v", r)
	}
	if body["plannedAt"] != int64(1700000000000) {
		t.Fatalf("expected epoch int, got %v", body["plannedAt"])
	}
}

func TestValidateTaskFields_PlannedAt_Float(t *testing.T) {
	p := payload("title", `"test"`, "plannedAt", `3.14`)
	_, r := validateTaskFields(p, taskWritableFields, nil)
	if r == nil {
		t.Fatal("expected error for float plannedAt")
	}
}

func TestValidateTaskFields_PlannedAt_Bool(t *testing.T) {
	p := payload("title", `"test"`, "plannedAt", `true`)
	_, r := validateTaskFields(p, taskWritableFields, nil)
	if r == nil {
		t.Fatal("expected error for bool plannedAt")
	}
}

func TestValidateTaskFields_IsDone_Valid(t *testing.T) {
	p := payload("title", `"test"`, "isDone", `false`)
	body, r := validateTaskFields(p, taskWritableFields, nil)
	if r != nil {
		t.Fatalf("expected nil, got %+v", r)
	}
	if body["isDone"] != false {
		t.Fatalf("expected false, got %v", body["isDone"])
	}
}

func TestValidateTaskFields_ParentId_Valid(t *testing.T) {
	p := payload("title", `"test"`, "parentId", `"parent-1"`)
	body, r := validateTaskFields(p, taskWritableFields, nil)
	if r != nil {
		t.Fatalf("expected nil, got %+v", r)
	}
	if body["parentId"] != "parent-1" {
		t.Fatalf("expected parent-1, got %v", body["parentId"])
	}
}

func TestValidateTaskFields_ParentId_Null(t *testing.T) {
	p := payload("title", `"test"`, "parentId", `null`)
	_, r := validateTaskFields(p, taskWritableFields, nil)
	if r == nil {
		t.Fatal("expected error for null parentId")
	}
}

func TestValidateTaskFields_ParentId_Empty(t *testing.T) {
	p := payload("title", `"test"`, "parentId", `""`)
	_, r := validateTaskFields(p, taskWritableFields, nil)
	if r == nil {
		t.Fatal("expected error for empty parentId")
	}
}

func TestValidateQueryOnly_Valid(t *testing.T) {
	p := payload("query", `"search term"`)
	params, r := validateQueryOnly(p)
	if r != nil {
		t.Fatalf("expected nil, got %+v", r)
	}
	if params["query"] != "search term" {
		t.Fatalf("expected 'search term', got '%s'", params["query"])
	}
}

func TestValidateQueryOnly_Empty(t *testing.T) {
	p := map[string]json.RawMessage{}
	params, r := validateQueryOnly(p)
	if r != nil {
		t.Fatalf("expected nil, got %+v", r)
	}
	if len(params) != 0 {
		t.Fatalf("expected empty params, got %v", params)
	}
}

func TestValidateQueryOnly_Unknown(t *testing.T) {
	p := payload("query", `"test"`, "extra", `"bad"`)
	_, r := validateQueryOnly(p)
	if r == nil {
		t.Fatal("expected error")
	}
}
