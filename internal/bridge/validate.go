package bridge

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// PayloadValue represents a decoded JSON value with three-state awareness:
// missing, null, or a concrete value.
type PayloadValue struct {
	Raw     json.RawMessage
	Present bool
}

// IsNull returns true if the value is JSON null.
func (pv PayloadValue) IsNull() bool {
	return pv.Present && string(pv.Raw) == "null"
}

// --- Payload field extraction helpers ---

// getString extracts a string field. Returns (value, isNull, error).
// If the field is missing, returns ("", false, nil) and present=false.
func getString(payload map[string]json.RawMessage, key string) (string, bool, bool, error) {
	raw, present := payload[key]
	if !present {
		return "", false, false, nil
	}
	if string(raw) == "null" {
		return "", true, true, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false, true, fmt.Errorf("field '%s' must be a string", key)
	}
	return s, false, true, nil
}

// getNonEmptyString extracts a required non-empty string field.
func getNonEmptyString(payload map[string]json.RawMessage, key string) (string, error) {
	raw, present := payload[key]
	if !present {
		return "", fmt.Errorf("Missing required field: %s (non-empty string)", key)
	}
	if string(raw) == "null" {
		return "", fmt.Errorf("Missing required field: %s (non-empty string)", key)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("Missing required field: %s (non-empty string)", key)
	}
	if strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("Missing required field: %s (non-empty string)", key)
	}
	return s, nil
}

// getNullableString extracts a string|null field.
func getNullableString(payload map[string]json.RawMessage, key string) (string, bool, bool, error) {
	return getString(payload, key)
}

// getInt extracts an integer field. Rejects booleans, floats, and strings.
// Uses raw string parsing to avoid float64 precision loss for large integers.
func getInt(payload map[string]json.RawMessage, key string) (int64, bool, bool, error) {
	raw, present := payload[key]
	if !present {
		return 0, false, false, nil
	}
	if string(raw) == "null" {
		return 0, true, true, nil
	}
	// Reject booleans
	if string(raw) == "true" || string(raw) == "false" {
		return 0, false, true, fmt.Errorf("field '%s' must be int, got bool", key)
	}
	s := strings.TrimSpace(string(raw))
	// Reject strings, arrays, objects
	if len(s) > 0 && (s[0] == '"' || s[0] == '[' || s[0] == '{') {
		return 0, false, true, fmt.Errorf("field '%s' must be int, got %s", key, describeRawType(raw))
	}
	// Reject fractional values (contains '.')
	if strings.Contains(s, ".") {
		return 0, false, true, fmt.Errorf("field '%s' must be int, got float", key)
	}
	// Reject exponent notation — integer fields must be plain integers
	if strings.ContainsAny(s, "eE") {
		return 0, false, true, fmt.Errorf("field '%s' must be int, got float", key)
	}
	// Parse as plain integer (handles negatives)
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false, true, fmt.Errorf("field '%s' must be int, got %s", key, describeRawType(raw))
	}
	return v, false, true, nil
}

// getNonNegativeInt extracts a non-negative integer (for timeEstimate, timeSpent).
func getNonNegativeInt(payload map[string]json.RawMessage, key string) (int64, bool, error) {
	val, isNull, present, err := getInt(payload, key)
	if err != nil {
		return 0, false, err
	}
	if !present {
		return 0, false, nil
	}
	if isNull {
		return 0, false, fmt.Errorf("field '%s' must be int, got NoneType", key)
	}
	if val < 0 {
		return 0, false, fmt.Errorf("Field '%s' must be non-negative (milliseconds), got %d", key, val)
	}
	return val, true, nil
}

// getBool extracts a boolean field.
func getBool(payload map[string]json.RawMessage, key string) (bool, bool, error) {
	raw, present := payload[key]
	if !present {
		return false, false, nil
	}
	if string(raw) == "null" {
		return false, false, fmt.Errorf("field '%s' must be bool, got NoneType", key)
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return false, false, fmt.Errorf("field '%s' must be bool, got %s", key, describeRawType(raw))
	}
	// Double-check it's actually true/false, not a number that happened to unmarshal
	s := string(raw)
	if s != "true" && s != "false" {
		return false, false, fmt.Errorf("field '%s' must be bool, got %s", key, describeRawType(raw))
	}
	return b, true, nil
}

// getStringArray extracts a []string field.
func getStringArray(payload map[string]json.RawMessage, key string) ([]string, bool, error) {
	raw, present := payload[key]
	if !present {
		return nil, false, nil
	}
	if string(raw) == "null" {
		return nil, false, fmt.Errorf("field '%s' must be array of strings, got NoneType", key)
	}
	// First check it's an array
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, false, fmt.Errorf("field '%s' must be array of strings, got %s", key, describeRawType(raw))
	}
	strs := make([]string, 0, len(arr))
	for _, item := range arr {
		var s string
		if err := json.Unmarshal(item, &s); err != nil {
			return nil, false, fmt.Errorf("tagIds must contain only strings")
		}
		strs = append(strs, s)
	}
	return strs, true, nil
}

