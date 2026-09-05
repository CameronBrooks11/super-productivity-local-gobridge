package bridge

// Error codes returned by the bridge.
const (
	ErrSPUnavailable        = "SP_UNAVAILABLE"
	ErrTimeout              = "TIMEOUT"
	ErrUnknownOperation     = "UNKNOWN_OPERATION"
	ErrUnsupportedOperation = "UNSUPPORTED_OPERATION"
	ErrInvalidInput         = "INVALID_INPUT"
	ErrTaskNotFound         = "TASK_NOT_FOUND"
	// ErrNotFound is Super Productivity's code for a route that does not exist,
	// as distinct from a task that does not exist. Both arrive as HTTP 404.
	ErrNotFound        = "NOT_FOUND"
	ErrProjectNotFound = "PROJECT_NOT_FOUND"
	ErrSPError         = "SP_ERROR"
	ErrInternalError   = "INTERNAL_ERROR"
)
