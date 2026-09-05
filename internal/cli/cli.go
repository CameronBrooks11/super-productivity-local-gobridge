package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/bridge"
	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/version"
)

// Usage prints help text.
func Usage() {
	fmt.Printf("sp-local-bridge %s\n", version.Version)
	fmt.Println("Usage: sp-local-bridge <command> [args]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  health                       Check SP connectivity")
	fmt.Println("  status                       Get SP app status")
	fmt.Println("  tasks list [filters]         List tasks (see filters below)")
	fmt.Println("  tasks get <id>               Get a task by ID")
	fmt.Println("  tasks add <title> [flags]    Create a new task")
	fmt.Println("  tasks update <id> [flags]    Update a task")
	fmt.Println("  tasks complete <id>          Mark a task as done")
	fmt.Println("  tasks uncomplete <id>        Mark a task as not done")
	fmt.Println("  tasks start <id>             Start time tracking")
	fmt.Println("  tasks stop-current           Stop current task tracking")
	fmt.Println("  tasks current                Get currently tracked task")
	fmt.Println("  tasks set-current <id>       Set current task by ID")
	fmt.Println("  tasks clear-current          Clear current task")
	fmt.Println("  tasks archive <id>           Archive a task")
	fmt.Println("  tasks restore <id>           Restore an archived task")
	fmt.Println("  projects list [--query ...]  List projects")
	fmt.Println("  tags list [--query ...]      List tags")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  mcp                          Run MCP stdio server")
	fmt.Println("  doctor                       Run diagnostics")
	fmt.Println("  print-config                 Print host config")
	fmt.Println("  configure                    Write host config")
	fmt.Println("                               Hosts: claude-code, claude-desktop, vscode-copilot, codex")
	fmt.Println()
	fmt.Println("Task list filters:")
	fmt.Println("  --query <text>               Filter by title substring")
	fmt.Println("  --project-id <id>            Filter by project")
	fmt.Println("  --tag-id <id>                Filter by tag (use TODAY for today's tasks)")
	fmt.Println("  --include-done               Include completed tasks")
	fmt.Println("  --source <active|archived|all>  Task pool (default: active)")
	fmt.Println("                               archived and all also need --include-done: SP applies")
	fmt.Println("                               the done filter to archived tasks whatever their isDone")
	fmt.Println("                               value, so --source archived alone returns nothing.")
	fmt.Println()
	fmt.Println("Task create flags (tasks add):")
	fmt.Println("  --project-id <id>            Assign to project (from projects list)")
	fmt.Println("  --tag-id <id>                Assign a tag (from tags list)")
	fmt.Println("  --notes <text>               Set notes")
	fmt.Println("  --due-day <YYYY-MM-DD>       Set due date")
	fmt.Println("  --time-estimate <ms>         Set time estimate (milliseconds)")
	fmt.Println()
	fmt.Println("Task update flags:")
	fmt.Println("  --title <text>               New title")
	fmt.Println("  --notes <text>               New notes")
	fmt.Println("  --project-id <id>            Set project")
	fmt.Println("  --due-day <YYYY-MM-DD>       Set due date")
	fmt.Println("  --time-estimate <ms>         Set time estimate (milliseconds)")
	fmt.Println("  --time-spent <ms>            Set time spent (milliseconds)")
}

