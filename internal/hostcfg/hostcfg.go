package hostcfg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// MCPConfig represents the MCP server configuration for host apps.
type MCPConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// RunPrintConfig prints the MCP configuration JSON. Returns exit code.
func RunPrintConfig(args []string) int {
	cfg := buildConfig()
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(cfg)
	return 0
}

// RunConfigure writes the MCP configuration to the appropriate host config file. Returns exit code.
func RunConfigure(args []string) int {
	host := "claude"
	if len(args) > 1 {
		host = args[1]
	}

	cfg := buildConfig()
	wrapper := map[string]any{
		"mcpServers": map[string]any{
			"sp-local-bridge": cfg,
		},
	}

	var configPath string
	switch host {
	case "claude":
		configPath = claudeConfigPath()
	default:
		fmt.Fprintf(os.Stderr, "Unsupported host: %s (supported: claude)\n", host)
		return 2
	}

	if err := writeConfig(configPath, wrapper); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		return 1
	}
	fmt.Printf("Wrote MCP config to %s\n", configPath)
	return 0
}

func buildConfig() MCPConfig {
	exe, _ := os.Executable()
	return MCPConfig{
		Command: exe,
		Args:    []string{"mcp"},
	}
}

func claudeConfigPath() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "Claude", "claude_desktop_config.json")
	default:
		return filepath.Join(home, ".config", "claude", "claude_desktop_config.json")
	}
}

func writeConfig(path string, data any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// Read existing config if present
	existing := make(map[string]any)
	if raw, err := os.ReadFile(path); err == nil {
		json.Unmarshal(raw, &existing)
	}

	// Merge mcpServers
	if servers, ok := data.(map[string]any)["mcpServers"]; ok {
		existingServers, _ := existing["mcpServers"].(map[string]any)
		if existingServers == nil {
			existingServers = make(map[string]any)
		}
		for k, v := range servers.(map[string]any) {
			existingServers[k] = v
		}
		existing["mcpServers"] = existingServers
	}

	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}
