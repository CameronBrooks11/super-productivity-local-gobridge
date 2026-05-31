package cli

import (
	"encoding/json"
	"testing"
)

func TestParseListFlags_Empty(t *testing.T) {
	result, err := parseListFlags(nil, taskListAllowed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestParseListFlags_Query(t *testing.T) {
	result, err := parseListFlags([]string{"--query", "hello"}, taskListAllowed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result["query"]) != `"hello"` {
		t.Fatalf("expected \"hello\", got %s", result["query"])
	}
}

func TestParseListFlags_MultipleFlags(t *testing.T) {
	result, err := parseListFlags([]string{"--query", "test", "--include-done", "--source", "all"}, taskListAllowed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result["query"]) != `"test"` {
		t.Fatalf("query: expected \"test\", got %s", result["query"])
	}
	if string(result["includeDone"]) != "true" {
		t.Fatalf("includeDone: expected true, got %s", result["includeDone"])
	}
	if string(result["source"]) != `"all"` {
		t.Fatalf("source: expected \"all\", got %s", result["source"])
	}
}

func TestParseListFlags_UnknownFlag(t *testing.T) {
	_, err := parseListFlags([]string{"--bogus"}, taskListAllowed)
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestParseListFlags_MissingValue(t *testing.T) {
	_, err := parseListFlags([]string{"--query"}, taskListAllowed)
	if err == nil {
		t.Fatal("expected error for missing value")
	}
}

func TestParseListFlags_UnexpectedPositional(t *testing.T) {
	_, err := parseListFlags([]string{"something"}, taskListAllowed)
	if err == nil {
		t.Fatal("expected error for positional arg")
	}
}

func TestParseListFlags_QueryOnly(t *testing.T) {
	_, err := parseListFlags([]string{"--include-done"}, queryOnlyAllowed)
	if err == nil {
		t.Fatal("expected error for disallowed flag")
	}
}

func TestRawPayload(t *testing.T) {
	p := rawPayload("id", "abc123")
	var s string
	if err := json.Unmarshal(p["id"], &s); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if s != "abc123" {
		t.Fatalf("expected abc123, got %s", s)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	code := Run([]string{"nonexistent"})
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestRun_Empty(t *testing.T) {
	code := Run(nil)
	if code != 0 {
		t.Fatalf("expected exit code 0 for empty args, got %d", code)
	}
}

func TestRun_TasksMissingID(t *testing.T) {
	cases := [][]string{
		{"tasks", "get"},
		{"tasks", "complete"},
		{"tasks", "uncomplete"},
		{"tasks", "start"},
		{"tasks", "archive"},
		{"tasks", "restore"},
		{"tasks", "set-current"},
	}
	for _, args := range cases {
		code := Run(args)
		if code != 2 {
			t.Fatalf("expected exit code 2 for %v, got %d", args, code)
		}
	}
}

func TestRun_TasksUpdateNoFields(t *testing.T) {
	code := Run([]string{"tasks", "update", "some-id"})
	if code != 2 {
		t.Fatalf("expected exit code 2 for update with no fields, got %d", code)
	}
}

func TestRun_ProjectsUnknownSub(t *testing.T) {
	code := Run([]string{"projects", "delete"})
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestRun_TagsUnknownSub(t *testing.T) {
	code := Run([]string{"tags", "delete"})
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}
