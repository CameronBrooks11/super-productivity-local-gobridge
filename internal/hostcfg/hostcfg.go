package hostcfg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Supported host identifiers.
const (
	HostClaudeDesktop = "claude-desktop"
	HostClaudeCode    = "claude-code"
	HostVSCodeCopilot = "vscode-copilot"
	HostCodex         = "codex"
)

// hostMeta describes a supported host.
type hostMeta struct {
	format     string // "json" or "toml"
	serverKey  string // top-level key in the config file
	entryName  string // our entry name within that key
	configPath func() string
	// localScopeKey is the top-level key under which a host records
	// per-project servers, keyed by project directory. Empty for hosts that
	// keep every server in one map. Detection reads it; configure does not
	// write it, since which project to write to depends on the caller's
	// working directory rather than on a fixed path.
	localScopeKey string
}

// appDataDir returns %APPDATA%, falling back to the conventional location when
// the variable is unset. Without the fallback, filepath.Join("", ...) yields a
// path relative to the working directory.
func appDataDir() string {
	if ad := os.Getenv("APPDATA"); ad != "" {
		return ad
	}
	return filepath.Join(homeDir(), "AppData", "Roaming")
}

// homeDir returns the user's home directory.
// It prefers $HOME (respects test overrides and standard Unix behavior)
// and falls back to os.UserHomeDir() for Windows/system directories.
func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}

var hosts = map[string]hostMeta{
	HostClaudeDesktop: {
		format:    "json",
		serverKey: "mcpServers",
		entryName: "super-productivity",
		configPath: func() string {
			home := homeDir()
			switch runtime.GOOS {
			case "darwin":
				return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
			case "windows":
				return filepath.Join(appDataDir(), "Claude", "claude_desktop_config.json")
			default:
				return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json")
			}
		},
	},
	// Claude Code is the CLI agent, a distinct host from Claude Desktop with its
	// own config file. Writing the top-level "mcpServers" key configures it at
	// user scope (available in every project). Claude Code also supports project
	// and local scopes, which live elsewhere in this file and in per-project
	// .mcp.json; those are left to `claude mcp add`, since they depend on the
	// working directory rather than on a single fixed path per host.
	HostClaudeCode: {
		format:        "json",
		serverKey:     "mcpServers",
		entryName:     "super-productivity",
		localScopeKey: "projects",
		configPath: func() string {
			return filepath.Join(homeDir(), ".claude.json")
		},
	},
	HostVSCodeCopilot: {
		format:    "json",
		serverKey: "servers",
		entryName: "superProductivity",
		configPath: func() string {
			home := homeDir()
			switch runtime.GOOS {
			case "darwin":
				return filepath.Join(home, "Library", "Application Support", "Code", "User", "mcp.json")
			case "windows":
				return filepath.Join(appDataDir(), "Code", "User", "mcp.json")
			default:
				return filepath.Join(home, ".config", "Code", "User", "mcp.json")
			}
		},
	},
	HostCodex: {
		format:    "toml",
		serverKey: "mcp_servers",
		entryName: "superProductivity",
		configPath: func() string {
			home := homeDir()
			return filepath.Join(home, ".codex", "config.toml")
		},
	},
}

// ConfigTarget describes where a host keeps its MCP configuration and how our
// entry is identified inside it. It exists so that callers which only need to
// *inspect* host configs (doctor) do not have to restate the platform-specific
// paths, which is how a newly supported host ends up invisible to diagnostics.
type ConfigTarget struct {
	Name      string
	Path      string
	ServerKey string
	EntryName string
	Format    string // "json" or "toml"
}

// ConfigTargets returns one entry per supported host, in stable name order.
func ConfigTargets() []ConfigTarget {
	names := sortedHostNames()
	targets := make([]ConfigTarget, 0, len(names))
	for _, name := range names {
		meta := hosts[name]
		targets = append(targets, ConfigTarget{
			Name:      name,
			Path:      meta.configPath(),
			ServerKey: meta.serverKey,
			EntryName: meta.entryName,
			Format:    meta.format,
		})
	}
	return targets
}

// --- Detection ---

// Scopes an entry can be found in. A host that keeps every server in one
// top-level map only ever reports ScopeUser.
const (
	// ScopeUser is the config that applies wherever the host runs.
	ScopeUser = "user"
	// ScopeLocal is an entry recorded against a single project directory.
	// Claude Code calls this "local" scope and stores it in ~/.claude.json
	// under projects.<dir>; it is the default scope for `claude mcp add`.
	ScopeLocal = "local"
)

