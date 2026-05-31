package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCheckHostConfigsNoFiles(t *testing.T) {
	// With a fake HOME, no config files should exist.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", tmp)
		t.Setenv("APPDATA", filepath.Join(tmp, "AppData", "Roaming"))
	}
	result := checkHostConfigs()
	if len(result) != 0 {
		t.Errorf("expected no configured hosts, got %v", result)
	}
}

func TestCheckHostConfigsDetectsJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", tmp)
		t.Setenv("APPDATA", filepath.Join(tmp, "AppData", "Roaming"))
	}

	// Determine where claude config lives on this OS
	var configDir string
	switch runtime.GOOS {
	case "darwin":
		configDir = filepath.Join(tmp, "Library", "Application Support", "Claude")
	case "windows":
		configDir = filepath.Join(tmp, "AppData", "Roaming", "Claude")
	default:
		configDir = filepath.Join(tmp, ".config", "Claude")
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"mcpServers":{"super-productivity":{"command":"sp-local-bridge","args":["mcp"]}}}`
	if err := os.WriteFile(filepath.Join(configDir, "claude_desktop_config.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result := checkHostConfigs()
	if len(result) != 1 || result[0] != "claude-desktop" {
		t.Errorf("expected [claude-desktop], got %v", result)
	}
}

func TestCheckHostConfigsDetectsTOML(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", tmp)
	}

	codexDir := filepath.Join(tmp, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "[mcp_servers.superProductivity]\ncommand = \"sp-local-bridge\"\nargs = [\"mcp\"]\n"
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result := checkHostConfigs()
	if len(result) != 1 || result[0] != "codex" {
		t.Errorf("expected [codex], got %v", result)
	}
}

func TestCheckMCPSelfEmptyPath(t *testing.T) {
	result := checkMCPSelf("")
	if result != "cannot determine binary path" {
		t.Errorf("expected 'cannot determine binary path', got %q", result)
	}
}

func TestCheckMCPSelfNonExistent(t *testing.T) {
	result := checkMCPSelf("/nonexistent/binary")
	if result == "" {
		t.Error("expected error for nonexistent binary")
	}
}

func TestCheckMCPSelfSuccess(t *testing.T) {
	// Build the binary
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "sp-local-bridge")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	// Find module root (go.mod location)
	modRoot := findModRoot()
	if modRoot == "" {
		t.Skip("cannot find module root")
	}

	cmd := exec.Command("go", "build", "-o", bin, "./cmd/sp-local-bridge")
	cmd.Dir = modRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	result := checkMCPSelf(bin)
	if result != "" {
		t.Errorf("expected success, got %q", result)
	}
}

// findModRoot walks up from cwd looking for go.mod.
func findModRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func TestCheckAliasesEmptyPath(t *testing.T) {
	result := checkAliases("")
	if result != "cannot determine binary path" {
		t.Errorf("expected 'cannot determine binary path', got %q", result)
	}
}

func TestCheckAliasesNonePresent(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "sp-local-bridge")
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	result := checkAliases(bin)
	if result == "" {
		t.Error("expected missing aliases, got empty string")
	}
	for _, alias := range multicallAliases {
		if !contains(result, alias) {
			t.Errorf("expected %q in result %q", alias, result)
		}
	}
}

func TestCheckAliasesAllPresent(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "sp-local-bridge")
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create all alias files
	for _, alias := range multicallAliases {
		aliasPath := filepath.Join(tmp, alias)
		if runtime.GOOS == "windows" {
			aliasPath += ".exe"
		}
		if err := os.WriteFile(aliasPath, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	result := checkAliases(bin)
	if result != "" {
		t.Errorf("expected empty string (all present), got %q", result)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
