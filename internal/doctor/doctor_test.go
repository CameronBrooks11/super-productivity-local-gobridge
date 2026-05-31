package doctor

import (
	"os"
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