// Detection reports whether one host has our MCP entry, and where it was found.
type Detection struct {
	Host       string
	Path       string
	Configured bool
	// Scope is ScopeUser when the entry applies everywhere, ScopeLocal when it
	// was found only against specific projects, and empty when not configured.
	Scope string
	// Projects lists the project directories carrying the entry, sorted. It is
	// populated whenever such entries exist, including alongside a user-scope
	// entry, so a caller can tell "configured everywhere" from "configured
	// everywhere and pinned here too".
	Projects []string
}

// ScopeSummary renders where a detected entry applies, in one short phrase, or
// the empty string when the host is not configured. It exists so that doctor
// and `configure --status` describe a scope the same way.
func (d Detection) ScopeSummary() string {
	if !d.Configured {
		return ""
	}
	if d.Scope != ScopeLocal {
		return "user scope"
	}
	if len(d.Projects) == 1 {
		return "local scope: " + d.Projects[0]
	}
	return fmt.Sprintf("local scope: %d projects", len(d.Projects))
}

// DetectConfigured reports, for every supported host in stable name order,
// whether that host's config file contains our entry.
//
// Reading is best effort: a missing or unparsable config counts as not
// configured rather than as an error, because the callers are diagnostics that
// still have to report on the remaining hosts.
func DetectConfigured() []Detection {
	targets := ConfigTargets()
	out := make([]Detection, 0, len(targets))
	for _, t := range targets {
		out = append(out, detectOne(t, hosts[t.Name].localScopeKey))
	}
	return out
}

func detectOne(t ConfigTarget, localScopeKey string) Detection {
	d := Detection{Host: t.Name, Path: t.Path}

	data, err := os.ReadFile(t.Path)
	if err != nil {
		return d
	}

	if t.Format != "json" {
		if tomlHasEntry(string(data), t.ServerKey, t.EntryName) {
			d.Configured = true
			d.Scope = ScopeUser
		}
		return d
	}

	var obj map[string]any
	if json.Unmarshal(data, &obj) != nil {
		return d
	}

	if servers, _ := obj[t.ServerKey].(map[string]any); servers != nil && servers[t.EntryName] != nil {
		d.Configured = true
		d.Scope = ScopeUser
	}

	if localScopeKey != "" {
		d.Projects = projectsWithEntry(obj, localScopeKey, t.ServerKey, t.EntryName)
		if len(d.Projects) > 0 {
			d.Configured = true
			if d.Scope == "" {
				d.Scope = ScopeLocal
			}
		}
	}
	return d
}

// projectsWithEntry returns the project directories whose per-project section
// carries our entry, sorted for stable output.
func projectsWithEntry(obj map[string]any, localScopeKey, serverKey, entryName string) []string {
	projects, _ := obj[localScopeKey].(map[string]any)
	var dirs []string
	for dir, v := range projects {
		section, _ := v.(map[string]any)
		if section == nil {
			continue
		}
		if servers, _ := section[serverKey].(map[string]any); servers != nil && servers[entryName] != nil {
			dirs = append(dirs, dir)
		}
	}
	sort.Strings(dirs)
	return dirs
}

// tomlHasEntry checks if a TOML file contains a table header [serverKey.entryName]
// at the start of a line, with a "command" key in the following section body.
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

// --- Command resolution ---

func resolveMCPCommand() string {
	// The Go binary uses "mcp" subcommand, so the command is always ourselves.
	exe, err := os.Executable()
	if err == nil {
		exe, _ = filepath.EvalSymlinks(exe)
		return exe
	}
	// Fallback: bare name
	return "sp-local-bridge"
}

func buildEntry(host string) map[string]any {
	cmd := resolveMCPCommand()
	entry := map[string]any{
		"command": cmd,
		"args":    []string{"mcp"},
	}
	// VS Code Copilot and Claude Code both record the transport explicitly.
	if host == HostVSCodeCopilot || host == HostClaudeCode {
		entry["type"] = "stdio"
	}
	return entry
}

// --- print-config ---

