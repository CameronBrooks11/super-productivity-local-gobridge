package hostcfg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// withTempHome sets HOME to a temp dir and returns a cleanup func.
// On Windows, also sets APPDATA so that config path resolution works.
func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	if runtime.GOOS == "windows" {
		appData := filepath.Join(dir, "AppData", "Roaming")
		os.MkdirAll(appData, 0o755)
		t.Setenv("APPDATA", appData)
	}
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
		case "windows":
			return filepath.Join(home, "AppData", "Roaming", "Claude", "claude_desktop_config.json")
		default:
			return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json")
		}
	case HostVSCodeCopilot:
		switch runtime.GOOS {
		case "darwin":
			return filepath.Join(home, "Library", "Application Support", "Code", "User", "mcp.json")
		case "windows":
			return filepath.Join(home, "AppData", "Roaming", "Code", "User", "mcp.json")
		default:
			return filepath.Join(home, ".config", "Code", "User", "mcp.json")
		}
	case HostClaudeCode:
		return filepath.Join(home, ".claude.json")
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

func TestRunPrintConfig_ClaudeCode(t *testing.T) {
	code := RunPrintConfig([]string{"claude-code"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestBuildEntry_ClaudeCodeHasType(t *testing.T) {
	entry := buildEntry(HostClaudeCode)
	if entry["type"] != "stdio" {
		t.Fatalf("claude-code entry should have type=stdio, got: %v", entry)
	}
}

func TestRunConfigure_DryRunClaudeCode(t *testing.T) {
	withTempHome(t)
	if code := RunConfigure([]string{"--dry-run", "claude-code"}); code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestRunConfigure_ClaudeCode_WritesJSON(t *testing.T) {
	home := withTempHome(t)
	if code := RunConfigure([]string{"claude-code"}); code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	configPath := testConfigPath(home, HostClaudeCode)
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
	entry, _ := servers["super-productivity"].(map[string]any)
	if entry["type"] != "stdio" {
		t.Fatalf("expected type=stdio, got: %v", entry)
	}
}

func TestRunConfigure_ClaudeCode_RemoveRoundTrip(t *testing.T) {
	home := withTempHome(t)
	if code := RunConfigure([]string{"claude-code"}); code != 0 {
		t.Fatalf("configure failed: %d", code)
	}
	if code := RunConfigure([]string{"--remove", "claude-code"}); code != 0 {
		t.Fatalf("remove failed: %d", code)
	}
	data, _ := os.ReadFile(testConfigPath(home, HostClaudeCode))
	var obj map[string]any
	json.Unmarshal(data, &obj)
	if servers, ok := obj["mcpServers"].(map[string]any); ok && servers["super-productivity"] != nil {
		t.Fatalf("entry should be gone, got: %v", obj)
	}
}

// Claude Code's config carries unrelated state that we must not disturb while
// adding one key. A float64 JSON round-trip would silently rewrite integers
// above 2^53, so readJSON keeps numbers as their original literal.
func TestConfigure_PreservesForeignKeysAndLargeIntegers(t *testing.T) {
	home := withTempHome(t)
	configPath := testConfigPath(home, HostClaudeCode)
	original := `{"numStartups":376,"bigId":9007199254740993,"firstStartTime":1788567895339,` +
		`"ratio":1.5,"mcpServers":{"existing":{"command":"keep-me"}}}`
	if err := os.WriteFile(configPath, []byte(original), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if code := RunConfigure([]string{"claude-code"}); code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read back failed: %v", err)
	}
	text := string(data)
	for _, want := range []string{"9007199254740993", "1788567895339", "376", "1.5", "keep-me"} {
		if !strings.Contains(text, want) {
			t.Errorf("value %q not preserved verbatim; got:\n%s", want, text)
		}
	}
}

func TestReadJSON_PreservesIntegerLiterals(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "big.json")
	os.WriteFile(tmp, []byte(`{"big":9007199254740993}`), 0o644)
	result, err := readJSON(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	num, ok := result["big"].(json.Number)
	if !ok {
		t.Fatalf("expected json.Number, got %T", result["big"])
	}
	if num.String() != "9007199254740993" {
		t.Fatalf("integer literal not preserved: %s", num.String())
	}
}

// json.Decoder stops at the first value, so without an explicit check a file
// corrupted by a doubled write would parse as its leading object and then be
// rewritten with the remainder discarded.
func TestReadJSON_RejectsTrailingData(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "doubled.json")
	os.WriteFile(tmp, []byte(`{"a":1}{"b":2}`), 0o644)
	if _, err := readJSON(tmp); err == nil {
		t.Fatal("expected an error for data after the top-level value")
	}
}

// Host configs can hold account identifiers, so the .bak must not be more
// permissive than the file it copies.
func TestBackup_PreservesMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not meaningful on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	backup(path)
	fi, err := os.Stat(path + ".bak")
	if err != nil {
		t.Fatalf("backup not created: %v", err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Fatalf("backup widened permissions: got %v, want -rw-------", got)
	}
}

func TestConcurrentModification_DetectsChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.json")
	os.WriteFile(path, []byte(`{"a":1}`), 0o644)
	before := fingerprint(path)
	if concurrentModification(path, before) {
		t.Fatal("unchanged file reported as modified")
	}
	os.WriteFile(path, []byte(`{"a":1,"b":2}`), 0o644)
	if !concurrentModification(path, before) {
		t.Fatal("changed file not detected")
	}
}

func TestAppDataDir_FallsBackWhenUnset(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("APPDATA", "")
	got := appDataDir()
	if !filepath.IsAbs(got) {
		t.Fatalf("fallback must be absolute, got %q", got)
	}
	if want := filepath.Join(home, "AppData", "Roaming"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestTomlHasEntryValid(t *testing.T) {
	data := "[mcp_servers.superProductivity]\ncommand = \"sp-local-bridge\"\nargs = [\"mcp\"]\n"
	if !tomlHasEntry(data, "mcp_servers", "superProductivity") {
		t.Error("expected true for valid TOML entry")
	}
}

func TestTomlHasEntryNoCommand(t *testing.T) {
	// Has the header but no command key
	data := "[mcp_servers.superProductivity]\nargs = [\"mcp\"]\n"
	if tomlHasEntry(data, "mcp_servers", "superProductivity") {
		t.Error("expected false when command key is missing")
	}
}

func TestTomlHasEntryInComment(t *testing.T) {
	// Header exists only in a comment
	data := "# [mcp_servers.superProductivity]\ncommand = \"sp-local-bridge\"\n"
	if tomlHasEntry(data, "mcp_servers", "superProductivity") {
		t.Error("expected false when header is in a comment")
	}
}

func TestTomlHasEntryDifferentSection(t *testing.T) {
	// Command is in a different section
	data := "[other.section]\ncommand = \"other\"\n\n[mcp_servers.superProductivity]\nargs = [\"mcp\"]\n"
	if tomlHasEntry(data, "mcp_servers", "superProductivity") {
		t.Error("expected false when command is in wrong section")
	}
}

func TestTomlHasEntrySubstring(t *testing.T) {
	// Header is a substring but not at line start
	data := "x = \"[mcp_servers.superProductivity]\"\ncommand = \"sp-local-bridge\"\n"
	if tomlHasEntry(data, "mcp_servers", "superProductivity") {
		t.Error("expected false when header is inside a string value")
	}
}

// --- Detection ---

// writeHostConfig writes content to a host's config path under a temp HOME,
// creating the parent directory.
func writeHostConfig(t *testing.T, home, hostName, content string) {
	t.Helper()
	path := testConfigPath(home, hostName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// detectionFor returns the Detection for one host, failing if it is absent.
func detectionFor(t *testing.T, hostName string) Detection {
	t.Helper()
	for _, d := range DetectConfigured() {
		if d.Host == hostName {
			return d
		}
	}
	t.Fatalf("no detection reported for host %q", hostName)
	return Detection{}
}

func TestDetectConfigured_CoversEveryHost(t *testing.T) {
	withTempHome(t)
	got := DetectConfigured()
	if len(got) != len(sortedHostNames()) {
		t.Fatalf("expected one detection per host (%d), got %d", len(sortedHostNames()), len(got))
	}
	for _, d := range got {
		if d.Configured {
			t.Errorf("host %q reported configured under an empty HOME", d.Host)
		}
		if d.Scope != "" {
			t.Errorf("host %q reported scope %q while not configured", d.Host, d.Scope)
		}
	}
}

func TestDetectConfigured_UserScope(t *testing.T) {
	home := withTempHome(t)
	writeHostConfig(t, home, HostClaudeCode,
		`{"mcpServers":{"super-productivity":{"command":"sp-local-bridge","args":["mcp"]}}}`)

	d := detectionFor(t, HostClaudeCode)
	if !d.Configured {
		t.Fatal("expected claude-code to be detected")
	}
	if d.Scope != ScopeUser {
		t.Errorf("expected scope %q, got %q", ScopeUser, d.Scope)
	}
	if len(d.Projects) != 0 {
		t.Errorf("expected no projects, got %v", d.Projects)
	}
}

// A server added with `claude mcp add` lands in local scope by default, which
// is stored per project rather than in the top-level map. Reading only the top
// level reported "none configured" for a host that was configured and
// connected.
func TestDetectConfigured_LocalScopeOnly(t *testing.T) {
	home := withTempHome(t)
	writeHostConfig(t, home, HostClaudeCode,
		`{"projects":{"/home/u/repo":{"mcpServers":{"super-productivity":{"command":"sp-local-bridge"}}}}}`)

	d := detectionFor(t, HostClaudeCode)
	if !d.Configured {
		t.Fatal("expected a local-scope entry to count as configured")
	}
	if d.Scope != ScopeLocal {
		t.Errorf("expected scope %q, got %q", ScopeLocal, d.Scope)
	}
	if len(d.Projects) != 1 || d.Projects[0] != "/home/u/repo" {
		t.Errorf("expected [/home/u/repo], got %v", d.Projects)
	}
}

// User scope answers "configured everywhere", so it wins the scope label, but
// the projects are still reported: they are where the entry is pinned.
func TestDetectConfigured_UserScopeWinsOverLocal(t *testing.T) {
	home := withTempHome(t)
	writeHostConfig(t, home, HostClaudeCode, `{
		"mcpServers":{"super-productivity":{"command":"sp-local-bridge"}},
		"projects":{"/home/u/repo":{"mcpServers":{"super-productivity":{"command":"sp-local-bridge"}}}}
	}`)

	d := detectionFor(t, HostClaudeCode)
	if d.Scope != ScopeUser {
		t.Errorf("expected scope %q, got %q", ScopeUser, d.Scope)
	}
	if len(d.Projects) != 1 {
		t.Errorf("expected the local-scope project to still be reported, got %v", d.Projects)
	}
}

func TestDetectConfigured_ProjectsSorted(t *testing.T) {
	home := withTempHome(t)
	writeHostConfig(t, home, HostClaudeCode, `{"projects":{
		"/b":{"mcpServers":{"super-productivity":{"command":"x"}}},
		"/a":{"mcpServers":{"super-productivity":{"command":"x"}}},
		"/c":{"mcpServers":{"other":{"command":"x"}}}
	}}`)

	d := detectionFor(t, HostClaudeCode)
	want := []string{"/a", "/b"}
	if len(d.Projects) != len(want) {
		t.Fatalf("expected %v, got %v", want, d.Projects)
	}
	for i := range want {
		if d.Projects[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, d.Projects)
		}
	}
}

// Only Claude Code records per-project servers. A "projects" key in another
// host's config must not be read as one.
func TestDetectConfigured_LocalScopeIsClaudeCodeOnly(t *testing.T) {
	home := withTempHome(t)
	writeHostConfig(t, home, HostClaudeDesktop,
		`{"projects":{"/home/u/repo":{"mcpServers":{"super-productivity":{"command":"x"}}}}}`)

	d := detectionFor(t, HostClaudeDesktop)
	if d.Configured {
		t.Error("claude-desktop has no local scope; the entry should not be detected")
	}
}

func TestDetectConfigured_TOMLHost(t *testing.T) {
	home := withTempHome(t)
	writeHostConfig(t, home, HostCodex,
		"[mcp_servers.superProductivity]\ncommand = \"sp-local-bridge\"\n")

	d := detectionFor(t, HostCodex)
	if !d.Configured || d.Scope != ScopeUser {
		t.Errorf("expected codex configured at user scope, got %+v", d)
	}
}

// Detection is a diagnostic: an unreadable config must not stop the remaining
// hosts from being reported.
func TestDetectConfigured_MalformedJSONIsNotConfigured(t *testing.T) {
	home := withTempHome(t)
	writeHostConfig(t, home, HostClaudeCode, `{"mcpServers":`)
	writeHostConfig(t, home, HostCodex,
		"[mcp_servers.superProductivity]\ncommand = \"sp-local-bridge\"\n")

	if d := detectionFor(t, HostClaudeCode); d.Configured {
		t.Error("malformed JSON should not count as configured")
	}
	if d := detectionFor(t, HostCodex); !d.Configured {
		t.Error("a malformed config for one host hid another host's entry")
	}
}

func TestRunConfigure_Status_NoHostArgument(t *testing.T) {
	withTempHome(t)
	if code := RunConfigure([]string{"--status"}); code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestRunConfigure_Status_ReportsConfiguredHost(t *testing.T) {
	home := withTempHome(t)
	writeHostConfig(t, home, HostCodex,
		"[mcp_servers.superProductivity]\ncommand = \"sp-local-bridge\"\n")

	out := captureStdout(t, func() {
		if code := RunConfigure([]string{"--status"}); code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
	})

	if !statusLineContains(out, HostCodex, "configured (user scope)") {
		t.Errorf("expected codex reported as configured at user scope, got:\n%s", out)
	}
	// Hosts that are not configured have to appear too: the question the
	// command answers is "which of these still needs configure?".
	if !statusLineContains(out, HostClaudeDesktop, "not configured") {
		t.Errorf("expected claude-desktop reported as not configured, got:\n%s", out)
	}
}

// statusLineContains reports whether the status line naming host also contains
// want, so assertions do not depend on column padding.
func statusLineContains(out, host, want string) bool {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == host {
			return strings.Contains(line, want)
		}
	}
	return false
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what it
// wrote. The report is the whole point of --status, so asserting on the exit
// status alone would not test it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()
	w.Close()
	return <-done
}

// --status runs before the mutating paths, so it has to reject rather than
// quietly discard them: `configure --remove --status <host>` returning 0 with
// the entry still in place is a destructive intent silently dropped.
func TestRunConfigure_Status_RejectsRemove(t *testing.T) {
	home := withTempHome(t)
	writeHostConfig(t, home, HostClaudeDesktop,
		`{"mcpServers":{"super-productivity":{"command":"sp-local-bridge"}}}`)

	if code := RunConfigure([]string{"--remove", "--status", "claude-desktop"}); code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}

	d := detectionFor(t, HostClaudeDesktop)
	if !d.Configured {
		t.Error("the entry was removed despite the command being rejected")
	}
}

func TestRunConfigure_Status_RejectsDryRun(t *testing.T) {
	withTempHome(t)
	if code := RunConfigure([]string{"--status", "--dry-run"}); code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

// --status reports on every host, so accepting a host argument would let
// `configure --status <host> | grep configured` succeed on a different host.
func TestRunConfigure_Status_RejectsHostArgument(t *testing.T) {
	withTempHome(t)
	if code := RunConfigure([]string{"--status", "claude-code"}); code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

// install.sh no longer carries its own copy of the host list, so the invitation
// has to name them whether or not anything is configured yet. Asserting that
// each name appears somewhere in the output would prove nothing: the table
// above lists every host either way. The assertion is on the invitation line.
func TestRunConfigure_Status_AlwaysNamesSupportedHosts(t *testing.T) {
	want := "Supported hosts: " + strings.Join(sortedHostNames(), ", ")

	for _, tc := range []struct {
		name      string
		configure bool
	}{
		{"nothing configured", false},
		{"one host configured", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := withTempHome(t)
			if tc.configure {
				writeHostConfig(t, home, HostCodex,
					"[mcp_servers.superProductivity]\ncommand = \"sp-local-bridge\"\n")
			}
			out := captureStdout(t, func() { RunConfigure([]string{"--status"}) })
			if !strings.Contains(out, want) {
				t.Errorf("expected %q in the invitation, got:\n%s", want, out)
			}
		})
	}
}

// captureStdio runs fn with stdout and stderr redirected, returning what each
// received. print-config writes its payload to stdout and errors to stderr, so
// a test that only checked the exit code could not tell a rejected flag from a
// printed config.
func captureStdio(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()

	fn()

	outW.Close()
	errW.Close()
	var outBuf, errBuf bytes.Buffer
	if _, err := io.Copy(&outBuf, outR); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if _, err := io.Copy(&errBuf, errR); err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return outBuf.String(), errBuf.String()
}

// TestRunConfigure_UnknownFlag_WritesNothing is the regression guard for #53.
// The exit code is not the point: `configure --dry-runn <host>` exited 0 and
// wrote the config for real, so the assertion that matters is the absence of
// the file the user was told they would only preview.
func TestRunConfigure_UnknownFlag_WritesNothing(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"typo for --dry-run", []string{"--dry-runn", "claude-desktop"}},
		{"bad flag before the host", []string{"--bogus", "claude-desktop"}},
		{"bad flag after the host", []string{"claude-desktop", "--bogus"}},
		{"short bad flag", []string{"-x", "claude-desktop"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := withTempHome(t)
			var code int
			_, stderr := captureStdio(t, func() { code = RunConfigure(tc.args) })
			// Errorf, not Fatalf: under the bug this guards, the exit code is 0
			// and a Fatalf here would return before the assertion that actually
			// matters. The file check is the point of the test.
			if code != 2 {
				t.Errorf("expected exit 2, got %d", code)
			}
			if !strings.Contains(stderr, "unknown flag") {
				t.Errorf("stderr should name the problem, got %q", stderr)
			}
			configPath := testConfigPath(home, HostClaudeDesktop)
			if _, err := os.Stat(configPath); err == nil {
				t.Fatalf("a rejected command wrote %s", configPath)
			}
		})
	}
}

// TestUnknownFlag_NamesTheFirstOne pins the choice doctor made: naming the last
// bad flag sends the user round the loop once per typo.
//
// Both commands are covered because both keep their own copy of the loop. A
// mutation sweep that flipped only print-config's copy to keep the last flag
// left the suite green, because the first version of this test called
// RunConfigure alone.
func TestUnknownFlag_NamesTheFirstOne(t *testing.T) {
	cases := []struct {
		name string
		run  func([]string) int
		args []string
	}{
		{"configure", RunConfigure, []string{"--aaa", "--zzz", "claude-desktop"}},
		{"print-config", RunPrintConfig, []string{"--aaa", "--zzz", "claude-code"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withTempHome(t)
			var code int
			_, stderr := captureStdio(t, func() { code = tc.run(tc.args) })
			if code != 2 {
				t.Fatalf("expected exit 2, got %d", code)
			}
			if !strings.Contains(stderr, "--aaa") {
				t.Errorf("expected the first bad flag in stderr, got %q", stderr)
			}
			if strings.Contains(stderr, "--zzz") {
				t.Errorf("expected only the first bad flag, but stderr named --zzz too: %q", stderr)
			}
		})
	}
}

// TestRunConfigure_UnknownFlag_TakesPrecedence covers the two paths that used
// to swallow a typo by returning 0 before anything examined it: --help printed
// usage, and --status printed a report the caller never asked for.
func TestRunConfigure_UnknownFlag_TakesPrecedence(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"over --help", []string{"--bogus", "--help"}},
		{"over --help, flag second", []string{"--help", "--bogus"}},
		{"over --status", []string{"--status", "--bogus"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withTempHome(t)
			var code int
			_, stderr := captureStdio(t, func() { code = RunConfigure(tc.args) })
			if code != 2 {
				t.Fatalf("expected exit 2, got %d", code)
			}
			if !strings.Contains(stderr, "--bogus") {
				t.Errorf("expected --bogus named in stderr, got %q", stderr)
			}
		})
	}
}

// TestEndOfOptionsMarker covers what "--" is actually for: everything after it
// is a positional, even when it looks like a flag. Deleting `endOfFlags = true`
// leaves `--` consumed by its own case arm and every other test green, because
// only a flag-shaped token *after* the marker tells the two apart.
func TestEndOfOptionsMarker(t *testing.T) {
	cases := []struct {
		name string
		run  func([]string) int
		args []string
	}{
		{"configure", RunConfigure, []string{"--", "--bogus"}},
		{"print-config", RunPrintConfig, []string{"--", "--bogus"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withTempHome(t)
			var code int
			_, stderr := captureStdio(t, func() { code = tc.run(tc.args) })
			if code != 2 {
				t.Fatalf("expected exit 2, got %d", code)
			}
			// The token after "--" is a host name, not a flag, so it must be
			// rejected as an unknown host.
			if !strings.Contains(stderr, "unknown host") {
				t.Errorf("expected --bogus to be read as a host, got %q", stderr)
			}
			if strings.Contains(stderr, "unknown flag") {
				t.Errorf("token after -- was treated as a flag: %q", stderr)
			}
		})
	}
}

// TestRunPrintConfig_UnknownFlag_PrintsNoConfig guards the print path: the
// output is the whole product, so a rejected flag must not emit an entry the
// user would paste into a host config.
func TestRunPrintConfig_UnknownFlag_PrintsNoConfig(t *testing.T) {
	cases := []struct {
		name string
		args []string
		// want names which flag must be rejected, so a case carrying a valid
		// flag alongside a bad one fails if the valid one is what got rejected.
		want string
	}{
		{"typo for --bare", []string{"--bear", "claude-code"}, "--bear"},
		{"bad flag with a good one", []string{"--absolute", "--bogus", "claude-code"}, "--bogus"},
		{"over --help", []string{"--bogus", "--help"}, "--bogus"},
		// Single-dash, because narrowing the prefix test to "--" would send
		// "-x" down the positional path and report it as an unknown *host* —
		// still exit 2, so only the message distinguishes the two.
		{"short bad flag", []string{"-x", "claude-code"}, "-x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withTempHome(t)
			var code int
			stdout, stderr := captureStdio(t, func() { code = RunPrintConfig(tc.args) })
			// Errorf for the same reason as above: the stdout assertion below is
			// what distinguishes the fix from the bug.
			if code != 2 {
				t.Errorf("expected exit 2, got %d", code)
			}
			if !strings.Contains(stderr, "unknown flag "+strconv.Quote(tc.want)) &&
				!strings.Contains(stderr, "unknown flag '"+tc.want+"'") {
				t.Errorf("expected %s to be the rejected flag, got %q", tc.want, stderr)
			}
			// Assert on the entry name, not the binary name: under `go test`
			// resolveMCPCommand returns the test binary's path, so stdout never
			// contains "sp-local-bridge" and a check for it could not fail.
			// printConfigUsage also goes to stdout, and it never names the
			// entry, so this discriminates the reject path from the print path.
			if strings.Contains(stdout, hosts[HostClaudeCode].entryName) {
				t.Errorf("a rejected command printed a config entry: %q", stdout)
			}
		})
	}
}

