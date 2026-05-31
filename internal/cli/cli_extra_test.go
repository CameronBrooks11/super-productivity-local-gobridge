package cli

import (
	"os"
	"testing"
)

// Test tasks add with flags parses correctly (exits 1 = SP unavailable, not 2 = parse error).
func TestRun_TasksAdd_WithFlags(t *testing.T) {
	t.Setenv("SP_BASE_URL", "http://127.0.0.1:1")
	code := Run([]string{"tasks", "add", "My Task", "--project-id", "proj-1"})
	if code != 1 {
		t.Errorf("expected exit code 1 (SP down), got %d", code)
	}
}

// Test tasks add with --tag-id.
func TestRun_TasksAdd_WithTagID(t *testing.T) {
	t.Setenv("SP_BASE_URL", "http://127.0.0.1:1")
	code := Run([]string{"tasks", "add", "Tagged Task", "--tag-id", "tag-abc"})
	if code != 1 {
		t.Errorf("expected exit code 1 (SP down), got %d", code)
	}
}

// Test tasks add with multiple flags.
func TestRun_TasksAdd_MultipleFlags(t *testing.T) {
	t.Setenv("SP_BASE_URL", "http://127.0.0.1:1")
	code := Run([]string{"tasks", "add", "Full Task", "--project-id", "proj-1", "--tag-id", "tag-1", "--notes", "Some notes", "--due-day", "2026-06-15"})
	if code != 1 {
		t.Errorf("expected exit code 1 (SP down), got %d", code)
	}
}

// Test tasks add rejects unknown flags.
func TestRun_TasksAdd_UnknownFlag(t *testing.T) {
	code := Run([]string{"tasks", "add", "Bad", "--bogus-flag", "val"})
	if code != 2 {
		t.Errorf("expected exit code 2 for unknown flag, got %d", code)
	}
}

// Test tasks add rejects missing flag value.
func TestRun_TasksAdd_MissingFlagValue(t *testing.T) {
	flags := []string{"--project-id", "--tag-id", "--notes", "--due-day", "--time-estimate"}
	for _, flag := range flags {
		code := Run([]string{"tasks", "add", "Title", flag})
		if code != 2 {
			t.Errorf("flag %s without value: expected exit code 2, got %d", flag, code)
		}
	}
}

// Test tasks add --help shows usage.
func TestRun_TasksAdd_Help(t *testing.T) {
	old := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = old }()

	code := Run([]string{"tasks", "add", "--help"})
	if code != 0 {
		t.Errorf("expected exit code 0 for --help, got %d", code)
	}
}

// Test SP_BASE_URL is respected.
func TestRun_SPBaseURLEnv(t *testing.T) {
	// Use a dead port so health fails immediately
	t.Setenv("SP_BASE_URL", "http://127.0.0.1:1")
	code := Run([]string{"health"})
	// Should fail with exit 1 (SP_UNAVAILABLE), not 2 (usage error)
	if code != 1 {
		t.Fatalf("expected exit code 1 (SP_UNAVAILABLE), got %d", code)
	}
}

// Test all commands that require no subcommand produce exit 1 when SP is down.
func TestRun_SPDown_ExitCode1(t *testing.T) {
	t.Setenv("SP_BASE_URL", "http://127.0.0.1:1")
	commands := [][]string{
		{"health"},
		{"status"},
		{"tasks", "list"},
		{"tasks", "get", "fake-id"},
		{"tasks", "add", "test task"},
		{"tasks", "complete", "fake-id"},
		{"tasks", "uncomplete", "fake-id"},
		{"tasks", "start", "fake-id"},
		{"tasks", "stop-current"},
		{"tasks", "current"},
		{"tasks", "set-current", "fake-id"},
		{"tasks", "clear-current"},
		{"tasks", "archive", "fake-id"},
		{"tasks", "restore", "fake-id"},
		{"tasks", "update", "fake-id", "--title", "new"},
		{"projects", "list"},
		{"tags", "list"},
	}
	for _, args := range commands {
		code := Run(args)
		if code != 1 {
			t.Errorf("command %v: expected exit code 1 (SP unavailable), got %d", args, code)
		}
	}
}