// RunPrintConfig prints the MCP configuration snippet for a host. Returns exit code.
func RunPrintConfig(args []string) int {
	absolute := true
	var remaining []string

	for _, arg := range args {
		switch arg {
		case "--absolute":
			absolute = true
		case "--bare":
			absolute = false
		case "--help", "-h":
			printConfigUsage()
			return 0
		default:
			if !strings.HasPrefix(arg, "-") {
				remaining = append(remaining, arg)
			}
		}
	}

	if len(remaining) == 0 {
		printConfigUsage()
		return 2
	}

	hostName := remaining[0]
	meta, ok := hosts[hostName]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: unknown host '%s'\n", hostName)
		fmt.Fprintf(os.Stderr, "Supported hosts: %s\n", strings.Join(sortedHostNames(), ", "))
		return 2
	}

	entry := buildEntry(hostName)
	if !absolute {
		entry["command"] = "sp-local-bridge"
	}

	if meta.format == "toml" {
		fmt.Printf("[%s.%s]\n", meta.serverKey, meta.entryName)
		for k, v := range entry {
			switch val := v.(type) {
			case string:
				fmt.Printf("%s = %s\n", k, tomlQuote(val))
			case []string:
				items := make([]string, len(val))
				for i, s := range val {
					items[i] = tomlQuote(s)
				}
				fmt.Printf("%s = [%s]\n", k, strings.Join(items, ", "))
			}
		}
	} else {
		config := map[string]any{
			meta.serverKey: map[string]any{
				meta.entryName: entry,
			},
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(config)
	}

	fmt.Println()
	fmt.Println("Add the above to your config file:")
	fmt.Printf("  Path: %s\n", meta.configPath())
	fmt.Println()
	fmt.Println("Then restart the host application.")
	return 0
}

func printConfigUsage() {
	fmt.Println("Usage: sp-local-bridge print-config [OPTIONS] <host>")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --absolute    Use absolute path for commands (default)")
	fmt.Println("  --bare        Use bare command names (requires PATH)")
	fmt.Println()
	fmt.Println("Supported hosts:")
	for _, h := range sortedHostNames() {
		fmt.Printf("  %s\n", h)
	}
}

// --- configure ---

// RunConfigure writes MCP configuration to a host's config file. Returns exit code.
func RunConfigure(args []string) int {
	dryRun := false
	remove := false
	status := false
	var remaining []string

	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "--remove":
			remove = true
		case "--status":
			status = true
		case "--help", "-h":
			configureUsage()
			return 0
		default:
			if !strings.HasPrefix(arg, "-") {
				remaining = append(remaining, arg)
			}
		}
	}

	// --status reports on every host, so it takes no host argument.
	if status {
		printHostStatus()
		return 0
	}

	if len(remaining) == 0 {
		configureUsage()
		return 2
	}

	hostName := remaining[0]
	meta, ok := hosts[hostName]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: unknown host '%s'\n", hostName)
		fmt.Fprintf(os.Stderr, "Supported hosts: %s\n", strings.Join(sortedHostNames(), ", "))
		return 2
	}

	configPath := meta.configPath()

	if remove {
		return removeEntry(configPath, meta, dryRun)
	}
	return addEntry(hostName, configPath, meta, dryRun)
}

// printHostStatus reports, for every supported host, whether our MCP entry is
// present. Installing replaces the binary and leaves host configs alone, so
// after an upgrade this is the answer to "do I need to run configure again?".
// It reads config files only and never contacts SP, so it stays usable from an
// install script.
func printHostStatus() {
	detections := DetectConfigured()
	anyConfigured := false

	fmt.Println("Host config status:")
	for _, d := range detections {
		if d.Configured {
			anyConfigured = true
			fmt.Printf("  %-20s configured (%s)\n", d.Host, d.ScopeSummary())
			continue
		}
		fmt.Printf("  %-20s not configured\n", d.Host)
	}

	fmt.Println()
	if anyConfigured {
		fmt.Println("Configure another with: sp-local-bridge configure <host>")
		return
	}
	fmt.Println("No host is configured. Configure one with:")
	fmt.Printf("  sp-local-bridge configure <host>    (%s)\n", strings.Join(sortedHostNames(), ", "))
}

func configureUsage() {
	fmt.Println("Usage: sp-local-bridge configure [OPTIONS] <host>")
	fmt.Println()
	fmt.Println("Write MCP configuration directly to a host's config file.")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --dry-run     Show what would be written without making changes")
	fmt.Println("  --remove      Remove our entry from the host config")
	fmt.Println("  --status      Report which hosts are configured (no host argument)")
	fmt.Println()
	fmt.Println("Supported hosts:")
	for _, h := range sortedHostNames() {
		meta := hosts[h]
		fmt.Printf("  %-20s → %s\n", h, meta.configPath())
	}
}

