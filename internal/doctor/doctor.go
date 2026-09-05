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

// deepTimeout caps each individual request in the --deep pull, which is far
// larger than anything the standard checks fetch.
const deepTimeout = 60 * time.Second

// deepTotalTimeout is the ceiling for the whole confirmation run: up to two
// passes of four requests, each capped at deepTimeout. The per-request cap is
// what actually bounds a hung request; this only stops the run as a whole from
// outliving its worst case. Sizing it any tighter starves the second pass and
// marks large stores unconfirmed.
const deepTotalTimeout = 2 * 4 * deepTimeout

// Run executes the doctor diagnostics. Returns exit code.
func Run(args []string) int {
	deep := false
	asJSON := false
	wantHelp := false
	var bad string
	badSeen := false

	for _, arg := range args {
		switch arg {
		case "--deep":
			deep = true
		case "--json":
			asJSON = true
		case "--help", "-h":
			wantHelp = true
		default:
			// Ignoring an unrecognised argument silently meant `doctor deep`
			// ran a shallow check and still printed "All checks passed", so the
			// user believed the integrity check had run. Keep the first one:
			// naming the last sends the user round the loop once per typo.
			// badSeen is separate from bad so that an argument which *is* the
			// empty string — `doctor "$UNSET_VAR"` — is still reported.
			if !badSeen {
				bad, badSeen = arg, true
			}
		}
	}

	// --json documents stdout as a JSON stream, so nothing else may go there.
	helpOut := io.Writer(os.Stdout)
	if asJSON {
		helpOut = os.Stderr
	}
	if badSeen {
		fmt.Fprintf(os.Stderr, "Error: unexpected argument '%s'\n", bad)
		usage(os.Stderr)
		return 2
	}
	if wantHelp {
		usage(helpOut)
		return 0
	}

	baseURL := bridge.DefaultBaseURL
	if env := os.Getenv("SP_BASE_URL"); env != "" {
		baseURL = env
	}

	if asJSON {
		client := bridge.NewClientWithTimeout(baseURL, deepTimeout)
		ctx, cancel := context.WithTimeout(context.Background(), deepTotalTimeout)
		defer cancel()
		report, err := CheckIntegrityConfirmed(ctx, client)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return 1
		}
		fmt.Println(integrityJSON(report))
		switch {
		case report.HasConfirmedAnomalies():
			// Survived both passes, so exit 3 regardless of anything else the
			// run could not resolve.
			return 3
		case !report.Confirmed():
			// Nothing seen twice. Exit 3 promises "SP answered and its data is
			// broken"; claiming that from a single observation is the false
			// positive confirmation exists to prevent.
			return 1
		default:
			return 0
		}
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
		deepCtx, deepCancel := context.WithTimeout(context.Background(), deepTotalTimeout)
		defer deepCancel()
		report, err := CheckIntegrityConfirmed(deepCtx, deepClient)
		if err != nil {
			fmt.Printf("FAILED: %s\n", err)
			failures++
		} else if !printIntegrity(report) {
			if report.HasConfirmedAnomalies() {
				// Something survived both passes. A race elsewhere in the store
				// does not make it less real, so it still gets exit 3.
				inconsistent = true
			} else {
				// Nothing was seen twice; treat it as a check that did not
				// complete rather than as evidence the store is broken.
				failures++
			}
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
	fmt.Fprintln(w, "  --json    Print only the integrity report as JSON (runs the deep check).")
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

// checkHostConfigs returns one label per host whose config file contains our
// entry. Detection lives in hostcfg, which owns the per-host paths and keys; a
// label here only decides how to say what was found.
func checkHostConfigs() []string {
	var configured []string
	for _, d := range hostcfg.DetectConfigured() {
		if !d.Configured {
			continue
		}
		configured = append(configured, hostConfigLabel(d))
	}
	return configured
}

// hostConfigLabel names a configured host, qualifying it when the entry only
// applies to specific projects. Saying "claude-code" flat in that case would
// claim more than was found: the host is configured where those projects are
// and nowhere else.
func hostConfigLabel(d hostcfg.Detection) string {
	if d.Scope != hostcfg.ScopeLocal {
		return d.Host
	}
	return fmt.Sprintf("%s (%s)", d.Host, d.ScopeSummary())
}

// expectedToolCount is the number of MCP tools the bridge should expose.
const expectedToolCount = 16

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
