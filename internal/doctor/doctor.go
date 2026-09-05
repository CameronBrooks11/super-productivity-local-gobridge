package doctor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/bridge"
	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/hostcfg"
	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/version"
)

// deepTimeout bounds the --deep store pull, which is far larger than any
// request the standard checks make.
const deepTimeout = 60 * time.Second

// Run executes the doctor diagnostics. Returns exit code.
func Run(args []string) int {
	deep := false
	asJSON := false
	for _, arg := range args {
		switch arg {
		case "--deep":
			deep = true
		case "--json":
			asJSON = true
		case "--help", "-h":
			usage(os.Stdout)
			return 0
		case "doctor":
			// main.go forwards os.Args[1:], so the subcommand word arrives here.
		default:
			// Anything else is a typo. Ignoring it silently meant `doctor deep`
			// ran a shallow check and still printed "All checks passed", so the
			// user believed the integrity check had run.
			// Help goes to stderr here: --json documents stdout as a JSON
			// stream, and a usage dump would corrupt it for any script parsing it.
			fmt.Fprintf(os.Stderr, "Error: unexpected argument '%s'\n", arg)
			usage(os.Stderr)
			return 2
		}
	}
	// --json prints only the integrity report, so it implies --deep.
	if asJSON {
		deep = true
	}

	baseURL := bridge.DefaultBaseURL
	if env := os.Getenv("SP_BASE_URL"); env != "" {
		baseURL = env
	}

	if asJSON {
		client := bridge.NewClientWithTimeout(baseURL, deepTimeout)
		ctx, cancel := context.WithTimeout(context.Background(), deepTimeout)
		defer cancel()
		report, err := CheckIntegrity(ctx, client)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return 1
		}
		fmt.Println(integrityJSON(report))
		if !report.Clean() {
			// Same contract as the human path: 3 means SP answered and its data
			// is broken, which a script must not confuse with a failed request.
			return 3
		}
		return 0
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

	// Store integrity (opt-in: pulls the whole store)
	inconsistent := false
	if deep && !health.OK {
		// Saying nothing here left a user who explicitly asked for --deep with
		// no signal that it never ran.
		fmt.Println("Store integrity... SKIPPED (health check failed)")
	}
	if deep && health.OK {
		fmt.Print("Store integrity... ")
		// The archived pull with includeDone is the largest response the bridge
		// ever requests, so it gets its own client: http.Client.Timeout caps
		// each request independently of the context deadline.
		deepClient := bridge.NewClientWithTimeout(baseURL, deepTimeout)
		deepCtx, deepCancel := context.WithTimeout(context.Background(), deepTimeout)
		defer deepCancel()
		report, err := CheckIntegrity(deepCtx, deepClient)
		if err != nil {
			fmt.Printf("FAILED: %s\n", err)
			failures++
		} else if !printIntegrity(report) {
			inconsistent = true
		}
	}

	// MCP self-check
	fmt.Print("MCP self-check... ")
	if mcpErr := checkMCPSelf(exe); mcpErr != "" {
		fmt.Printf("FAILED: %s\n", mcpErr)
		failures++
	} else {
		fmt.Println("OK (16 tools)")
	}

	// Multicall alias check
	fmt.Print("Multicall aliases... ")
	if aliasErr := checkAliases(exe); aliasErr != "" {
		fmt.Printf("WARN: %s\n", aliasErr)
	} else {
		fmt.Println("OK")
	}

	fmt.Println()
	code := exitCode(failures, inconsistent)
	switch code {
	case 1:
		fmt.Printf("%d check(s) failed.\n", failures)
	case 3:
		// Not a failed check: every request succeeded. The store itself is the
		// problem, and that is worth a distinct exit code so scripts can tell
		// "cannot reach SP" from "SP answered, and its data is broken".
		fmt.Println("Checks passed, but the store is inconsistent (see above).")
	default:
		fmt.Println("All checks passed.")
	}
	return code
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "Usage: sp-local-bridge doctor [OPTIONS]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  --deep    Also cross-check task entities against project/tag indexes.")
	fmt.Fprintln(w, "            Pulls the whole store, so it is slower than the default run.")
	fmt.Fprintln(w, "  --json    Print only the integrity report as JSON (implies --deep).")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Exit codes: 0 ok, 1 a check failed, 2 bad usage, 3 store inconsistent.")
}

// exitCode maps the run's outcome to a process exit code.
//
// A failed check outranks an inconsistent store: if a request did not complete,
// the integrity verdict is not trustworthy enough to report as one.
func exitCode(failures int, inconsistent bool) int {
	switch {
	case failures > 0:
		return 1
	case inconsistent:
		return 3
	default:
		return 0
	}
}

