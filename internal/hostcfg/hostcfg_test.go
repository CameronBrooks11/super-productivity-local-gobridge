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
