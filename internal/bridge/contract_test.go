package bridge

import (
	"testing"
)

// contractOperations defines the exact v0.2.0 operation set.
// Any addition or removal is a deliberate contract change.
var contractOperations = []string{
	"task.list",
	"task.get",
	"task.create",
	"task.update",
	"task.complete",
	"task.uncomplete",
	"task.start",
	"task.stop_current",
	"task.get_current",
	"task.set_current",
	"task.archive",
	"task.restore",
	"project.list",
	"tag.list",
	"status.get",
	"bridge.health",
}

// contractErrorCodes defines the error codes the bridge may return.
var contractErrorCodes = []string{
	ErrSPUnavailable,
	ErrTimeout,
	ErrUnknownOperation,
	ErrUnsupportedOperation,
	ErrInvalidInput,
	ErrTaskNotFound,
	ErrProjectNotFound,
	ErrSPError,
	ErrInternalError,
}

func TestContract_AllOperationsHaveHandlers(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	service := NewService(client)
	for _, op := range contractOperations {
		if _, ok := service.handlers[op]; !ok {
			t.Errorf("contract operation %q has no handler", op)
		}
	}
}

func TestContract_NoExtraHandlers(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	service := NewService(client)
	contractSet := make(map[string]bool, len(contractOperations))
	for _, op := range contractOperations {
		contractSet[op] = true
	}
	for op := range service.handlers {
		if !contractSet[op] {
			t.Errorf("handler %q is not in the contract — add to contract or remove", op)
		}
	}
}

func TestContract_Exactly16Operations(t *testing.T) {
	if len(contractOperations) != 16 {
		t.Errorf("expected 16 contract operations, got %d", len(contractOperations))
	}
	client := NewClient("http://127.0.0.1:1")
	service := NewService(client)
	if len(service.handlers) != 16 {
		t.Errorf("expected 16 registered handlers, got %d", len(service.handlers))
	}
}

func TestContract_TaskDeleteNotExposed(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	service := NewService(client)
	if _, ok := service.handlers["task.delete"]; ok {
		t.Error("task.delete must not be exposed (intentionally excluded)")
	}
}

func TestContract_ErrorCodesAreDefined(t *testing.T) {
	// Ensure all contract error codes are non-empty.
	for _, code := range contractErrorCodes {
		if code == "" {
			t.Error("empty error code in contract")
		}
	}
}

func TestContract_OperationConstants(t *testing.T) {
	// Verify that the named constants match the expected strings.
	expected := map[string]string{
		"OpTaskList":        "task.list",
		"OpTaskGet":         "task.get",
		"OpTaskCreate":      "task.create",
		"OpTaskUpdate":      "task.update",
		"OpTaskComplete":    "task.complete",
		"OpTaskUncomplete":  "task.uncomplete",
		"OpTaskStart":       "task.start",
		"OpTaskStopCurrent": "task.stop_current",
		"OpTaskGetCurrent":  "task.get_current",
		"OpTaskSetCurrent":  "task.set_current",
		"OpTaskArchive":     "task.archive",
		"OpTaskRestore":     "task.restore",
		"OpProjectList":     "project.list",
		"OpTagList":         "tag.list",
		"OpStatusGet":       "status.get",
		"OpBridgeHealth":    "bridge.health",
	}
	actual := map[string]string{
		"OpTaskList":        OpTaskList,
		"OpTaskGet":         OpTaskGet,
		"OpTaskCreate":      OpTaskCreate,
		"OpTaskUpdate":      OpTaskUpdate,
		"OpTaskComplete":    OpTaskComplete,
		"OpTaskUncomplete":  OpTaskUncomplete,
		"OpTaskStart":       OpTaskStart,
		"OpTaskStopCurrent": OpTaskStopCurrent,
		"OpTaskGetCurrent":  OpTaskGetCurrent,
		"OpTaskSetCurrent":  OpTaskSetCurrent,
		"OpTaskArchive":     OpTaskArchive,
		"OpTaskRestore":     OpTaskRestore,
		"OpProjectList":     OpProjectList,
		"OpTagList":         OpTagList,
		"OpStatusGet":       OpStatusGet,
		"OpBridgeHealth":    OpBridgeHealth,
	}
	for name, exp := range expected {
		if actual[name] != exp {
			t.Errorf("constant %s = %q, want %q", name, actual[name], exp)
		}
	}
}
