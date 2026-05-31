package hostcfg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// withTempHome sets HOME to a temp dir and returns a cleanup func.
func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

// testConfigPath returns the platform-aware config path for a host,
// using the same logic as the production code.
func testConfigPath(home, hostName string) string {
	switch hostName {
	case HostClaudeDesktop:
		switch runtime.GOOS {
		case "darwin":
			return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
		default:
			return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json")
		}
	case HostVSCodeCopilot:
		switch runtime.GOOS {
		case "darwin":
			return filepath.Join(home, "Library", "Application Support", "Code", "User", "mcp.json")
		default:
			return filepath.Join(home, ".config", "Code", "User", "mcp.json")
		}
	case HostCodex:
		return filepath.Join(home, ".codex", "config.toml")
	default:
		return ""
	}
}

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

func TestSurgicalTOMLWrite_RemoveDescendantTables(t *testing.T) {
	original := strings.SplitAfter("[mcp_servers.superProductivity]\ncommand = '/path'\n\n[mcp_servers.superProductivity.env]\nFOO = 'bar'\n\n[other.section]\nfoo = 'bar'\n", "\n")
	result := surgicalTOMLWrite(original, "mcp_servers", "superProductivity", nil)
	got := strings.Join(result, "")
	if strings.Contains(got, "superProductivity") {
		t.Fatalf("entry and descendants should be removed, got: %s", got)
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
	if !strings.Contains(got, `command = "/usr/bin/sp-local-bridge"`) {
		t.Fatalf("expected TOML basic string, got: %s", got)
	}
	if !strings.Contains(got, `args = ["mcp"]`) {
		t.Fatalf("expected args array, got: %s", got)
	}
}

func TestFormatTOMLEntry_EscapesSpecialChars(t *testing.T) {
	entry := map[string]any{
		"command": `/home/o'brien/bin/sp-bridge`,
		"args":    []string{`--path="foo"`},
	}
	got := formatTOMLEntry(entry)
	if !strings.Contains(got, `command = "/home/o'brien/bin/sp-bridge"`) {
		t.Fatalf("apostrophe should pass through in basic string, got: %s", got)
	}
	if !strings.Contains(got, `--path=\"foo\"`) {
		t.Fatalf("double quotes should be escaped, got: %s", got)
	}
}

// --- TOML validation tests ---

func TestValidateTOMLStructure_Valid(t *testing.T) {
	content := "[mcp_servers.superProductivity]\ncommand = '/usr/bin/sp-local-bridge'\nargs = ['mcp']\n\n[other]\nfoo = 'bar'\n"
	if err := validateTOMLStructure(content); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateTOMLStructure_Comments(t *testing.T) {
	content := "# this is a comment\n[section]\nkey = 'value'\n"
	if err := validateTOMLStructure(content); err != nil {
		t.Fatalf("expected valid with comments, got: %v", err)
	}
}

func TestValidateTOMLStructure_UnclosedHeader(t *testing.T) {
	content := "[unclosed\nkey = 'value'\n"
	if err := validateTOMLStructure(content); err == nil {
		t.Fatal("expected error for unclosed table header")
	}
}

func TestValidateTOMLStructure_NoEquals(t *testing.T) {
	content := "[section]\nthis is not valid\n"
	if err := validateTOMLStructure(content); err == nil {
		t.Fatal("expected error for line without =")
	}
}

func TestValidateTOMLStructure_EmptyHeader(t *testing.T) {
	content := "[]\nkey = 'value'\n"
	if err := validateTOMLStructure(content); err == nil {
		t.Fatal("expected error for empty table header")
	}
}

// --- Public function tests ---

func TestRunConfigure_UnknownHost(t *testing.T) {
	code := RunConfigure([]string{"nonexistent-host"})
	if code != 2 {
		t.Fatalf("expected exit 2 for unknown host, got %d", code)
	}
}

func TestRunConfigure_NoArgs(t *testing.T) {
	code := RunConfigure(nil)
	if code != 2 {
		t.Fatalf("expected exit 2 for no args, got %d", code)
	}
}

func TestRunConfigure_Help(t *testing.T) {
	code := RunConfigure([]string{"--help"})
	if code != 0 {
		t.Fatalf("expected exit 0 for --help, got %d", code)
	}
}

func TestRunConfigure_DryRunClaude(t *testing.T) {
	home := withTempHome(t)
	_ = home
	code := RunConfigure([]string{"--dry-run", "claude-desktop"})
	if code != 0 {
		t.Fatalf("expected exit 0 for dry-run, got %d", code)
	}
	// Verify no file was written
	configPath := testConfigPath(home, HostClaudeDesktop)
	if _, err := os.Stat(configPath); err == nil {
		t.Fatal("dry-run should not create config file")
	}
}

func TestRunConfigure_DryRunVSCode(t *testing.T) {
	home := withTempHome(t)
	code := RunConfigure([]string{"--dry-run", "vscode-copilot"})
	if code != 0 {
		t.Fatalf("expected exit 0 for dry-run, got %d", code)
	}
	configPath := testConfigPath(home, HostVSCodeCopilot)
	if _, err := os.Stat(configPath); err == nil {
		t.Fatal("dry-run should not create config file")
	}
}

func TestRunConfigure_DryRunCodex(t *testing.T) {
	home := withTempHome(t)
	code := RunConfigure([]string{"--dry-run", "codex"})
	if code != 0 {
		t.Fatalf("expected exit 0 for dry-run, got %d", code)
	}
	configPath := testConfigPath(home, HostCodex)
	if _, err := os.Stat(configPath); err == nil {
		t.Fatal("dry-run should not create config file")
	}
}

func TestRunConfigure_Claude_WritesJSON(t *testing.T) {
	home := withTempHome(t)
	code := RunConfigure([]string{"claude-desktop"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	configPath := testConfigPath(home, HostClaudeDesktop)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	servers, _ := obj["mcpServers"].(map[string]any)
	if servers == nil || servers["super-productivity"] == nil {
		t.Fatalf("expected mcpServers.super-productivity, got: %v", obj)
	}
}

func TestRunConfigure_VSCode_WritesJSON(t *testing.T) {
	home := withTempHome(t)
	code := RunConfigure([]string{"vscode-copilot"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	configPath := testConfigPath(home, HostVSCodeCopilot)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	servers, _ := obj["servers"].(map[string]any)
	if servers == nil || servers["superProductivity"] == nil {
		t.Fatalf("expected servers.superProductivity, got: %v", obj)
	}
	entry, _ := servers["superProductivity"].(map[string]any)
	if entry["type"] != "stdio" {
		t.Fatalf("expected type=stdio, got: %v", entry)
	}
}

func TestRunConfigure_Codex_WritesTOML(t *testing.T) {
	home := withTempHome(t)
	code := RunConfigure([]string{"codex"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	configPath := testConfigPath(home, HostCodex)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "[mcp_servers.superProductivity]") {
		t.Fatalf("expected TOML header, got: %s", content)
	}
	if !strings.Contains(content, `command = "`) {
		t.Fatalf("expected command entry, got: %s", content)
	}
}

func TestRunConfigure_JSON_CreatesBackup(t *testing.T) {
	home := withTempHome(t)
	configPath := testConfigPath(home, HostClaudeDesktop)
	os.MkdirAll(filepath.Dir(configPath), 0o755)
	os.WriteFile(configPath, []byte(`{"existing":"data"}`), 0o644)

	code := RunConfigure([]string{"claude-desktop"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	bakData, err := os.ReadFile(configPath + ".bak")
	if err != nil {
		t.Fatalf("expected backup: %v", err)
	}
	if string(bakData) != `{"existing":"data"}` {
		t.Fatalf("backup content wrong: %s", string(bakData))
	}
}

func TestRunConfigure_TOML_CreatesBackup(t *testing.T) {
	home := withTempHome(t)
	configPath := testConfigPath(home, HostCodex)
	os.MkdirAll(filepath.Dir(configPath), 0o755)
	os.WriteFile(configPath, []byte("[other]\nkey = 'val'\n"), 0o644)

	code := RunConfigure([]string{"codex"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	bakData, err := os.ReadFile(configPath + ".bak")
	if err != nil {
		t.Fatalf("expected backup: %v", err)
	}
	if !strings.Contains(string(bakData), "[other]") {
		t.Fatalf("backup should contain original content: %s", string(bakData))
	}
}

func TestRunConfigure_MalformedJSON_FailsClosed(t *testing.T) {
	home := withTempHome(t)
	configPath := testConfigPath(home, HostClaudeDesktop)
	os.MkdirAll(filepath.Dir(configPath), 0o755)
	os.WriteFile(configPath, []byte(`{not valid json!!!`), 0o644)

	code := RunConfigure([]string{"claude-desktop"})
	if code != 1 {
		t.Fatalf("expected exit 1 for malformed JSON, got %d", code)
	}
}

func TestRunConfigure_MalformedTOML_FailsClosed(t *testing.T) {
	home := withTempHome(t)
	configPath := testConfigPath(home, HostCodex)
	os.MkdirAll(filepath.Dir(configPath), 0o755)
	os.WriteFile(configPath, []byte("[unclosed bracket\n"), 0o644)

	code := RunConfigure([]string{"codex"})
	if code != 1 {
		t.Fatalf("expected exit 1 for malformed TOML, got %d", code)
	}
}

func TestRunConfigure_Remove_JSON(t *testing.T) {
	home := withTempHome(t)
	configPath := testConfigPath(home, HostClaudeDesktop)
	os.MkdirAll(filepath.Dir(configPath), 0o755)
	os.WriteFile(configPath, []byte(`{"mcpServers":{"super-productivity":{"command":"x","args":["mcp"]}}}`), 0o644)

	code := RunConfigure([]string{"--remove", "claude-desktop"})
	if code != 0 {
		t.Fatalf("expected exit 0 for remove, got %d", code)
	}
	data, _ := os.ReadFile(configPath)
	if strings.Contains(string(data), "super-productivity") {
		t.Fatalf("entry should be removed: %s", string(data))
	}
}

func TestRunConfigure_Remove_TOML(t *testing.T) {
	home := withTempHome(t)
	configPath := testConfigPath(home, HostCodex)
	os.MkdirAll(filepath.Dir(configPath), 0o755)
	os.WriteFile(configPath, []byte("[mcp_servers.superProductivity]\ncommand = '/x'\nargs = ['mcp']\n\n[other]\nkey = 'val'\n"), 0o644)

	code := RunConfigure([]string{"--remove", "codex"})
	if code != 0 {
		t.Fatalf("expected exit 0 for remove, got %d", code)
	}
	data, _ := os.ReadFile(configPath)
	if strings.Contains(string(data), "superProductivity") {
		t.Fatalf("entry should be removed: %s", string(data))
	}
	if !strings.Contains(string(data), "[other]") {
		t.Fatalf("other section should be preserved: %s", string(data))
	}
}

func TestRunConfigure_Remove_MalformedTOML_FailsClosed(t *testing.T) {
	home := withTempHome(t)
	configPath := testConfigPath(home, HostCodex)
	os.MkdirAll(filepath.Dir(configPath), 0o755)
	os.WriteFile(configPath, []byte("[bad header\ncommand = 'x'\n"), 0o644)

	code := RunConfigure([]string{"--remove", "codex"})
	if code != 1 {
		t.Fatalf("expected exit 1 for malformed TOML on remove, got %d", code)
	}
}

func TestRunPrintConfig_UnknownHost(t *testing.T) {
	code := RunPrintConfig([]string{"nonexistent"})
	if code != 2 {
		t.Fatalf("expected exit 2 for unknown host, got %d", code)
	}
}

func TestRunPrintConfig_NoArgs(t *testing.T) {
	code := RunPrintConfig(nil)
	if code != 2 {
		t.Fatalf("expected exit 2 for no args, got %d", code)
	}
}

func TestRunPrintConfig_Bare_Codex(t *testing.T) {
	code := RunPrintConfig([]string{"--bare", "codex"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestRunPrintConfig_Claude(t *testing.T) {
	code := RunPrintConfig([]string{"claude-desktop"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestRunPrintConfig_VSCode(t *testing.T) {
	code := RunPrintConfig([]string{"vscode-copilot"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}
