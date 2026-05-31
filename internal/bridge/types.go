package bridge

import "encoding/json"

// DefaultBaseURL is the SP Local REST API default address.
const DefaultBaseURL = "http://127.0.0.1:3876"

// Result is the bridge response shape.
type Result struct {
	OK    bool         `json:"ok"`
	Data  any          `json:"data,omitempty"`
	Error *BridgeError `json:"error,omitempty"`
}

// BridgeError is the structured error in a failed Result.
type BridgeError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// Success creates a successful result.
func Success(data any) Result {
	return Result{OK: true, Data: data}
}

// Failure creates an error result.
func Failure(code, message string, details ...map[string]any) Result {
	var d map[string]any
	if len(details) > 0 && details[0] != nil {
		d = details[0]
	}
	return Result{OK: false, Error: &BridgeError{Code: code, Message: message, Details: d}}
}

// Request is a bridge operation request.
type Request struct {
	Operation string
	Payload   map[string]json.RawMessage
}