// TestRunPrintConfig_ValidFlagsStillAccepted is print-config's twin of the test
// below. Without it, deleting `case "--absolute"` or the "-h" arm from
// print-config's loop left the whole suite green, because every other
// print-config test in this file happens to pass no flag at all.
func TestRunPrintConfig_ValidFlagsStillAccepted(t *testing.T) {
	cases := []struct {
		name string
		args []string
		// wantBare is "yes" when the output should carry the bare command name,
		// "no" when it should carry a resolved path, and "" to skip the check
		// (the help cases print no entry at all).
		wantBare string
	}{
		{"--help", []string{"--help"}, ""},
		{"-h", []string{"-h"}, ""},
		{"--absolute", []string{"--absolute", "claude-code"}, "no"},
		{"--bare", []string{"--bare", "claude-code"}, "yes"},
		{"flag after the host", []string{"claude-code", "--bare"}, "yes"},
		{"end-of-options marker", []string{"--", "claude-code"}, "no"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withTempHome(t)
			var code int
			stdout, stderr := captureStdio(t, func() { code = RunPrintConfig(tc.args) })
			if code != 0 {
				t.Errorf("expected exit 0, got %d (stderr: %q)", code, stderr)
			}
			if tc.wantBare == "" {
				return
			}
			// Exit 0 alone does not say the flag did anything: flipping the
			// value --bare and --absolute set left the whole suite green.
			if got := strings.Contains(stdout, `"sp-local-bridge"`); got != (tc.wantBare == "yes") {
				t.Errorf("%s: bare command in output = %v, want %v\noutput: %s",
					tc.name, got, tc.wantBare == "yes", stdout)
			}
		})
	}
}

