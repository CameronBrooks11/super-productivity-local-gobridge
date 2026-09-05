package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Handler is a function that handles a bridge operation.
type Handler func(ctx context.Context, client *Client, payload map[string]json.RawMessage) Result

// Service dispatches bridge operations to handlers.
type Service struct {
	client   *Client
	handlers map[string]Handler
}

// NewService creates a bridge service with all registered handlers.
func NewService(client *Client) *Service {
	s := &Service{client: client}
	s.handlers = map[string]Handler{
		OpTaskList:        handleTaskList,
		OpTaskGet:         handleTaskGet,
		OpTaskCreate:      handleTaskCreate,
		OpTaskUpdate:      handleTaskUpdate,
		OpTaskComplete:    handleTaskComplete,
		OpTaskUncomplete:  handleTaskUncomplete,
		OpTaskStart:       handleTaskStart,
		OpTaskStopCurrent: handleTaskStopCurrent,
		OpTaskGetCurrent:  handleTaskGetCurrent,
		OpTaskSetCurrent:  handleTaskSetCurrent,
		OpTaskArchive:     handleTaskArchive,
		OpTaskRestore:     handleTaskRestore,
		OpProjectList:     handleProjectList,
		OpTagList:         handleTagList,
		OpStatusGet:       handleStatusGet,
		OpBridgeHealth:    handleBridgeHealth,
	}
	return s
}

// Execute runs a bridge operation.
func (s *Service) Execute(ctx context.Context, req Request) Result {
	handler, ok := s.handlers[req.Operation]
	if !ok {
		return Failure(ErrUnknownOperation, fmt.Sprintf("Unknown operation: %s", req.Operation))
	}
	return handler(ctx, s.client, req.Payload)
}

// --- Operation handlers ---

func handleTaskList(ctx context.Context, client *Client, payload map[string]json.RawMessage) Result {
	params, errResult := validateTaskListFilters(payload)
	if errResult != nil {
		return *errResult
	}
	if len(params) == 0 {
		return client.ListTasks(ctx, nil)
	}
	return client.ListTasks(ctx, params)
}

func handleTaskGet(ctx context.Context, client *Client, payload map[string]json.RawMessage) Result {
	if r := validateIDOnly(payload); r != nil {
		return *r
	}
	id, r := extractID(payload)
	if r != nil {
		return *r
	}
	return client.GetTask(ctx, id)
}

func handleTaskCreate(ctx context.Context, client *Client, payload map[string]json.RawMessage) Result {
	// Check for disallowed "id" field
	if _, ok := payload["id"]; ok {
		return Failure(ErrInvalidInput, "Field 'id' is not allowed on task.create")
	}

	// Validate title is present and non-empty
	title, err := getNonEmptyString(payload, "title")
	if err != nil {
		return Failure(ErrInvalidInput, "Missing required field: title (non-empty string)")
	}

	// Validate all fields
	body, r := validateTaskFields(payload, taskWritableFields, nil)
	if r != nil {
		return *r
	}

	// Ensure title is set
	body["title"] = title

	// parentId constraint: cannot combine with projectId or tagIds
	if _, hasParent := body["parentId"]; hasParent {
		if _, hasProjID := body["projectId"]; hasProjID {
			return Failure(ErrInvalidInput, "Cannot set projectId or tagIds when parentId is specified (subtasks inherit from parent)")
		}
		if _, hasTagIDs := body["tagIds"]; hasTagIDs {
			return Failure(ErrInvalidInput, "Cannot set projectId or tagIds when parentId is specified (subtasks inherit from parent)")
		}
	}

	return client.CreateTask(ctx, body)
}

func handleTaskUpdate(ctx context.Context, client *Client, payload map[string]json.RawMessage) Result {
	// Extract and validate id
	id, err := getNonEmptyString(payload, "id")
	if err != nil {
		return Failure(ErrInvalidInput, "Missing required field: id (non-empty string)")
	}

	// Must have at least one field besides id
	if len(payload) <= 1 {
		return Failure(ErrInvalidInput, "No fields to update")
	}

	// parentId not allowed on update
	if _, ok := payload["parentId"]; ok {
		return Failure(ErrInvalidInput, "Field 'parentId' is not allowed on task.update")
	}

	// Validate fields (exclude id from field checks)
	body, r := validateTaskFields(payload, taskUpdateFields, map[string]bool{"id": true})
	if r != nil {
		return *r
	}

	_ = id
	return client.UpdateTask(ctx, id, body)
}

func handleTaskComplete(ctx context.Context, client *Client, payload map[string]json.RawMessage) Result {
	if r := validateIDOnly(payload); r != nil {
		return *r
	}
	id, r := extractID(payload)
	if r != nil {
		return *r
	}
	return client.UpdateTask(ctx, id, map[string]any{"isDone": true})
}

func handleTaskUncomplete(ctx context.Context, client *Client, payload map[string]json.RawMessage) Result {
	if r := validateIDOnly(payload); r != nil {
		return *r
	}
	id, r := extractID(payload)
	if r != nil {
		return *r
	}
	return client.UpdateTask(ctx, id, map[string]any{"isDone": false})
}