// checkHostConfigs returns list of host names whose config files contain our entry.
func checkHostConfigs() []string {
	checks := hostcfg.ConfigTargets()

	var configured []string
	for _, c := range checks {
		data, err := os.ReadFile(c.Path)
		if err != nil {
			continue
		}
		if c.Format == "json" {
			var obj map[string]any
			if json.Unmarshal(data, &obj) != nil {
				continue
			}
			servers, _ := obj[c.ServerKey].(map[string]any)
			if servers != nil && servers[c.EntryName] != nil {
				configured = append(configured, c.Name)
			}
		} else {
			// TOML check: verify the table header exists at start of line
			// and has a command key in the following section.
			if tomlHasEntry(string(data), c.ServerKey, c.EntryName) {
				configured = append(configured, c.Name)
			}
		}
	}
	return configured
}

// expectedToolCount is the number of MCP tools the bridge should expose.
const expectedToolCount = 16

// tomlHasEntry checks if a TOML file contains a table header [serverKey.entryName]
// at the start of a line, with a "command" key in the section body.
// This is a line-based parse that avoids matching inside strings or comments.
func tomlHasEntry(data, serverKey, entryName string) bool {
	header := fmt.Sprintf("[%s.%s]", serverKey, entryName)
	lines := strings.Split(data, "\n")
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip comments
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == header {
			inSection = true
			continue
		}
		// Another table header ends our section
		if inSection && strings.HasPrefix(trimmed, "[") {
			break
		}
		if inSection && strings.HasPrefix(trimmed, "command") {
			// Verify it's a key assignment (command = ...)
			rest := strings.TrimPrefix(trimmed, "command")
			rest = strings.TrimSpace(rest)
			if strings.HasPrefix(rest, "=") {
				return true
			}
		}
	}
	return false
}

// multicallAliases are the expected symlink/hardlink names.
var multicallAliases = []string{
	"sp-local-bridge-mcp",
	"sp-local-bridge-doctor",
	"sp-local-bridge-print-config",
	"sp-local-bridge-configure",
}

// checkMCPSelf spawns the binary with "mcp", sends initialize + tools/list,
// and verifies the expected tool count. Returns empty string on success.
func checkMCPSelf(binaryPath string) string {
	if binaryPath == "" {
		return "cannot determine binary path"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "mcp")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Sprintf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Sprintf("stdout pipe: %v", err)
	}
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return fmt.Sprintf("start: %v", err)
	}
	defer func() {
		stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	scanner := bufio.NewScanner(stdout)

	// Send initialize
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"doctor","version":"1.0.0"}}}` + "\n"
	if _, err := io.WriteString(stdin, initReq); err != nil {
		return fmt.Sprintf("write initialize: %v", err)
	}

	if !scanner.Scan() {
		return "no response to initialize"
	}
	var initResp struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &initResp); err != nil {
		return fmt.Sprintf("parse initialize response: %v", err)
	}
	if initResp.Error != nil {
		return fmt.Sprintf("initialize error: %s", initResp.Error.Message)
	}

	// Send initialized notification
	initializedNotif := `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"
	if _, err := io.WriteString(stdin, initializedNotif); err != nil {
		return fmt.Sprintf("write initialized: %v", err)
	}

	// Send tools/list
	toolsReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	if _, err := io.WriteString(stdin, toolsReq); err != nil {
		return fmt.Sprintf("write tools/list: %v", err)
	}

	if !scanner.Scan() {
		return "no response to tools/list"
	}
	var toolsResp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &toolsResp); err != nil {
		return fmt.Sprintf("parse tools/list response: %v", err)
	}
	if toolsResp.Error != nil {
		return fmt.Sprintf("tools/list error: %s", toolsResp.Error.Message)
	}

	count := len(toolsResp.Result.Tools)
	if count != expectedToolCount {
		return fmt.Sprintf("expected %d tools, got %d", expectedToolCount, count)
	}

	return ""
}

// checkAliases verifies that multicall aliases exist next to the binary.
// Returns empty string if all found, or a description of missing ones.
func checkAliases(binaryPath string) string {
	if binaryPath == "" {
		return "cannot determine binary path"
	}

	dir := filepath.Dir(binaryPath)
	var missing []string
	for _, alias := range multicallAliases {
		aliasPath := filepath.Join(dir, alias)
		if runtime.GOOS == "windows" {
			aliasPath += ".exe"
		}
		if _, err := os.Stat(aliasPath); err != nil {
			missing = append(missing, alias)
		}
	}

	if len(missing) > 0 {
		return fmt.Sprintf("missing: %s", strings.Join(missing, ", "))
	}
	return ""
}
