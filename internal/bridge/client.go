package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultTimeout = 10 * time.Second

// Client communicates with the SP Local REST API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new SP REST client with the default per-request timeout.
func NewClient(baseURL string) *Client {
	return NewClientWithTimeout(baseURL, defaultTimeout)
}

// NewClientWithTimeout creates a client with a caller-chosen per-request
// timeout. http.Client.Timeout caps every request regardless of the context
// deadline, so a caller that needs longer than defaultTimeout — pulling the
// whole task store, say — must raise it here rather than only widening the
// context. A non-positive timeout falls back to the default.
func NewClientWithTimeout(baseURL string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// request executes an HTTP request and translates the response.
func (c *Client) request(ctx context.Context, method, path string, body any, params map[string]string) Result {
	reqURL := c.baseURL + path
	if len(params) > 0 {
		q := url.Values{}
		for k, v := range params {
			q.Set(k, v)
		}
		reqURL += "?" + q.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return Failure(ErrInternalError, fmt.Sprintf("failed to marshal request body: %v", err))
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return Failure(ErrInternalError, fmt.Sprintf("failed to create request: %v", err))
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return c.translateError(err)
	}
	defer resp.Body.Close()

	return c.translateResponse(resp)
}

func (c *Client) translateError(err error) Result {
	// Check for timeout
	if isTimeout(err) {
		return Failure(ErrTimeout, "Request to Super Productivity timed out.")
	}
	// Check for connection refused / DNS errors
	if isConnectionError(err) {
		return Failure(ErrSPUnavailable, "Cannot connect to Super Productivity Local REST API.")
	}
	return Failure(ErrSPUnavailable, fmt.Sprintf("HTTP error: %v", err))
}

func (c *Client) translateResponse(resp *http.Response) Result {
	// A 404 is not read specially. SP distinguishes a missing task from a
	// missing route in the response body — TASK_NOT_FOUND versus NOT_FOUND —
	// and short-circuiting before parsing threw that away, reporting every 404
	// as a missing task. Callers that branch on the code were then unable to
	// tell "this task is gone" from "this route does not exist", and a mistyped
	// or removed route sent whoever was debugging it looking for a task.
	//
	// The envelope path below handles both: translateEnvelope uses the code SP
	// supplied. A 404 whose body is empty or unparseable falls through to
	// SP_ERROR, which is the honest answer when we cannot tell what was missing.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return Success(nil)
		}
		return Failure(ErrSPError, fmt.Sprintf("SP returned status %d.", resp.StatusCode),
			map[string]any{"status_code": resp.StatusCode})
	}

	if len(respBody) == 0 {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return Success(nil)
		}
		return Failure(ErrSPError, fmt.Sprintf("SP returned non-JSON response with status %d.", resp.StatusCode),
			map[string]any{"status_code": resp.StatusCode})
	}

	// Try to parse as JSON
	var parsed any
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return Success(nil)
		}
		return Failure(ErrSPError, fmt.Sprintf("SP returned non-JSON response with status %d.", resp.StatusCode),
			map[string]any{"status_code": resp.StatusCode})
	}

	// Check for SP envelope: { "ok": bool, ... }
	if obj, ok := parsed.(map[string]any); ok {
		if _, hasOK := obj["ok"]; hasOK {
			return c.translateEnvelope(obj, resp.StatusCode)
		}
	}

	// Non-envelope success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return Success(parsed)
	}

	return Failure(ErrSPError, fmt.Sprintf("SP returned status %d.", resp.StatusCode),
		map[string]any{"status_code": resp.StatusCode, "body": parsed})
}