// Test all commands that require missing args produce exit 2.
func TestRun_MissingArgs_ExitCode2(t *testing.T) {
	commands := [][]string{
		{"tasks", "get"},
		{"tasks", "add"},
		{"tasks", "complete"},
		{"tasks", "uncomplete"},
		{"tasks", "start"},
		{"tasks", "archive"},
		{"tasks", "restore"},
		{"tasks", "set-current"},
		{"tasks", "update"},
		{"tasks", "update", "id"},
	}
	for _, args := range commands {
		code := Run(args)
		if code != 2 {
			t.Errorf("command %v: expected exit code 2 (usage error), got %d", args, code)
		}
	}
}

// Test unknown subcommands produce exit 2.
func TestRun_UnknownSubcommands_ExitCode2(t *testing.T) {
	commands := [][]string{
		{"unknown"},
		{"tasks", "delete"},
		{"tasks", "unknown"},
		{"projects", "create"},
		{"tags", "delete"},
	}
	for _, args := range commands {
		code := Run(args)
		if code != 2 {
			t.Errorf("command %v: expected exit code 2, got %d", args, code)
		}
	}
}

// Test tasks default subcommand is 'list'.
func TestRun_TasksDefault_IsList(t *testing.T) {
	t.Setenv("SP_BASE_URL", "http://127.0.0.1:1")
	// "tasks" with no subcommand should behave like "tasks list"
	// It will fail with exit 1 (SP unavailable), not 2 (usage error)
	code := Run([]string{"tasks"})
	if code != 1 {
		t.Errorf("expected exit code 1 for 'tasks' (defaults to list), got %d", code)
	}
}

// Test task update with valid flags.
func TestRun_TaskUpdate_ValidFlags(t *testing.T) {
	t.Setenv("SP_BASE_URL", "http://127.0.0.1:1")
	// Should fail with 1 (SP unavailable) not 2 (parse error)
	code := Run([]string{"tasks", "update", "task-1", "--title", "New Title"})
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
}

// Test task update with --done and --not-done.
func TestRun_TaskUpdate_DoneFlags(t *testing.T) {
	t.Setenv("SP_BASE_URL", "http://127.0.0.1:1")
	code := Run([]string{"tasks", "update", "task-1", "--done"})
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	code = Run([]string{"tasks", "update", "task-1", "--not-done"})
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
}

// Test task update unknown flag.
func TestRun_TaskUpdate_UnknownFlag(t *testing.T) {
	code := Run([]string{"tasks", "update", "task-1", "--bogus"})
	if code != 2 {
		t.Errorf("expected exit code 2 for unknown flag, got %d", code)
	}
}

// Test task update missing flag value.
func TestRun_TaskUpdate_MissingFlagValue(t *testing.T) {
	flags := []string{"--title", "--notes", "--project-id", "--due-day", "--time-estimate", "--time-spent"}
	for _, flag := range flags {
		code := Run([]string{"tasks", "update", "task-1", flag})
		if code != 2 {
			t.Errorf("flag %s without value: expected exit code 2, got %d", flag, code)
		}
	}
}

// Test tasks list with valid filters passes through.
func TestRun_TasksList_Filters(t *testing.T) {
	t.Setenv("SP_BASE_URL", "http://127.0.0.1:1")
	code := Run([]string{"tasks", "list", "--query", "hello", "--include-done", "--source", "all"})
	if code != 1 {
		t.Errorf("expected exit code 1 (SP down), got %d", code)
	}
}

// Test tasks list with invalid filter.
func TestRun_TasksList_InvalidFilter(t *testing.T) {
	code := Run([]string{"tasks", "list", "--bogus"})
	if code != 2 {
		t.Errorf("expected exit code 2 for invalid filter, got %d", code)
	}
}

// Test projects list with query.
func TestRun_ProjectsList_Query(t *testing.T) {
	t.Setenv("SP_BASE_URL", "http://127.0.0.1:1")
	code := Run([]string{"projects", "list", "--query", "work"})
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
}

// Test empty args shows usage (exit 0).
func TestRun_Help(t *testing.T) {
	// Redirect stdout to avoid noise
	old := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = old }()

	code := Run(nil)
	if code != 0 {
		t.Errorf("expected exit code 0 for help, got %d", code)
	}
}
