package hostcfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSurgicalTOMLWrite_AddNew(t *testing.T) {
	original := []string{}
	content := "command = '/usr/bin/sp-local-bridge'\nargs = ['mcp']"
	result := surgicalTOMLWrite(original, "mcp_servers", "superProductivity", &content)
	got := strings.Join(result, "")
	if !strings.Contains(got, "[mcp_servers.superProductivity]") {
		t.Fatalf("expected header, got: %s", got)
	}
	if !strings.Contains(got, "command = '/usr/bin/sp-local-bridge'") {
		t.Fatalf("expected command, got: %s", got)
	}
}

func TestSurgicalTOMLWrite_ReplaceExisting(t *testing.T) {
	original := strings.SplitAfter("[mcp_servers.superProductivity]\ncommand = '/old/path'\nargs = ['mcp']\n\n[other.section]\nfoo = 'bar'\n", "\n")
	content := "command = '/new/path'\nargs = ['mcp']"
	result := surgicalTOMLWrite(original, "mcp_servers", "superProductivity", &content)
	got := strings.Join(result, "")
	if strings.Contains(got, "/old/path") {
		t.Fatalf("old entry should be replaced, got: %s", got)
	}
	if !strings.Contains(got, "/new/path") {
		t.Fatalf("expected new path, got: %s", got)
	}
	if !strings.Contains(got, "[other.section]") {
		t.Fatalf("other section should be preserved, got: %s", got)
	}
}

func TestSurgicalTOMLWrite_Remove(t *testing.T) {
	original := strings.SplitAfter("[mcp_servers.superProductivity]\ncommand = '/some/path'\nargs = ['mcp']\n\n[other.section]\nfoo = 'bar'\n", "\n")
	result := surgicalTOMLWrite(original, "mcp_servers", "superProductivity", nil)
	got := strings.Join(result, "")
	if strings.Contains(got, "superProductivity") {
		t.Fatalf("entry should be removed, got: %s", got)
	}
	if !strings.Contains(got, "[other.section]") {
		t.Fatalf("other section should be preserved, got: %s", got)
	}
}

func TestReadJSON_NonExistent(t *testing.T) {
	result, err := readJSON("/tmp/nonexistent-sp-test-file.json")
	if err != nil {
		t.Fatalf("non-existent should return empty map, got err: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty map, got: %v", result)
	}
}

func TestReadJSON_Valid(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.json")
	os.WriteFile(tmp, []byte(`{"servers":{"foo":{"command":"bar"}}}`), 0o644)
	result, err := readJSON(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	servers, ok := result["servers"].(map[string]any)
	if !ok {
		t.Fatalf("expected servers map, got: %v", result)
	}
	if servers["foo"] == nil {
		t.Fatalf("expected foo entry")
	}
}

func TestReadJSON_Malformed(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(tmp, []byte(`{not valid json`), 0o644)
	_, err := readJSON(tmp)
	if err == nil {
		t.Fatalf("expected error for malformed JSON")
	}
}

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "test.txt")
	err := atomicWrite(path, "hello world\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "hello world\n" {
		t.Fatalf("expected 'hello world\\n', got: %q", string(got))
	}
}

func TestBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte("original"), 0o644)
	backup(path)
	bakData, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("expected backup file: %v", err)
	}
	if string(bakData) != "original" {
		t.Fatalf("expected 'original', got: %q", string(bakData))
	}
}

func TestBuildEntry_VSCodeHasType(t *testing.T) {
	entry := buildEntry(HostVSCodeCopilot)
	if entry["type"] != "stdio" {
		t.Fatalf("vscode-copilot entry should have type=stdio, got: %v", entry)
	}
}

func TestBuildEntry_ClaudeNoType(t *testing.T) {
	entry := buildEntry(HostClaudeDesktop)
	if _, ok := entry["type"]; ok {
		t.Fatalf("claude-desktop entry should not have type field, got: %v", entry)
	}
}

func TestFormatTOMLEntry(t *testing.T) {
	entry := map[string]any{
		"command": "/usr/bin/sp-local-bridge",
		"args":    []string{"mcp"},
	}
	got := formatTOMLEntry(entry)
	if !strings.Contains(got, "command = '/usr/bin/sp-local-bridge'") {
		t.Fatalf("expected TOML literal string, got: %s", got)
	}
	if !strings.Contains(got, "args = ['mcp']") {
		t.Fatalf("expected args array, got: %s", got)
	}
}
