package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/bridge"
	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/version"
)

// Run executes the doctor diagnostics. Returns exit code.
func Run(args []string) int {
	baseURL := bridge.DefaultBaseURL
	if env := os.Getenv("SP_BASE_URL"); env != "" {
		baseURL = env
	}

	fmt.Printf("sp-local-bridge doctor (%s)\n", version.String())
	fmt.Println("─────────────────────────────────────")
	failures := 0

	// Binary info
	exe, _ := os.Executable()
	exe, _ = filepath.EvalSymlinks(exe)
	fmt.Printf("Binary:   %s\n", exe)
	fmt.Printf("Version:  %s\n", version.String())
	fmt.Printf("OS/Arch:  %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Target:   %s\n", baseURL)
	fmt.Println()

	// PATH check
	fmt.Print("PATH visibility... ")
	if exe != "" {
		dir := filepath.Dir(exe)
		pathDirs := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
		found := false
		for _, d := range pathDirs {
			if d == dir {
				found = true
				break
			}
		}
		if found {
			fmt.Println("OK")
		} else {
			fmt.Printf("WARN: %s not in PATH\n", dir)
		}
	} else {
		fmt.Println("WARN: could not determine binary path")
	}

	// Host config status
	fmt.Print("Host configs... ")
	hostStatus := checkHostConfigs()
	if len(hostStatus) > 0 {
		fmt.Printf("found: %s\n", strings.Join(hostStatus, ", "))
	} else {
		fmt.Println("none configured")
	}

	fmt.Println()

	client := bridge.NewClient(baseURL)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Health check
	fmt.Print("Health check... ")
	health := client.Health(ctx)
	if health.OK {
		fmt.Println("OK")
	} else {
		fmt.Printf("FAILED: %s\n", health.Error.Message)
		fmt.Println("  → Is Super Productivity running with the Local REST API enabled?")
		failures++
	}

	// Status check
	fmt.Print("Status check... ")
	status := client.Status(ctx)
	if status.OK {
		fmt.Println("OK")
		data, _ := json.MarshalIndent(status.Data, "  ", "  ")
		fmt.Printf("  %s\n", data)
	} else {
		fmt.Printf("FAILED: %s\n", status.Error.Message)
		failures++
	}

	// Task list smoke test (only if health passed)
	if health.OK {
		fmt.Print("Task list... ")
		taskCtx, taskCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer taskCancel()
		result := client.ListTasks(taskCtx, nil)
		if result.OK {
			fmt.Println("OK")
		} else {
			fmt.Printf("FAILED: %s\n", result.Error.Message)
			failures++
		}
	}

	fmt.Println()
	if failures > 0 {
		fmt.Printf("%d check(s) failed.\n", failures)
		return 1
	}
	fmt.Println("All checks passed.")
	return 0
}

// checkHostConfigs returns list of host names whose config files contain our entry.
func checkHostConfigs() []string {
	type hostCheck struct {
		name      string
		path      string
		serverKey string
		entryName string
		format    string
	}

	home, _ := os.UserHomeDir()
	checks := []hostCheck{
		{"claude-desktop", filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"), "mcpServers", "super-productivity", "json"},
		{"vscode-copilot", filepath.Join(home, ".config", "Code", "User", "mcp.json"), "servers", "superProductivity", "json"},
		{"codex", filepath.Join(home, ".codex", "config.toml"), "mcp_servers", "superProductivity", "toml"},
	}

	if runtime.GOOS == "darwin" {
		checks[0].path = filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
		checks[1].path = filepath.Join(home, "Library", "Application Support", "Code", "User", "mcp.json")
	} else if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		checks[0].path = filepath.Join(appData, "Claude", "claude_desktop_config.json")
		checks[1].path = filepath.Join(appData, "Code", "User", "mcp.json")
		// codex path stays the same (~/.codex/config.toml)
	}

	var configured []string
	for _, c := range checks {
		data, err := os.ReadFile(c.path)
		if err != nil {
			continue
		}
		if c.format == "json" {
			var obj map[string]any
			if json.Unmarshal(data, &obj) != nil {
				continue
			}
			servers, _ := obj[c.serverKey].(map[string]any)
			if servers != nil && servers[c.entryName] != nil {
				configured = append(configured, c.name)
			}
		} else {
			// Simple TOML check: look for the header line
			header := fmt.Sprintf("[%s.%s]", c.serverKey, c.entryName)
			if strings.Contains(string(data), header) {
				configured = append(configured, c.name)
			}
		}
	}
	return configured
}
