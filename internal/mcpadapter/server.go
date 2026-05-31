package mcpadapter

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/bridge"
	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/version"
)

// JSON-RPC 2.0 types
type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Tool definition for MCP
type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations,omitempty"`
}

// Annotations
var readOnlyAnnotations = map[string]any{
	"readOnlyHint":    true,
	"destructiveHint": false,
	"idempotentHint":  true,
	"openWorldHint":   false,
}

var mutatingAnnotations = map[string]any{
	"readOnlyHint":    false,
	"destructiveHint": false,
	"idempotentHint":  false,
	"openWorldHint":   false,
}

var idempotentMutatingAnnotations = map[string]any{
	"readOnlyHint":    false,
	"destructiveHint": false,
	"idempotentHint":  true,
	"openWorldHint":   false,
}

// Server is the MCP stdio server.
type Server struct {
	service *bridge.Service
	tools   []toolDef
	toolMap map[string]string // tool name → operation
}

// Serve creates and runs an MCP server on stdin/stdout.
func Serve(baseURL string) error {
	client := bridge.NewClient(baseURL)
	service := bridge.NewService(client)
	server := NewServer(service)
	return server.Run()
}

// NewServer creates an MCP server backed by the bridge service.
func NewServer(service *bridge.Service) *Server {
	s := &Server{
		service: service,
		toolMap: make(map[string]string),
	}
	s.registerTools()
	return s
}

func (s *Server) registerTools() {
	s.addTool("health", bridge.OpBridgeHealth,
		"Check SP connectivity and status.",
		map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
		readOnlyAnnotations)

	s.addTool("get_status", bridge.OpStatusGet,
		"Get current SP application status including active task and tracking state.",
		map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
		readOnlyAnnotations)

	s.addTool("list_tasks", bridge.OpTaskList,
		"List tasks with optional filters. Use list_projects or list_tags first to get IDs for filtering.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":       map[string]any{"type": "string", "description": "Filter by title substring (case-insensitive)."},
				"projectId":   map[string]any{"type": "string", "description": "Filter by project ID (from list_projects, not a name)."},
				"tagId":       map[string]any{"type": "string", "description": "Filter by tag ID (from list_tags). Use 'TODAY' for today's tasks."},
				"includeDone": map[string]any{"type": "boolean", "description": "Include completed tasks (default: false). Required to see done tasks even with source=all."},
				"source":      map[string]any{"type": "string", "enum": []string{"active", "archived", "all"}, "description": "Task pool to query (default: active). Note: archived tasks are typically done, so source=all without includeDone still shows only open tasks."},
			},
			"additionalProperties": false,
		},
		readOnlyAnnotations)

	s.addTool("get_task", bridge.OpTaskGet,
		"Get a task by ID.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "Task ID."},
			},
			"required":             []string{"id"},
			"additionalProperties": false,
		},
		readOnlyAnnotations)

	s.addTool("create_task", bridge.OpTaskCreate,
		"Create a new task. projectId and tagIds must be IDs from list_projects/list_tags, not names.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":        map[string]any{"type": "string", "description": "Task title (required)."},
				"notes":        map[string]any{"type": "string", "description": "Task notes/description."},
				"projectId":    map[string]any{"type": []any{"string", "null"}, "description": "Project ID to assign (from list_projects)."},
				"tagIds":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Tag IDs to assign (from list_tags)."},
				"plannedAt":    map[string]any{"description": "Planned date (ISO string or epoch ms)."},
				"dueDay":       map[string]any{"type": []any{"string", "null"}, "description": "Due date (YYYY-MM-DD)."},
				"dueWithTime":  map[string]any{"type": []any{"integer", "null"}, "description": "Due timestamp (epoch ms)."},
				"isDone":       map[string]any{"type": "boolean", "description": "Initial completion state."},
				"timeEstimate": map[string]any{"type": "integer", "minimum": 0, "description": "Time estimate (ms)."},
				"timeSpent":    map[string]any{"type": "integer", "minimum": 0, "description": "Time spent (ms)."},
				"parentId":     map[string]any{"type": "string", "description": "Parent task ID (creates subtask; cannot combine with projectId/tagIds)."},
			},
			"required":             []string{"title"},
			"additionalProperties": false,
		},
		mutatingAnnotations)

	s.addTool("update_task", bridge.OpTaskUpdate,
		"Update a task. Provide only fields to change.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":           map[string]any{"type": "string", "description": "Task ID (required)."},
				"title":        map[string]any{"type": "string", "description": "New title."},
				"notes":        map[string]any{"type": "string", "description": "New notes."},
				"projectId":    map[string]any{"type": []any{"string", "null"}, "description": "New project ID (from list_projects; null to clear)."},
				"tagIds":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "New tag IDs (from list_tags)."},
				"plannedAt":    map[string]any{"description": "Planned date (ISO string or epoch ms, null to clear)."},
				"dueDay":       map[string]any{"type": []any{"string", "null"}, "description": "Due date (YYYY-MM-DD, null to clear)."},
				"dueWithTime":  map[string]any{"type": []any{"integer", "null"}, "description": "Due timestamp (epoch ms, null to clear)."},
				"isDone":       map[string]any{"type": "boolean", "description": "Completion state."},
				"timeEstimate": map[string]any{"type": "integer", "minimum": 0, "description": "Time estimate (ms)."},
				"timeSpent":    map[string]any{"type": "integer", "minimum": 0, "description": "Time spent (ms)."},
			},
			"required":             []string{"id"},
			"additionalProperties": false,
		},
		mutatingAnnotations)

	s.addTool("complete_task", bridge.OpTaskComplete,
		"Mark a task as done.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "Task ID."},
			},
			"required":             []string{"id"},
			"additionalProperties": false,
		},
		idempotentMutatingAnnotations)

	s.addTool("uncomplete_task", bridge.OpTaskUncomplete,
		"Mark a task as not done.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "Task ID."},
			},
			"required":             []string{"id"},
			"additionalProperties": false,
		},
		idempotentMutatingAnnotations)

	s.addTool("start_task", bridge.OpTaskStart,
		"Start time tracking on a task.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "Task ID."},
			},
			"required":             []string{"id"},
			"additionalProperties": false,
		},
		mutatingAnnotations)

	s.addTool("stop_current_task", bridge.OpTaskStopCurrent,
		"Stop time tracking on the current task.",
		map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
		mutatingAnnotations)

	s.addTool("get_current_task", bridge.OpTaskGetCurrent,
		"Get the currently tracked task.",
		map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
		readOnlyAnnotations)

	s.addTool("set_current_task", bridge.OpTaskSetCurrent,
		"Set or clear the currently tracked task.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"taskId": map[string]any{"type": []any{"string", "null"}, "description": "Task ID to set as current, or null to clear."},
			},
			"required":             []string{"taskId"},
			"additionalProperties": false,
		},
		mutatingAnnotations)

	s.addTool("archive_task", bridge.OpTaskArchive,
		"Archive a task.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "Task ID."},
			},
			"required":             []string{"id"},
			"additionalProperties": false,
		},
		mutatingAnnotations)

	s.addTool("restore_task", bridge.OpTaskRestore,
		"Restore an archived task.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "Task ID."},
			},
			"required":             []string{"id"},
			"additionalProperties": false,
		},
		mutatingAnnotations)

	s.addTool("list_projects", bridge.OpProjectList,
		"List projects.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "Filter by name substring."},
			},
			"additionalProperties": false,
		},
		readOnlyAnnotations)

	s.addTool("list_tags", bridge.OpTagList,
		"List tags.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "Filter by name substring."},
			},
			"additionalProperties": false,
		},
		readOnlyAnnotations)
}