// Run executes a CLI command. Returns exit code.
func Run(args []string) int {
	if len(args) == 0 {
		Usage()
		return 0
	}

	baseURL := bridge.DefaultBaseURL
	if env := os.Getenv("SP_BASE_URL"); env != "" {
		baseURL = env
	}
	client := bridge.NewClient(baseURL)
	service := bridge.NewService(client)
	ctx := context.Background()

	command := args[0]

	switch command {
	case "health":
		return execOp(ctx, service, bridge.OpBridgeHealth, nil)
	case "status":
		return execOp(ctx, service, bridge.OpStatusGet, nil)
	case "tasks":
		return handleTasks(ctx, service, args[1:])
	case "projects":
		return handleProjects(ctx, service, args[1:])
	case "tags":
		return handleTags(ctx, service, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command '%s'\n", command)
		fmt.Fprintf(os.Stderr, "Available commands: health, status, tasks, projects, tags\n")
		return 2
	}
}

// hasHelpFlag returns true if -h or --help appears in args.
func hasHelpFlag(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}

func handleTasks(ctx context.Context, service *bridge.Service, args []string) int {
	if hasHelpFlag(args) {
		Usage()
		return 0
	}
	sub := "list"
	if len(args) > 0 {
		sub = args[0]
	}

	switch sub {
	case "list":
		var flagArgs []string
		if len(args) > 0 {
			flagArgs = args[1:]
		}
		flags, err := parseListFlags(flagArgs, taskListAllowed)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return 2
		}
		return execOp(ctx, service, bridge.OpTaskList, flags)

	case "get":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: tasks get requires a task ID")
			return 2
		}
		return execOp(ctx, service, bridge.OpTaskGet, rawPayload("id", args[1]))

	case "add":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: tasks add requires a title")
			return 2
		}
		return handleTaskAdd(ctx, service, args[1:])

	case "update":
		return handleTaskUpdate(ctx, service, args[1:])

	case "complete":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: tasks complete requires a task ID")
			return 2
		}
		return execOp(ctx, service, bridge.OpTaskComplete, rawPayload("id", args[1]))

	case "uncomplete":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: tasks uncomplete requires a task ID")
			return 2
		}
		return execOp(ctx, service, bridge.OpTaskUncomplete, rawPayload("id", args[1]))

	case "start":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: tasks start requires a task ID")
			return 2
		}
		return execOp(ctx, service, bridge.OpTaskStart, rawPayload("id", args[1]))

	case "stop-current":
		return execOp(ctx, service, bridge.OpTaskStopCurrent, nil)

	case "current":
		return execOp(ctx, service, bridge.OpTaskGetCurrent, nil)

	case "set-current":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: tasks set-current requires a task ID")
			return 2
		}
		return execOp(ctx, service, bridge.OpTaskSetCurrent, rawPayload("taskId", args[1]))

	case "clear-current":
		payload := map[string]json.RawMessage{"taskId": json.RawMessage("null")}
		return execOp(ctx, service, bridge.OpTaskSetCurrent, payload)

	case "archive":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: tasks archive requires a task ID")
			return 2
		}
		return execOp(ctx, service, bridge.OpTaskArchive, rawPayload("id", args[1]))

	case "restore":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: tasks restore requires a task ID")
			return 2
		}
		return execOp(ctx, service, bridge.OpTaskRestore, rawPayload("id", args[1]))

	default:
		fmt.Fprintf(os.Stderr, "Error: unknown tasks subcommand '%s'\n", sub)
		return 2
	}
}

func handleTaskAdd(ctx context.Context, service *bridge.Service, args []string) int {
	payload := map[string]json.RawMessage{}

	// First non-flag arg is the title
	var title string
	i := 0
	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		title = args[0]
		i = 1
	}

	// Parse flags
	for i < len(args) {
		flag := args[i]
		switch flag {
		case "--project-id":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: Flag --project-id requires a value")
				return 2
			}
			payload["projectId"] = mustMarshal(args[i+1])
			i += 2
		case "--tag-id":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: Flag --tag-id requires a value")
				return 2
			}
			payload["tagIds"] = mustMarshal([]string{args[i+1]})
			i += 2
		case "--notes":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: Flag --notes requires a value")
				return 2
			}
			payload["notes"] = mustMarshal(args[i+1])
			i += 2
		case "--due-day":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: Flag --due-day requires a value")
				return 2
			}
			payload["dueDay"] = mustMarshal(args[i+1])
			i += 2
		case "--time-estimate":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: Flag --time-estimate requires a value")
				return 2
			}
			payload["timeEstimate"] = json.RawMessage(args[i+1])
			i += 2
		default:
			fmt.Fprintf(os.Stderr, "Error: Unknown flag: %s\n", flag)
			return 2
		}
	}

	if title == "" {
		fmt.Fprintln(os.Stderr, "Error: tasks add requires a title")
		return 2
	}
	payload["title"] = mustMarshal(title)

	return execOp(ctx, service, bridge.OpTaskCreate, payload)
}