// fingerprint captures size and mtime so a read-modify-write can tell that
// something else changed the file in between. Claude Code rewrites
// ~/.claude.json while it runs, and rewriting the whole file from a stale read
// would silently revert whatever it wrote.
func fingerprint(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d:%d", fi.Size(), fi.ModTime().UnixNano())
}

func concurrentModification(configPath, before string) bool {
	return fingerprint(configPath) != before
}

// --- JSON config manipulation ---

func addEntry(hostName, configPath string, meta hostMeta, dryRun bool) int {
	entry := buildEntry(hostName)

	if meta.format == "toml" {
		return addTOMLEntry(configPath, meta, entry, dryRun)
	}

	// JSON host
	before := fingerprint(configPath)
	existing, err := readJSON(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot parse %s\n", configPath)
		fmt.Fprintf(os.Stderr, "  %v\n", err)
		fmt.Fprintln(os.Stderr, "  Manual repair needed. Then re-run this command.")
		return 1
	}

	servers, _ := existing[meta.serverKey].(map[string]any)
	if servers == nil {
		servers = make(map[string]any)
	}
	servers[meta.entryName] = entry
	existing[meta.serverKey] = servers

	if dryRun {
		fmt.Printf("Would write to: %s\n", configPath)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(existing)
		return 0
	}

	if concurrentModification(configPath, before) {
		fmt.Fprintf(os.Stderr, "Error: %s changed while we were editing it.\n", configPath)
		fmt.Fprintln(os.Stderr, "  Nothing was written. Close the host application and re-run.")
		return 1
	}

	if err := writeJSON(configPath, existing); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		return 1
	}

	fmt.Printf("✓ Configured %s\n", hostName)
	fmt.Printf("  Written to: %s\n", configPath)
	fmt.Printf("  Restart %s to pick up the change.\n", hostName)
	if hostName == HostClaudeCode {
		fmt.Println("  Note: Claude Code rewrites this file as it runs. A session that was")
		fmt.Println("  already open can overwrite this entry when it exits — if the entry is")
		fmt.Println("  missing later, close all sessions and re-run this command.")
	}
	return 0
}

func removeEntry(configPath string, meta hostMeta, dryRun bool) int {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("Config file does not exist: %s\n", configPath)
		fmt.Println("Nothing to remove.")
		return 0
	}

	if meta.format == "toml" {
		return removeTOMLEntry(configPath, meta, dryRun)
	}

	before := fingerprint(configPath)
	existing, err := readJSON(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot parse %s\n", configPath)
		fmt.Fprintf(os.Stderr, "  %v\n", err)
		fmt.Fprintln(os.Stderr, "  Manual repair needed. Then re-run this command.")
		return 1
	}

	servers, _ := existing[meta.serverKey].(map[string]any)
	if servers == nil || servers[meta.entryName] == nil {
		fmt.Printf("Entry '%s' not found in %s\n", meta.entryName, configPath)
		fmt.Println("Nothing to remove.")
		return 0
	}

	delete(servers, meta.entryName)
	if len(servers) == 0 {
		delete(existing, meta.serverKey)
	}

	if dryRun {
		fmt.Printf("Would remove '%s' from: %s\n", meta.entryName, configPath)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(existing)
		return 0
	}

	if concurrentModification(configPath, before) {
		fmt.Fprintf(os.Stderr, "Error: %s changed while we were editing it.\n", configPath)
		fmt.Fprintln(os.Stderr, "  Nothing was written. Close the host application and re-run.")
		return 1
	}

	if err := writeJSON(configPath, existing); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		return 1
	}

	fmt.Printf("✓ Removed sp-local-bridge entry from %s\n", configPath)
	return 0
}

// --- TOML support (surgical line-level editing) ---

func addTOMLEntry(configPath string, meta hostMeta, entry map[string]any, dryRun bool) int {
	targetHeader := fmt.Sprintf("[%s.%s]", meta.serverKey, meta.entryName)
	entryContent := formatTOMLEntry(entry)

	if dryRun {
		fmt.Printf("Would write to: %s\n", configPath)
		fmt.Println(targetHeader)
		fmt.Println(entryContent)
		return 0
	}

	var originalLines []string
	if data, err := os.ReadFile(configPath); err == nil {
		content := string(data)
		if err := validateTOMLStructure(content); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot parse %s\n", configPath)
			fmt.Fprintf(os.Stderr, "  %v\n", err)
			fmt.Fprintln(os.Stderr, "  Manual repair needed. Then re-run this command.")
			return 1
		}
		originalLines = strings.SplitAfter(content, "\n")
	}

	resultLines := surgicalTOMLWrite(originalLines, meta.serverKey, meta.entryName, &entryContent)

	backup(configPath)
	if err := atomicWrite(configPath, strings.Join(resultLines, "")); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		return 1
	}

	fmt.Printf("✓ Configured %s\n", HostCodex)
	fmt.Printf("  Written to: %s\n", configPath)
	fmt.Printf("  Restart codex to pick up the change.\n")
	return 0
}