func handleTaskStart(ctx context.Context, client *Client, payload map[string]json.RawMessage) Result {
	if r := validateIDOnly(payload); r != nil {
		return *r
	}
	id, r := extractID(payload)
	if r != nil {
		return *r
	}
	return client.StartTask(ctx, id)
}

func handleTaskStopCurrent(ctx context.Context, client *Client, payload map[string]json.RawMessage) Result {
	if r := validateNoPayload(payload); r != nil {
		return *r
	}
	return client.StopCurrentTask(ctx)
}

func handleTaskGetCurrent(ctx context.Context, client *Client, payload map[string]json.RawMessage) Result {
	if r := validateNoPayload(payload); r != nil {
		return *r
	}
	return client.GetCurrentTask(ctx)
}

func handleTaskSetCurrent(ctx context.Context, client *Client, payload map[string]json.RawMessage) Result {
	// taskId is required
	raw, present := payload["taskId"]
	if !present {
		return Failure(ErrInvalidInput, "Field 'taskId' is required (string to set, null to clear)")
	}

	// No extra fields
	extra := extraKeys(payload, map[string]bool{"taskId": true})
	if len(extra) > 0 {
		return Failure(ErrInvalidInput, fmt.Sprintf("This operation only accepts 'taskId', got extra: %s", strings.Join(extra, ", ")))
	}

	// null → clear
	if string(raw) == "null" {
		return client.SetCurrentTask(ctx, nil)
	}

	// Must be a non-empty string
	var taskID string
	if err := json.Unmarshal(raw, &taskID); err != nil {
		return Failure(ErrInvalidInput, "Field 'taskId' must be a non-empty string or null (to clear)")
	}
	if strings.TrimSpace(taskID) == "" {
		return Failure(ErrInvalidInput, "Field 'taskId' must be a non-empty string or null (to clear)")
	}
	return client.SetCurrentTask(ctx, &taskID)
}

func handleTaskArchive(ctx context.Context, client *Client, payload map[string]json.RawMessage) Result {
	if r := validateIDOnly(payload); r != nil {
		return *r
	}
	id, r := extractID(payload)
	if r != nil {
		return *r
	}

	// SP's archive route answers {"ok":true,"archived":true} for ids that never
	// existed, unlike get, update, start and restore, which all return
	// TASK_NOT_FOUND. Reporting success for a task that is not there is worse
	// than a wrong exit code here: agents invent plausible-looking ids, and this
	// is the one operation where an invented one produces a confident success,
	// leaving the model no signal to correct itself.
	//
	// The check is a plain read-before-write. Its TOCTOU window does not matter:
	// if the task disappears in between, the archive is a no-op either way.
	if existing := client.GetTask(ctx, id); !existing.OK {
		if existing.Error != nil && existing.Error.Code == ErrTaskNotFound {
			// Restate it in terms of what was attempted. The client's generic
			// "Resource not found." leaves the caller guessing whether the task
			// or the route was missing, and whether anything happened.
			return Failure(ErrTaskNotFound,
				"Task not found in the active list; nothing was archived. "+
					"An already-archived task reports this too.")
		}
		// Anything else — SP unreachable, a timeout — passes through unchanged
		// rather than being recast as a missing task.
		return existing
	}
	return client.ArchiveTask(ctx, id)
}

func handleTaskRestore(ctx context.Context, client *Client, payload map[string]json.RawMessage) Result {
	if r := validateIDOnly(payload); r != nil {
		return *r
	}
	id, r := extractID(payload)
	if r != nil {
		return *r
	}
	return client.RestoreTask(ctx, id)
}

func handleProjectList(ctx context.Context, client *Client, payload map[string]json.RawMessage) Result {
	params, r := validateQueryOnly(payload)
	if r != nil {
		return *r
	}
	if len(params) == 0 {
		return client.ListProjects(ctx, nil)
	}
	return client.ListProjects(ctx, params)
}

func handleTagList(ctx context.Context, client *Client, payload map[string]json.RawMessage) Result {
	params, r := validateQueryOnly(payload)
	if r != nil {
		return *r
	}
	if len(params) == 0 {
		return client.ListTags(ctx, nil)
	}
	return client.ListTags(ctx, params)
}

func handleStatusGet(ctx context.Context, client *Client, payload map[string]json.RawMessage) Result {
	if r := validateNoPayload(payload); r != nil {
		return *r
	}
	return client.Status(ctx)
}

func handleBridgeHealth(ctx context.Context, client *Client, payload map[string]json.RawMessage) Result {
	if r := validateNoPayload(payload); r != nil {
		return *r
	}
	health := client.Health(ctx)
	if !health.OK {
		return health
	}
	status := client.Status(ctx)
	if !status.OK {
		return Failure(ErrSPError, "SP is reachable but status check failed.",
			map[string]any{"health": health.Data, "status_error": status.Error.Code})
	}
	return Success(map[string]any{"health": health.Data, "status": status.Data})
}