// TestRunConfigure_ValidFlagsStillAccepted is the other half: rejecting typos
// is only correct if it does not also reject the real flags.
func TestRunConfigure_ValidFlagsStillAccepted(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"--help", []string{"--help"}, 0},
		{"-h", []string{"-h"}, 0},
		{"--status", []string{"--status"}, 0},
		{"--dry-run", []string{"--dry-run", "claude-desktop"}, 0},
		{"--remove on an absent entry", []string{"--remove", "claude-desktop"}, 0},
		// The positive half of TestEndOfOptionsMarker: the host after "--" must
		// still resolve. Without this, making `case "--"` keep the marker as a
		// positional passed every other test while `configure -- <host>` went
		// back to exit 2.
		{"end-of-options marker", []string{"--", "claude-desktop"}, 0},
		{"--dry-run --remove", []string{"--dry-run", "--remove", "claude-desktop"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withTempHome(t)
			var code int
			captureStdio(t, func() { code = RunConfigure(tc.args) })
			if code != tc.want {
				t.Fatalf("expected exit %d, got %d", tc.want, code)
			}
		})
	}
}

// TestExtraPositional_Rejected is the guard for #60. As with the unknown-flag
// guard, the exit code is not the point: the old behaviour also had a
// well-defined exit code (0), with a config file written next to it for a
// command the user had reason to think did something else.
func TestExtraPositional_Rejected(t *testing.T) {
	cases := []struct {
		name string
		args []string
		// wrote names the host whose config file must NOT appear. Every case
		// names the same host because the point is that the *first* positional
		// is not acted on either; untouched names the second host, where there
		// is one, so both ends of the rejection are asserted.
		wrote     string
		untouched string
		// count is the number of positionals the message must report. Without
		// a three-positional case, len(remaining) is indistinguishable from a
		// hardcoded 2.
		count int
		// unexpected is the argument the message must name. Naming the host
		// instead would still say "expected one host" and still exit 2, so
		// without this the message could point at the wrong argument and no
		// test would notice.
		unexpected string
	}{
		{"two hosts", []string{"claude-desktop", "claude-code"}, HostClaudeDesktop, HostClaudeCode, 2, "claude-code"},
		// Three, because with only N=2 fixtures `len(remaining)` and the index
		// `remaining[1]` are indistinguishable from the constant 2 and from
		// "the last one" - two mutations that survived the first sweep.
		{"three hosts", []string{"claude-desktop", "claude-code", "codex"}, HostClaudeDesktop, HostCodex, 3, "claude-code"},
		{"flag after the marker", []string{"claude-desktop", "--", "--dry-run"}, HostClaudeDesktop, "", 2, "--dry-run"},
		{"junk after the host", []string{"claude-desktop", "nonsense"}, HostClaudeDesktop, "", 2, "nonsense"},
		// An unset shell variable, which expands to an empty argument rather
		// than to nothing.
		{"empty argument after the host", []string{"claude-desktop", ""}, HostClaudeDesktop, "", 2, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := withTempHome(t)
			var code int
			_, stderr := captureStdio(t, func() { code = RunConfigure(tc.args) })
			if code != 2 {
				t.Errorf("expected exit 2, got %d", code)
			}
			if !strings.Contains(stderr, "expected one host") {
				t.Errorf("stderr should name the problem, got %q", stderr)
			}
			if !strings.Contains(stderr, fmt.Sprintf("got %d", tc.count)) {
				t.Errorf("expected the message to report %d positionals, got %q", tc.count, stderr)
			}
			// An empty unexpected argument is still worth the case: it proves
			// the command rejects rather than silently proceeding. There is no
			// substring to look for, so only the count assertion above applies.
			if tc.unexpected != "" && !strings.Contains(stderr, tc.unexpected) {
				t.Errorf("expected %q named as the unexpected argument, got %q", tc.unexpected, stderr)
			}
			if _, err := os.Stat(testConfigPath(home, tc.wrote)); err == nil {
				t.Errorf("a rejected command configured %s", tc.wrote)
			}
			if tc.untouched == "" {
				return
			}
			if _, err := os.Stat(testConfigPath(home, tc.untouched)); err == nil {
				t.Errorf("a rejected command configured %s", tc.untouched)
			}
		})
	}
}

