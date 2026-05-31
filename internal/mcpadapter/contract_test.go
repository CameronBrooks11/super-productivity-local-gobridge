package mcpadapter

import (
	"testing"

	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/bridge"
)

// mcpContractTools defines the exact MCP tool names and their expected operations.
var mcpContractTools = map[string]string{
	"health":            bridge.OpBridgeHealth,
	"get_status":        bridge.OpStatusGet,
	"list_tasks":        bridge.OpTaskList,
	"get_task":          bridge.OpTaskGet,
	"create_task":       bridge.OpTaskCreate,
	"update_task":       bridge.OpTaskUpdate,
	"complete_task":     bridge.OpTaskComplete,
	"uncomplete_task":   bridge.OpTaskUncomplete,
	"start_task":        bridge.OpTaskStart,
	"stop_current_task": bridge.OpTaskStopCurrent,
	"get_current_task":  bridge.OpTaskGetCurrent,
	"set_current_task":  bridge.OpTaskSetCurrent,
	"archive_task":      bridge.OpTaskArchive,
	"restore_task":      bridge.OpTaskRestore,
	"list_projects":     bridge.OpProjectList,
	"list_tags":         bridge.OpTagList,
}

func TestMCPContract_Exactly16Tools(t *testing.T) {
	server := newTestServer(t, nil)
	if len(server.tools) != 16 {
		t.Errorf("expected 16 tools, got %d", len(server.tools))
	}
	if len(server.toolMap) != 16 {
		t.Errorf("expected 16 tool mappings, got %d", len(server.toolMap))
	}
}

func TestMCPContract_ToolOperationMapping(t *testing.T) {
	server := newTestServer(t, nil)
	for toolName, expectedOp := range mcpContractTools {
		op, ok := server.toolMap[toolName]
		if !ok {
			t.Errorf("tool %q not registered", toolName)
			continue
		}
		if op != expectedOp {
			t.Errorf("tool %q maps to %q, want %q", toolName, op, expectedOp)
		}
	}
}

func TestMCPContract_NoExtraTools(t *testing.T) {
	server := newTestServer(t, nil)
	for name := range server.toolMap {
		if _, ok := mcpContractTools[name]; !ok {
			t.Errorf("tool %q is registered but not in contract", name)
		}
	}
}

func TestMCPContract_AllToolsHaveAnnotations(t *testing.T) {
	server := newTestServer(t, nil)
	for _, tool := range server.tools {
		if tool.Annotations == nil {
			t.Errorf("tool %q has no annotations", tool.Name)
			continue
		}
		for _, key := range []string{"readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint"} {
			if _, ok := tool.Annotations[key]; !ok {
				t.Errorf("tool %q missing annotation %q", tool.Name, key)
			}
		}
	}
}

func TestMCPContract_OpenWorldHintFalse(t *testing.T) {
	server := newTestServer(t, nil)
	for _, tool := range server.tools {
		hint, ok := tool.Annotations["openWorldHint"]
		if !ok {
			t.Errorf("tool %q missing openWorldHint", tool.Name)
			continue
		}
		if hint != false {
			t.Errorf("tool %q: openWorldHint = %v, want false", tool.Name, hint)
		}
	}
}

func TestMCPContract_DestructiveHintFalse(t *testing.T) {
	server := newTestServer(t, nil)
	for _, tool := range server.tools {
		hint, ok := tool.Annotations["destructiveHint"]
		if !ok {
			continue
		}
		if hint != false {
			t.Errorf("tool %q: destructiveHint = %v, want false (no destructive ops)", tool.Name, hint)
		}
	}
}

func TestMCPContract_ReadOnlyAnnotations(t *testing.T) {
	readOnly := map[string]bool{
		"health": true, "get_status": true, "list_tasks": true,
		"get_task": true, "get_current_task": true,
		"list_projects": true, "list_tags": true,
	}
	server := newTestServer(t, nil)
	for _, tool := range server.tools {
		hint := tool.Annotations["readOnlyHint"]
		if readOnly[tool.Name] {
			if hint != true {
				t.Errorf("tool %q should be readOnly, got readOnlyHint=%v", tool.Name, hint)
			}
		} else {
			if hint != false {
				t.Errorf("tool %q should NOT be readOnly, got readOnlyHint=%v", tool.Name, hint)
			}
		}
	}
}