func handleTaskUpdate(ctx context.Context, service *bridge.Service, args []string) int {
	if hasHelpFlag(args) {
		Usage()
		return 0
	}
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: tasks update requires a task ID")
		return 2
	}
	id := args[0]
	payload := map[string]json.RawMessage{
		"id": mustMarshal(id),
	}

	// Parse update flags
	i := 1
	for i < len(args) {
		flag := args[i]
		switch flag {
		case "--title":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: Flag --title requires a value")
				return 2
			}
			payload["title"] = mustMarshal(args[i+1])
			i += 2
		case "--notes":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: Flag --notes requires a value")
				return 2
			}
			payload["notes"] = mustMarshal(args[i+1])
			i += 2
		case "--project-id":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: Flag --project-id requires a value")
				return 2
			}
			payload["projectId"] = mustMarshal(args[i+1])
			i += 2
		case "--due-day":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: Flag --due-day requires a value")
				return 2
			}
			payload["dueDay"] = mustMarshal(args[i+1])
			i += 2
		case "--time-estimate":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: Flag --time-estimate requires a value")
				return 2
			}
			payload["timeEstimate"] = json.RawMessage(args[i+1])
			i += 2
		case "--time-spent":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: Flag --time-spent requires a value")
				return 2
			}
			payload["timeSpent"] = json.RawMessage(args[i+1])
			i += 2
		case "--done":
			payload["isDone"] = json.RawMessage("true")
			i++
		case "--not-done":
			payload["isDone"] = json.RawMessage("false")
			i++
		default:
			fmt.Fprintf(os.Stderr, "Error: Unknown flag: %s\n", flag)
			return 2
		}
	}

	if len(payload) <= 1 {
		fmt.Fprintln(os.Stderr, "Error: tasks update requires at least one field to update")
		return 2
	}

	return execOp(ctx, service, bridge.OpTaskUpdate, payload)
}

func handleProjects(ctx context.Context, service *bridge.Service, args []string) int {
	if hasHelpFlag(args) {
		Usage()
		return 0
	}
	sub := "list"
	if len(args) > 0 {
		sub = args[0]
	}
	if sub != "list" {
		fmt.Fprintf(os.Stderr, "Error: unknown projects subcommand '%s'\n", sub)
		return 2
	}
	flags, err := parseListFlags(args[1:], queryOnlyAllowed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return 2
	}
	return execOp(ctx, service, bridge.OpProjectList, flags)
}

func handleTags(ctx context.Context, service *bridge.Service, args []string) int {
	if hasHelpFlag(args) {
		Usage()
		return 0
	}
	sub := "list"
	if len(args) > 0 {
		sub = args[0]
	}
	if sub != "list" {
		fmt.Fprintf(os.Stderr, "Error: unknown tags subcommand '%s'\n", sub)
		return 2
	}
	flags, err := parseListFlags(args[1:], queryOnlyAllowed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return 2
	}
	return execOp(ctx, service, bridge.OpTagList, flags)
}

// --- Helpers ---

func execOp(ctx context.Context, service *bridge.Service, op string, payload map[string]json.RawMessage) int {
	req := bridge.Request{Operation: op, Payload: payload}
	result := service.Execute(ctx, req)
	return printResult(result)
}

func printResult(result bridge.Result) int {
	if result.OK {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result.Data)
		return 0
	}
	fmt.Fprintf(os.Stderr, "Error [%s]: %s\n", result.Error.Code, result.Error.Message)
	return 1
}

func rawPayload(key, value string) map[string]json.RawMessage {
	return map[string]json.RawMessage{key: mustMarshal(value)}
}

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

// --- Flag parsing ---

var taskListAllowed = map[string]bool{
	"--query": true, "--project-id": true, "--tag-id": true,
	"--include-done": true, "--source": true,
}

var queryOnlyAllowed = map[string]bool{
	"--query": true,
}

func parseListFlags(args []string, allowed map[string]bool) (map[string]json.RawMessage, error) {
	payload := make(map[string]json.RawMessage)
	i := 0
	for i < len(args) {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			return nil, fmt.Errorf("Unexpected argument: %s", arg)
		}
		if !allowed[arg] {
			return nil, fmt.Errorf("Unknown flag: %s", arg)
		}
		switch arg {
		case "--query":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("Flag --query requires a value")
			}
			payload["query"] = mustMarshal(args[i+1])
			i += 2
		case "--project-id":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("Flag --project-id requires a value")
			}
			payload["projectId"] = mustMarshal(args[i+1])
			i += 2
		case "--tag-id":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("Flag --tag-id requires a value")
			}
			payload["tagId"] = mustMarshal(args[i+1])
			i += 2
		case "--include-done":
			payload["includeDone"] = json.RawMessage("true")
			i++
		case "--source":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("Flag --source requires a value")
			}
			payload["source"] = mustMarshal(args[i+1])
			i += 2
		default:
			return nil, fmt.Errorf("Unknown flag: %s", arg)
		}
	}
	if len(payload) == 0 {
		return nil, nil
	}
	return payload, nil
}