// TestExtraPositional_RejectedByPrintConfig is the print-config half. Its
// hazard is different: it cannot write a file, but it can print an entry for a
// host the caller did not name last, which is then pasted somewhere by hand.
func TestExtraPositional_RejectedByPrintConfig(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		count      int
		unexpected string
	}{
		{"two hosts", []string{"claude-code", "codex"}, 2, "codex"},
		// Three, for the same reason as the configure table: with only N=2 the
		// count is indistinguishable from a hardcoded 2, and remaining[1] from
		// "the last one". Both mutations survived until this case existed.
		{"three hosts", []string{"claude-code", "codex", "claude-desktop"}, 3, "codex"},
		// The rest mirror the configure table. Without them five further
		// mutations survived here that the configure table already caught:
		// counting args rather than remaining, indexing args rather than
		// remaining, and three ways of quietly skipping an empty or
		// flag-shaped positional.
		{"flag after the marker", []string{"claude-code", "--", "--bare"}, 2, "--bare"},
		{"junk after the host", []string{"claude-code", "nonsense"}, 2, "nonsense"},
		{"empty argument after the host", []string{"claude-code", ""}, 2, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withTempHome(t)
			var code int
			stdout, stderr := captureStdio(t, func() { code = RunPrintConfig(tc.args) })
			if code != 2 {
				t.Errorf("expected exit 2, got %d", code)
			}
			if !strings.Contains(stderr, "expected one host") {
				t.Errorf("stderr should name the problem, got %q", stderr)
			}
			if !strings.Contains(stderr, fmt.Sprintf("got %d", tc.count)) {
				t.Errorf("expected the message to report %d positionals, got %q", tc.count, stderr)
			}
			// As in the configure table, an empty unexpected argument has no
			// substring to look for; the count assertion above is what covers
			// it, and it is the only case that catches a guard which skips
			// empty positionals.
			if tc.unexpected != "" && !strings.Contains(stderr, tc.unexpected) {
				t.Errorf("expected %q named as the unexpected argument, got %q", tc.unexpected, stderr)
			}
			if strings.Contains(stdout, hosts[HostClaudeCode].entryName) {
				t.Errorf("a rejected command printed a config entry: %q", stdout)
			}
		})
	}
}