func TestMCPContract_IdempotentAnnotations(t *testing.T) {
	idempotent := map[string]bool{
		"health": true, "get_status": true, "list_tasks": true,
		"get_task": true, "get_current_task": true,
		"list_projects": true, "list_tags": true,
		"complete_task": true, "uncomplete_task": true,
	}
	server := newTestServer(t, nil)
	for _, tool := range server.tools {
		hint := tool.Annotations["idempotentHint"]
		if idempotent[tool.Name] {
			if hint != true {
				t.Errorf("tool %q should be idempotent, got idempotentHint=%v", tool.Name, hint)
			}
		} else {
			if hint != false {
				t.Errorf("tool %q should NOT be idempotent, got idempotentHint=%v", tool.Name, hint)
			}
		}
	}
}

func TestMCPContract_AllToolsHaveInputSchema(t *testing.T) {
	server := newTestServer(t, nil)
	for _, tool := range server.tools {
		if tool.InputSchema == nil {
			t.Errorf("tool %q has no inputSchema", tool.Name)
			continue
		}
		typ, ok := tool.InputSchema["type"]
		if !ok || typ != "object" {
			t.Errorf("tool %q: inputSchema type=%v, want object", tool.Name, typ)
		}
		// additionalProperties must be false
		ap, ok := tool.InputSchema["additionalProperties"]
		if !ok || ap != false {
			t.Errorf("tool %q: additionalProperties=%v, want false", tool.Name, ap)
		}
	}
}

func TestMCPContract_ToolsWithRequiredID(t *testing.T) {
	requiresID := []string{
		"get_task", "complete_task", "uncomplete_task",
		"start_task", "archive_task", "restore_task", "update_task",
	}
	server := newTestServer(t, nil)
	toolByName := make(map[string]toolDef)
	for _, tool := range server.tools {
		toolByName[tool.Name] = tool
	}
	for _, name := range requiresID {
		tool := toolByName[name]
		req, ok := tool.InputSchema["required"].([]string)
		if !ok {
			t.Errorf("tool %q: required is not []string", name)
			continue
		}
		found := false
		for _, r := range req {
			if r == "id" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("tool %q should require 'id'", name)
		}
	}
}

func TestMCPContract_CreateTaskRequiresTitle(t *testing.T) {
	server := newTestServer(t, nil)
	for _, tool := range server.tools {
		if tool.Name != "create_task" {
			continue
		}
		req, ok := tool.InputSchema["required"].([]string)
		if !ok {
			t.Fatal("create_task required is not []string")
		}
		found := false
		for _, r := range req {
			if r == "title" {
				found = true
				break
			}
		}
		if !found {
			t.Error("create_task should require 'title'")
		}
	}
}

func TestMCPContract_SetCurrentTaskRequiresTaskId(t *testing.T) {
	server := newTestServer(t, nil)
	for _, tool := range server.tools {
		if tool.Name != "set_current_task" {
			continue
		}
		req, ok := tool.InputSchema["required"].([]string)
		if !ok {
			t.Fatal("set_current_task required is not []string")
		}
		found := false
		for _, r := range req {
			if r == "taskId" {
				found = true
				break
			}
		}
		if !found {
			t.Error("set_current_task should require 'taskId'")
		}
	}
}

func TestMCPContract_NoToolExposesDelete(t *testing.T) {
	server := newTestServer(t, nil)
	for _, tool := range server.tools {
		if tool.Name == "delete_task" {
			t.Error("delete_task must not be exposed")
		}
	}
	if _, ok := server.toolMap["delete_task"]; ok {
		t.Error("delete_task must not be in toolMap")
	}
}

func TestMCPContract_AllToolsHaveDescription(t *testing.T) {
	server := newTestServer(t, nil)
	for _, tool := range server.tools {
		if tool.Description == "" {
			t.Errorf("tool %q has empty description", tool.Name)
		}
	}
}