func (s *Server) addTool(name, operation, description string, schema map[string]any, annotations map[string]any) {
	s.tools = append(s.tools, toolDef{
		Name:        name,
		Description: description,
		InputSchema: schema,
		Annotations: annotations,
	})
	s.toolMap[name] = operation
}

// Run starts the MCP stdio server.
func (s *Server) Run() error {
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("reading stdin: %w", err)
		}
		if len(line) == 0 {
			continue
		}

		var req jsonrpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeError(nil, -32700, "Parse error", nil)
			continue
		}

		s.handleRequest(req)
	}
}

func (s *Server) handleRequest(req jsonrpcRequest) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "notifications/initialized":
		// No response needed for notifications
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(req)
	case "ping":
		s.writeResult(req.ID, map[string]any{})
	default:
		// Notifications don't get responses
		if req.ID == nil {
			return
		}
		s.writeError(req.ID, -32601, "Method not found", nil)
	}
}

func (s *Server) handleInitialize(req jsonrpcRequest) {
	result := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "sp-local-bridge",
			"version": version.Version,
		},
	}
	s.writeResult(req.ID, result)
}

func (s *Server) handleToolsList(req jsonrpcRequest) {
	result := map[string]any{
		"tools": s.tools,
	}
	s.writeResult(req.ID, result)
}

func (s *Server) handleToolsCall(req jsonrpcRequest) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.writeError(req.ID, -32602, "Invalid params", nil)
		return
	}

	operation, ok := s.toolMap[params.Name]
	if !ok {
		s.writeError(req.ID, -32602, fmt.Sprintf("Unknown tool: %s", params.Name), nil)
		return
	}

	// Parse arguments as payload
	var payload map[string]json.RawMessage
	if len(params.Arguments) > 0 && string(params.Arguments) != "null" && string(params.Arguments) != "{}" {
		if err := json.Unmarshal(params.Arguments, &payload); err != nil {
			s.writeError(req.ID, -32602, "Invalid tool arguments", nil)
			return
		}
	}

	// Execute operation
	ctx := context.Background()
	bridgeReq := bridge.Request{Operation: operation, Payload: payload}
	result := s.service.Execute(ctx, bridgeReq)

	// Convert to MCP tool result
	s.writeToolResult(req.ID, result)
}

func (s *Server) writeToolResult(id json.RawMessage, result bridge.Result) {
	var content []map[string]any
	if result.OK {
		data, _ := json.Marshal(result.Data)
		content = []map[string]any{
			{"type": "text", "text": string(data)},
		}
	} else {
		errJSON, _ := json.Marshal(map[string]any{
			"code":    result.Error.Code,
			"message": result.Error.Message,
			"details": result.Error.Details,
		})
		content = []map[string]any{
			{"type": "text", "text": string(errJSON)},
		}
	}
	resp := map[string]any{
		"content": content,
		"isError": !result.OK,
	}
	// Include structuredContent for successful results with data.
	// If data is already a map/object, pass through directly.
	// If data is a non-object (array, scalar), wrap as {"result": data}
	// to match Python SDK behavior and ensure host compatibility.
	if result.OK && result.Data != nil {
		if _, isMap := result.Data.(map[string]any); isMap {
			resp["structuredContent"] = result.Data
		} else {
			resp["structuredContent"] = map[string]any{"result": result.Data}
		}
	}
	s.writeResult(id, resp)
}

func (s *Server) writeResult(id json.RawMessage, result any) {
	resp := jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, _ := json.Marshal(resp)
	fmt.Fprintf(os.Stdout, "%s\n", data)
}

func (s *Server) writeError(id json.RawMessage, code int, message string, data any) {
	resp := jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &jsonrpcError{Code: code, Message: message, Data: data},
	}
	respData, _ := json.Marshal(resp)
	fmt.Fprintf(os.Stdout, "%s\n", respData)
}