func (c *Client) translateEnvelope(obj map[string]any, statusCode int) Result {
	okVal, _ := obj["ok"].(bool)
	if okVal && (statusCode < 200 || statusCode >= 300) {
		// An HTTP error status carrying ok:true is contradictory, and trusting
		// the body would report a failed request as a success. That matters
		// beyond tidiness: task.archive reads a task to decide whether it exists
		// before writing, and treating a 404 as "it exists" would send the very
		// call that crashed SP's renderer. Believe the status.
		return Failure(ErrSPError,
			fmt.Sprintf("SP returned status %d with a success envelope; treating as an error.", statusCode),
			map[string]any{"status_code": statusCode, "body": obj})
	}
	if okVal {
		data := obj["data"]
		if data == nil {
			// data might just not be present
			if _, hasData := obj["data"]; !hasData {
				return Success(obj)
			}
		}
		return Success(data)
	}

	// Error envelope
	errorVal := obj["error"]
	switch e := errorVal.(type) {
	case map[string]any:
		code, _ := e["code"].(string)
		if code == "" {
			code = ErrSPError
		}
		message, _ := e["message"].(string)
		if message == "" {
			message = "Unknown SP error."
		}
		details := map[string]any{"status_code": statusCode}
		if d := e["details"]; d != nil {
			details["sp_details"] = d
		}
		return Failure(code, message, details)
	case string:
		return Failure(ErrSPError, e, map[string]any{"status_code": statusCode})
	default:
		return Failure(ErrSPError, "Unknown SP error.",
			map[string]any{"status_code": statusCode, "body": obj})
	}
}

// --- Public API methods ---

func (c *Client) Health(ctx context.Context) Result {
	return c.request(ctx, http.MethodGet, "/health", nil, nil)
}

func (c *Client) Status(ctx context.Context) Result {
	return c.request(ctx, http.MethodGet, "/status", nil, nil)
}

func (c *Client) ListTasks(ctx context.Context, params map[string]string) Result {
	return c.request(ctx, http.MethodGet, "/tasks", nil, params)
}

func (c *Client) GetTask(ctx context.Context, id string) Result {
	return c.request(ctx, http.MethodGet, "/tasks/"+id, nil, nil)
}

func (c *Client) CreateTask(ctx context.Context, body map[string]any) Result {
	return c.request(ctx, http.MethodPost, "/tasks", body, nil)
}

func (c *Client) UpdateTask(ctx context.Context, id string, body map[string]any) Result {
	return c.request(ctx, http.MethodPatch, "/tasks/"+id, body, nil)
}

func (c *Client) StartTask(ctx context.Context, id string) Result {
	return c.request(ctx, http.MethodPost, "/tasks/"+id+"/start", nil, nil)
}

func (c *Client) StopCurrentTask(ctx context.Context) Result {
	return c.request(ctx, http.MethodPost, "/task-control/stop", nil, nil)
}

func (c *Client) GetCurrentTask(ctx context.Context) Result {
	return c.request(ctx, http.MethodGet, "/task-control/current", nil, nil)
}

func (c *Client) SetCurrentTask(ctx context.Context, taskID *string) Result {
	body := map[string]any{"taskId": nil}
	if taskID != nil {
		body["taskId"] = *taskID
	}
	return c.request(ctx, http.MethodPost, "/task-control/current", body, nil)
}

func (c *Client) ArchiveTask(ctx context.Context, id string) Result {
	return c.request(ctx, http.MethodPost, "/tasks/"+id+"/archive", nil, nil)
}

func (c *Client) RestoreTask(ctx context.Context, id string) Result {
	return c.request(ctx, http.MethodPost, "/tasks/"+id+"/restore", nil, nil)
}

func (c *Client) ListProjects(ctx context.Context, params map[string]string) Result {
	return c.request(ctx, http.MethodGet, "/projects", nil, params)
}

func (c *Client) ListTags(ctx context.Context, params map[string]string) Result {
	return c.request(ctx, http.MethodGet, "/tags", nil, params)
}

// --- Error classification ---

func isTimeout(err error) bool {
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	return false
}

func isConnectionError(err error) bool {
	if opErr, ok := err.(*net.OpError); ok {
		if _, ok := opErr.Err.(*net.DNSError); ok {
			return true
		}
		// Connection refused
		return true
	}
	// URL errors wrapping net errors
	if urlErr, ok := err.(*url.Error); ok {
		return isConnectionError(urlErr.Err)
	}
	// Check for "connection refused" in the error string as fallback
	if strings.Contains(err.Error(), "connection refused") {
		return true
	}
	if strings.Contains(err.Error(), "no such host") {
		return true
	}
	return false
}