// describeRawType returns a type name for an unrecognized JSON raw value.
func describeRawType(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "null" {
		return "NoneType"
	}
	if s == "true" || s == "false" {
		return "bool"
	}
	if len(s) > 0 && s[0] == '"' {
		return "str"
	}
	if len(s) > 0 && s[0] == '[' {
		return "list"
	}
	if len(s) > 0 && s[0] == '{' {
		return "dict"
	}
	// Probably a number
	return "number"
}

// --- Payload validation functions ---

// validateNoPayload rejects any payload fields.
func validateNoPayload(payload map[string]json.RawMessage) *Result {
	if len(payload) > 0 {
		keys := sortedKeys(payload)
		r := Failure(ErrInvalidInput, fmt.Sprintf("This operation takes no payload, got: %s", strings.Join(keys, ", ")))
		return &r
	}
	return nil
}

// validateIDOnly allows only the "id" key.
func validateIDOnly(payload map[string]json.RawMessage) *Result {
	extra := extraKeys(payload, map[string]bool{"id": true})
	if len(extra) > 0 {
		r := Failure(ErrInvalidInput, fmt.Sprintf("This operation only accepts 'id', got extra: %s", strings.Join(extra, ", ")))
		return &r
	}
	return nil
}

// extractID validates and extracts the "id" field.
func extractID(payload map[string]json.RawMessage) (string, *Result) {
	id, err := getNonEmptyString(payload, "id")
	if err != nil {
		r := Failure(ErrInvalidInput, err.Error())
		return "", &r
	}
	return id, nil
}

// Valid task writable fields (create).
var taskWritableFields = map[string]bool{
	"title": true, "notes": true, "projectId": true, "tagIds": true,
	"plannedAt": true, "dueDay": true, "dueWithTime": true, "isDone": true,
	"timeEstimate": true, "timeSpent": true, "parentId": true,
}

// Valid task update fields (no parentId).
var taskUpdateFields = map[string]bool{
	"title": true, "notes": true, "projectId": true, "tagIds": true,
	"plannedAt": true, "dueDay": true, "dueWithTime": true, "isDone": true,
	"timeEstimate": true, "timeSpent": true,
}