// TestOneHostStillAccepted is the other half: rejecting a second positional is
// only correct if exactly one still works, including when it arrives after the
// end-of-options marker or after a flag.
func TestOneHostStillAccepted(t *testing.T) {
	cases := []struct {
		name string
		run  func([]string) int
		args []string
	}{
		{"configure", RunConfigure, []string{"claude-desktop"}},
		{"configure after a flag", RunConfigure, []string{"--dry-run", "claude-desktop"}},
		{"configure after --", RunConfigure, []string{"--", "claude-desktop"}},
		{"print-config", RunPrintConfig, []string{"claude-code"}},
		{"print-config after a flag", RunPrintConfig, []string{"--bare", "claude-code"}},
		{"print-config after --", RunPrintConfig, []string{"--", "claude-code"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withTempHome(t)
			var code int
			_, stderr := captureStdio(t, func() { code = tc.run(tc.args) })
			if code != 0 {
				t.Fatalf("expected exit 0, got %d (stderr: %q)", code, stderr)
			}
		})
	}
}

// TestExtraPositional_HelpStillWins pins where the arity guard sits. Moving it
// ahead of --help, or behind the unknown-host lookup, are both real behaviour
// changes that left the whole suite green before this existed.
func TestExtraPositional_HelpStillWins(t *testing.T) {
	cases := []struct {
		name string
		run  func([]string) int
		args []string
		want int
	}{
		// --help is an explicit request for usage, so it is honoured even
		// alongside a second positional.
		{"configure, help wins", RunConfigure, []string{"claude-desktop", "claude-code", "--help"}, 0},
		{"print-config, help wins", RunPrintConfig, []string{"claude-code", "codex", "--help"}, 0},
		// A mistyped flag does not win, because the user asked for an action.
		// This is the unknown-flag check running ahead of both.
		{"configure, bad flag beats help", RunConfigure, []string{"claude-desktop", "claude-code", "--bogus"}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withTempHome(t)
			var code int
			_, stderr := captureStdio(t, func() { code = tc.run(tc.args) })
			if code != tc.want {
				t.Fatalf("expected exit %d, got %d (stderr: %q)", tc.want, code, stderr)
			}
		})
	}
}

// TestExtraPositional_BeatsUnknownHost pins the other end: the arity error is
// reported before the host is looked up, so a user who typed two wrong things
// is told about the argument count rather than being sent to fix a host name
// they will then have to remove anyway.
func TestExtraPositional_BeatsUnknownHost(t *testing.T) {
	withTempHome(t)
	var code int
	_, stderr := captureStdio(t, func() { code = RunConfigure([]string{"bogus-host", "extra"}) })
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "expected one host") {
		t.Errorf("expected the arity error first, got %q", stderr)
	}
	if strings.Contains(stderr, "unknown host") {
		t.Errorf("host lookup should not have run: %q", stderr)
	}
}