func removeTOMLEntry(configPath string, meta hostMeta, dryRun bool) int {
	data, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("Config file does not exist: %s\n", configPath)
		fmt.Println("Nothing to remove.")
		return 0
	}

	content := string(data)
	if err := validateTOMLStructure(content); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot parse %s\n", configPath)
		fmt.Fprintf(os.Stderr, "  %v\n", err)
		fmt.Fprintln(os.Stderr, "  Manual repair needed. Then re-run this command.")
		return 1
	}

	originalLines := strings.SplitAfter(content, "\n")
	targetHeader := fmt.Sprintf("[%s.%s]", meta.serverKey, meta.entryName)

	// Check if entry exists
	found := false
	for _, line := range originalLines {
		if strings.TrimSpace(line) == targetHeader {
			found = true
			break
		}
	}
	if !found {
		fmt.Printf("Entry '%s' not found in %s\n", meta.entryName, configPath)
		fmt.Println("Nothing to remove.")
		return 0
	}

	if dryRun {
		fmt.Printf("Would remove '%s' from: %s\n", meta.entryName, configPath)
		fmt.Printf("  (would remove [%s.%s] section)\n", meta.serverKey, meta.entryName)
		return 0
	}

	resultLines := surgicalTOMLWrite(originalLines, meta.serverKey, meta.entryName, nil)

	backup(configPath)
	if err := atomicWrite(configPath, strings.Join(resultLines, "")); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		return 1
	}

	fmt.Printf("✓ Removed sp-local-bridge entry from %s\n", configPath)
	return 0
}

func formatTOMLEntry(entry map[string]any) string {
	var lines []string
	if cmd, ok := entry["command"].(string); ok {
		lines = append(lines, fmt.Sprintf("command = %s", tomlQuote(cmd)))
	}
	if args, ok := entry["args"].([]string); ok {
		items := make([]string, len(args))
		for i, s := range args {
			items[i] = tomlQuote(s)
		}
		lines = append(lines, fmt.Sprintf("args = [%s]", strings.Join(items, ", ")))
	}
	return strings.Join(lines, "\n")
}

// tomlQuote produces a valid TOML basic string (double-quoted with escaping).
// Single-quoted literal strings cannot contain single quotes, so we always
// use basic strings for safety with arbitrary paths.
func tomlQuote(s string) string {
	// Escape backslashes and double quotes per TOML spec
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// validateTOMLStructure performs a basic structural guard before surgical editing.
// It checks that table headers are balanced ([...]) and non-header lines contain
// a key=value assignment. This is NOT a full TOML parser: it does not validate
// strings, arrays, inline tables, escaping, duplicate keys, or value types.
// Its purpose is to reject obvious corruption that would make surgical line
// editing unsafe, not to guarantee valid TOML.
func validateTOMLStructure(content string) error {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		stripped := strings.TrimSpace(line)
		if stripped == "" || strings.HasPrefix(stripped, "#") {
			continue
		}
		// Table header validation
		if strings.HasPrefix(stripped, "[") {
			// Must end with ]
			if !strings.HasSuffix(stripped, "]") {
				return fmt.Errorf("line %d: unclosed table header: %s", i+1, stripped)
			}
			// Extract content between brackets (handle [[array]])
			inner := stripped[1 : len(stripped)-1]
			if strings.HasPrefix(inner, "[") && strings.HasSuffix(inner, "]") {
				inner = inner[1 : len(inner)-1] // array of tables
			}
			// Must have a valid dotted key
			inner = strings.TrimSpace(inner)
			if inner == "" {
				return fmt.Errorf("line %d: empty table header", i+1)
			}
			continue
		}
		// Key=value line validation — must contain = outside of quotes
		if !strings.Contains(stripped, "=") {
			return fmt.Errorf("line %d: expected key = value, got: %s", i+1, stripped)
		}
	}
	return nil
}