// validateTaskFields validates task payload fields and returns the clean body or error.
func validateTaskFields(payload map[string]json.RawMessage, allowedFields map[string]bool, excludeKeys map[string]bool) (map[string]any, *Result) {
	// Check for unknown fields
	for k := range payload {
		if excludeKeys[k] {
			continue
		}
		if !allowedFields[k] {
			r := Failure(ErrInvalidInput, fmt.Sprintf("Unknown fields: %s", k))
			return nil, &r
		}
	}

	body := make(map[string]any)

	// title
	if raw, ok := payload["title"]; ok {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil || string(raw) == "null" {
			r := Failure(ErrInvalidInput, "Field 'title' must be str, got "+describeRawType(raw))
			return nil, &r
		}
		body["title"] = s
	}

	// notes
	if raw, ok := payload["notes"]; ok {
		if string(raw) == "null" {
			r := Failure(ErrInvalidInput, "Field 'notes' must be str, got NoneType")
			return nil, &r
		}
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			r := Failure(ErrInvalidInput, "Field 'notes' must be str, got "+describeRawType(raw))
			return nil, &r
		}
		body["notes"] = s
	}

	// projectId (string|null)
	if raw, ok := payload["projectId"]; ok {
		if string(raw) == "null" {
			body["projectId"] = nil
		} else {
			var s string
			if err := json.Unmarshal(raw, &s); err != nil {
				r := Failure(ErrInvalidInput, "Field 'projectId' must be str | NoneType, got "+describeRawType(raw))
				return nil, &r
			}
			body["projectId"] = s
		}
	}

	// tagIds ([]string)
	if _, ok := payload["tagIds"]; ok {
		tags, _, err := getStringArray(payload, "tagIds")
		if err != nil {
			r := Failure(ErrInvalidInput, err.Error())
			return nil, &r
		}
		body["tagIds"] = tags
	}

	// plannedAt (string|int|null)
	if raw, ok := payload["plannedAt"]; ok {
		if string(raw) == "null" {
			body["plannedAt"] = nil
		} else if string(raw) == "true" || string(raw) == "false" {
			r := Failure(ErrInvalidInput, "Field 'plannedAt' must be str | int | NoneType, got bool")
			return nil, &r
		} else {
			// Try string first
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				body["plannedAt"] = s
			} else {
				// Try number via raw string parsing
				numStr := strings.TrimSpace(string(raw))
				if strings.Contains(numStr, ".") {
					r := Failure(ErrInvalidInput, "Field 'plannedAt' must be str | int | NoneType, got float")
					return nil, &r
				}
				if v, err := strconv.ParseInt(numStr, 10, 64); err == nil {
					body["plannedAt"] = v
				} else {
					r := Failure(ErrInvalidInput, "Field 'plannedAt' must be str | int | NoneType, got "+describeRawType(raw))
					return nil, &r
				}
			}
		}
	}

	// dueDay (string|null)
	if raw, ok := payload["dueDay"]; ok {
		if string(raw) == "null" {
			body["dueDay"] = nil
		} else {
			var s string
			if err := json.Unmarshal(raw, &s); err != nil {
				r := Failure(ErrInvalidInput, "Field 'dueDay' must be str | NoneType, got "+describeRawType(raw))
				return nil, &r
			}
			body["dueDay"] = s
		}
	}

	// dueWithTime (int|null)
	if raw, ok := payload["dueWithTime"]; ok {
		if string(raw) == "null" {
			body["dueWithTime"] = nil
		} else if string(raw) == "true" || string(raw) == "false" {
			r := Failure(ErrInvalidInput, "Field 'dueWithTime' must be int | NoneType, got bool")
			return nil, &r
		} else {
			numStr := strings.TrimSpace(string(raw))
			if strings.Contains(numStr, ".") {
				r := Failure(ErrInvalidInput, "Field 'dueWithTime' must be int | NoneType, got float")
				return nil, &r
			}
			v, err := strconv.ParseInt(numStr, 10, 64)
			if err != nil {
				r := Failure(ErrInvalidInput, "Field 'dueWithTime' must be int | NoneType, got "+describeRawType(raw))
				return nil, &r
			}
			body["dueWithTime"] = v
		}
	}

	// isDone (bool)
	if _, ok := payload["isDone"]; ok {
		b, _, err := getBool(payload, "isDone")
		if err != nil {
			r := Failure(ErrInvalidInput, err.Error())
			return nil, &r
		}
		body["isDone"] = b
	}

	// timeEstimate (non-negative int)
	if _, ok := payload["timeEstimate"]; ok {
		val, present, err := getNonNegativeInt(payload, "timeEstimate")
		if err != nil {
			r := Failure(ErrInvalidInput, err.Error())
			return nil, &r
		}
		if present {
			body["timeEstimate"] = val
		}
	}

	// timeSpent (non-negative int)
	if _, ok := payload["timeSpent"]; ok {
		val, present, err := getNonNegativeInt(payload, "timeSpent")
		if err != nil {
			r := Failure(ErrInvalidInput, err.Error())
			return nil, &r
		}
		if present {
			body["timeSpent"] = val
		}
	}

	// parentId (string, create-only)
	if raw, ok := payload["parentId"]; ok {
		if string(raw) == "null" {
			r := Failure(ErrInvalidInput, "Field 'parentId' must be a non-empty string")
			return nil, &r
		}
		var s string
		if err := json.Unmarshal(raw, &s); err != nil || strings.TrimSpace(s) == "" {
			r := Failure(ErrInvalidInput, "Field 'parentId' must be a non-empty string")
			return nil, &r
		}
		body["parentId"] = s
	}

	return body, nil
}

// --- List filter validation ---

var taskListFilterFields = map[string]bool{
	"query": true, "projectId": true, "tagId": true,
	"includeDone": true, "source": true,
	// Applied by the bridge after SP responds, never forwarded: SP ignores
	// limit, offset and field selection entirely.
	"limit": true, "offset": true, "full": true,
}

var validSourceValues = map[string]bool{
	"active": true, "archived": true, "all": true,
}

// validateTaskListFilters validates task list filter payload and returns query params.
func validateTaskListFilters(payload map[string]json.RawMessage) (map[string]string, *Result) {
	// Check for unknown fields
	unknown := extraKeys(payload, taskListFilterFields)
	if len(unknown) > 0 {
		r := Failure(ErrInvalidInput, fmt.Sprintf("Unknown filter fields: %s", strings.Join(unknown, ", ")))
		return nil, &r
	}

	params := make(map[string]string)

	// query (string)
	if raw, ok := payload["query"]; ok {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil || string(raw) == "null" {
			r := Failure(ErrInvalidInput, "Filter 'query' must be str, got "+describeRawType(raw))
			return nil, &r
		}
		params["query"] = s
	}

	// projectId (string)
	if raw, ok := payload["projectId"]; ok {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil || string(raw) == "null" {
			r := Failure(ErrInvalidInput, "Filter 'projectId' must be str, got "+describeRawType(raw))
			return nil, &r
		}
		params["projectId"] = s
	}

	// tagId (string)
	if raw, ok := payload["tagId"]; ok {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil || string(raw) == "null" {
			r := Failure(ErrInvalidInput, "Filter 'tagId' must be str, got "+describeRawType(raw))
			return nil, &r
		}
		params["tagId"] = s
	}

	// includeDone (bool)
	if raw, ok := payload["includeDone"]; ok {
		s := string(raw)
		if s != "true" && s != "false" {
			r := Failure(ErrInvalidInput, "Filter 'includeDone' must be bool, got "+describeRawType(raw))
			return nil, &r
		}
		if s == "true" {
			params["includeDone"] = "true"
		} else {
			params["includeDone"] = "false"
		}
	}

	// source (string enum)
	if raw, ok := payload["source"]; ok {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil || string(raw) == "null" {
			r := Failure(ErrInvalidInput, "Filter 'source' must be str, got "+describeRawType(raw))
			return nil, &r
		}
		if !validSourceValues[s] {
			r := Failure(ErrInvalidInput, "Filter 'source' must be one of: active, all, archived")
			return nil, &r
		}
		params["source"] = s
	}

	return params, nil
}

// validateQueryOnly validates the payload for project.list and tag.list: an
// optional 'query' filter plus the bridge-side shaping options (limit, offset,
// full), which are applied after SP responds and never forwarded.
func validateQueryOnly(payload map[string]json.RawMessage) (map[string]string, *Result) {
	unknown := extraKeys(payload, map[string]bool{
		"query": true, "limit": true, "offset": true, "full": true,
	})
	if len(unknown) > 0 {
		r := Failure(ErrInvalidInput, fmt.Sprintf("Unknown filter fields: %s", strings.Join(unknown, ", ")))
		return nil, &r
	}
	params := make(map[string]string)
	if raw, ok := payload["query"]; ok {
		if string(raw) == "null" {
			r := Failure(ErrInvalidInput, "Filter 'query' must be str")
			return nil, &r
		}
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			r := Failure(ErrInvalidInput, "Filter 'query' must be str")
			return nil, &r
		}
		params["query"] = s
	}
	return params, nil
}

// --- Helpers ---

func sortedKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func extraKeys(payload map[string]json.RawMessage, allowed map[string]bool) []string {
	var extra []string
	for k := range payload {
		if !allowed[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	return extra
}

// validateListOptions extracts the bridge-side shaping options from a list
// payload. They are removed from the caller's view of "filters" because SP
// never sees them — passing them upstream would be ignored, and leaving them in
// the query string would be a lie about what was asked of the app.
func validateListOptions(payload map[string]json.RawMessage) (listOptions, *Result) {
	var opts listOptions

	for _, name := range []string{"limit", "offset"} {
		raw, ok := payload[name]
		if !ok {
			continue
		}
		val, present, err := getNonNegativeInt(payload, name)
		if err != nil {
			r := Failure(ErrInvalidInput, fmt.Sprintf("Filter '%s' must be a non-negative integer, got %s", name, describeRawType(raw)))
			return opts, &r
		}
		if !present {
			continue
		}
		if val > maxListLimit {
			r := Failure(ErrInvalidInput, fmt.Sprintf("Filter '%s' must not exceed %d", name, maxListLimit))
			return opts, &r
		}
		if name == "limit" {
			// Zero used to mean "no limit", which is the worst reading: a caller
			// paging a list and computing limit = remaining, or a host filling
			// an integer field with its zero default, asked for nothing and got
			// the entire store — the blow-up this option exists to prevent.
			if val == 0 {
				r := Failure(ErrInvalidInput, "Filter 'limit' must be at least 1; omit it entirely to return everything")
				return opts, &r
			}
			opts.limit = int(val)
		} else {
			opts.offset = int(val)
		}
	}

	if raw, ok := payload["full"]; ok {
		switch string(raw) {
		case "true":
			opts.full = true
		case "false":
		default:
			r := Failure(ErrInvalidInput, "Filter 'full' must be bool, got "+describeRawType(raw))
			return opts, &r
		}
	}
	return opts, nil
}

// MaxListLimit bounds limit and offset so a typo cannot ask for an allocation
// far larger than any real store. Exported so the CLI and the MCP schemas
// advertise the same ceiling the bridge enforces.
const MaxListLimit = 100000

// maxListLimit bounds limit and offset so a typo cannot ask for an allocation
// far larger than any real store.
const maxListLimit = MaxListLimit