// surgicalTOMLWrite adds, replaces, or removes a single [serverKey.entryName] block
// while preserving the rest of the file verbatim.
func surgicalTOMLWrite(originalLines []string, serverKey, entryName string, entryContent *string) []string {
	targetHeader := fmt.Sprintf("[%s.%s]", serverKey, entryName)
	targetPrefix := fmt.Sprintf("[%s.%s", serverKey, entryName)

	// Find boundaries of our entry
	entryStart := -1
	entryEnd := -1
	for i, line := range originalLines {
		stripped := strings.TrimSpace(line)
		if stripped == targetHeader {
			entryStart = i
		} else if entryStart >= 0 && strings.HasPrefix(stripped, "[") && i > entryStart {
			// Check if descendant table
			if strings.HasPrefix(stripped, targetPrefix+".") || strings.HasPrefix(stripped, targetPrefix+"]") {
				continue
			}
			entryEnd = i
			break
		}
	}

	if entryStart >= 0 {
		if entryEnd < 0 {
			entryEnd = len(originalLines)
		}
		if entryContent != nil {
			// Replace
			replacement := targetHeader + "\n" + *entryContent + "\n"
			result := make([]string, 0, len(originalLines))
			result = append(result, originalLines[:entryStart]...)
			result = append(result, replacement)
			result = append(result, originalLines[entryEnd:]...)
			return result
		}
		// Remove — also eat preceding blank line
		start := entryStart
		if start > 0 && strings.TrimSpace(originalLines[start-1]) == "" {
			start--
		}
		result := make([]string, 0, len(originalLines))
		result = append(result, originalLines[:start]...)
		result = append(result, originalLines[entryEnd:]...)
		return result
	}

	if entryContent != nil {
		// Append
		result := make([]string, len(originalLines))
		copy(result, originalLines)
		if len(result) > 0 && strings.TrimSpace(result[len(result)-1]) != "" {
			result = append(result, "\n")
		}
		result = append(result, targetHeader+"\n"+*entryContent+"\n")
		return result
	}

	// Nothing to remove
	return originalLines
}

// --- File I/O ---

func readJSON(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), nil
		}
		return nil, err
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return make(map[string]any), nil
	}
	// UseNumber keeps numbers as their original literal instead of decoding to
	// float64. Host configs are rewritten wholesale, so a float64 round-trip
	// would rewrite every number in a file we only meant to add one key to, and
	// would silently lose precision on integers above 2^53.
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var result map[string]any
	if err := dec.Decode(&result); err != nil {
		return nil, err
	}
	// Decode stops after the first JSON value, so unlike json.Unmarshal it would
	// accept a file corrupted by a doubled or partially appended write and let us
	// rewrite it with only the leading object. Reject that instead.
	if dec.More() {
		return nil, fmt.Errorf("unexpected data after top-level JSON value")
	}
	return result, nil
}

func writeJSON(path string, data map[string]any) error {
	// Backup existing file
	backup(path)

	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, string(content)+"\n")
}

func backup(path string) {
	fi, err := os.Stat(path)
	if err != nil {
		return
	}
	backupPath := path + ".bak"
	// Best-effort copy. Mirror the source file's mode rather than defaulting to
	// 0644: host configs can hold account identifiers and session state, so a
	// 0600 original must not be copied to a world-readable .bak. WriteFile
	// leaves the mode alone if the backup already exists, hence the Chmod.
	src, err := os.ReadFile(path)
	if err != nil {
		return
	}
	mode := fi.Mode().Perm()
	if os.WriteFile(backupPath, src, mode) == nil {
		os.Chmod(backupPath, mode)
	}
}

func atomicWrite(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// Write to temp file in same directory, then rename
	f, err := os.CreateTemp(dir, ".sp-local-bridge-*.tmp")
	if err != nil {
		return err
	}
	tmpName := f.Name()

	_, writeErr := f.WriteString(content)
	closeErr := f.Close()

	if writeErr != nil {
		os.Remove(tmpName)
		return writeErr
	}
	if closeErr != nil {
		os.Remove(tmpName)
		return closeErr
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// --- Helpers ---

// sortedHostNames derives the supported host list from the hosts map rather
// than restating it, so a newly added host cannot be missing from `--help`,
// the unknown-host error, or doctor's config detection.
func sortedHostNames() []string {
	names := make([]string, 0, len(hosts))
	for name := range hosts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
