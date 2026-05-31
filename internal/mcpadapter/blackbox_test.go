package mcpadapter

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"testing"
)

// TestMCPBlackBox_Initialize tests the compiled binary's MCP server via stdio.
func TestMCPBlackBox_Initialize(t *testing.T) {
	binary := buildBinary(t)
	resp := mcpRPC(t, binary, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`)
	assertNoError(t, resp)
	result := resp["result"].(map[string]any)
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("expected protocolVersion 2024-11-05, got %v", result["protocolVersion"])
	}
	serverInfo := result["serverInfo"].(map[string]any)
	if serverInfo["name"] != "sp-local-bridge" {
		t.Errorf("expected name sp-local-bridge, got %v", serverInfo["name"])
	}
}

func TestMCPBlackBox_ToolsList(t *testing.T) {
	binary := buildBinary(t)
	resp := mcpRPC(t, binary, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	assertNoError(t, resp)
	result := resp["result"].(map[string]any)
	tools := result["tools"].([]any)
	if len(tools) != 16 {
		t.Errorf("expected 16 tools, got %d", len(tools))
	}
}

func TestMCPBlackBox_Ping(t *testing.T) {
	binary := buildBinary(t)
	resp := mcpRPC(t, binary, `{"jsonrpc":"2.0","id":3,"method":"ping","params":{}}`)
	assertNoError(t, resp)
}

func TestMCPBlackBox_UnknownMethod(t *testing.T) {
	binary := buildBinary(t)
	resp := mcpRPC(t, binary, `{"jsonrpc":"2.0","id":4,"method":"nonexistent","params":{}}`)
	if resp["error"] == nil {
		t.Error("expected error for unknown method")
	}
}

func TestMCPBlackBox_ParseError(t *testing.T) {
	binary := buildBinary(t)
	resp := mcpRPC(t, binary, `{not valid json}`)
	if resp["error"] == nil {
		t.Error("expected parse error")
	}
	errObj := resp["error"].(map[string]any)
	if errObj["code"].(float64) != -32700 {
		t.Errorf("expected code -32700, got %v", errObj["code"])
	}
}

func TestMCPBlackBox_ToolsCallUnknown(t *testing.T) {
	binary := buildBinary(t)
	resp := mcpRPC(t, binary, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"nonexistent_tool","arguments":{}}}`)
	if resp["error"] == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestMCPBlackBox_ToolsCallHealthResult(t *testing.T) {
	// Tests that tools/call returns a proper MCP tool result (not a JSON-RPC error)
	binary := buildBinary(t)
	resp := mcpRPC(t, binary, `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"health","arguments":{}}}`)
	// Should NOT be a JSON-RPC error — should be a tool result
	if resp["error"] != nil {
		t.Fatalf("expected tool result, got JSON-RPC error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	// Must have content array
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatal("expected non-empty content array")
	}
	// First content item must be text type
	item := content[0].(map[string]any)
	if item["type"] != "text" {
		t.Errorf("expected type 'text', got %v", item["type"])
	}
	if item["text"] == nil || item["text"] == "" {
		t.Error("expected non-empty text in content")
	}

	// Check structuredContent behavior based on isError
	isError, _ := result["isError"].(bool)
	if isError {
		// Error results should NOT have structuredContent
		if result["structuredContent"] != nil {
			t.Error("error results should not have structuredContent")
		}
	} else {
		// Successful results SHOULD have structuredContent
		if result["structuredContent"] == nil {
			t.Error("successful results should have structuredContent")
		}
	}
}

func TestMCPBlackBox_MultiMessage(t *testing.T) {
	binary := buildBinary(t)
	// Send multiple messages in one session
	messages := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"ping","params":{}}`,
	}
	responses := mcpRPCMulti(t, binary, messages)
	// Should get 3 responses (notification doesn't get a response)
	if len(responses) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(responses))
	}
	// Check IDs
	ids := []float64{1, 2, 3}
	for i, resp := range responses {
		id, ok := resp["id"].(float64)
		if !ok {
			t.Errorf("response %d: missing id", i)
			continue
		}
		if id != ids[i] {
			t.Errorf("response %d: expected id %v, got %v", i, ids[i], id)
		}
	}
}

func TestMCPBlackBox_EOFCleanShutdown(t *testing.T) {
	binary := buildBinary(t)
	cmd := exec.Command(binary, "mcp")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Close stdin immediately (EOF)
	stdin.Close()
	// Should exit cleanly
	if err := cmd.Wait(); err != nil {
		t.Errorf("expected clean exit on EOF, got: %v", err)
	}
}

// --- Helpers ---

func buildBinary(t *testing.T) string {
	t.Helper()
	binary := t.TempDir() + "/sp-local-bridge-test"
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/sp-local-bridge")
	cmd.Dir = findModuleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return binary
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("go", "env", "GOMOD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	modFile := strings.TrimSpace(string(out))
	if modFile == "" {
		t.Fatal("not in a Go module")
	}
	// go.mod path → module root
	return modFile[:len(modFile)-len("/go.mod")]
}

func mcpRPC(t *testing.T, binary, request string) map[string]any {
	t.Helper()
	cmd := exec.Command(binary, "mcp")
	cmd.Stdin = strings.NewReader(request + "\n")
	out, err := cmd.Output()
	if err != nil {
		// Check if there's stderr
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("binary failed: %v\nstderr: %s", err, exitErr.Stderr)
		}
		t.Fatalf("binary failed: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to parse response: %v\nraw: %s", err, out)
	}
	return resp
}

func mcpRPCMulti(t *testing.T, binary string, requests []string) []map[string]any {
	t.Helper()
	input := strings.Join(requests, "\n") + "\n"
	cmd := exec.Command(binary, "mcp")
	cmd.Stdin = strings.NewReader(input)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	var responses []map[string]any
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var resp map[string]any
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("failed to parse response line: %v\nraw: %s", err, line)
		}
		responses = append(responses, resp)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("process error: %v", err)
	}
	return responses
}

func assertNoError(t *testing.T, resp map[string]any) {
	t.Helper()
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
}

func init() {
	// Ensure the test uses the Go binary path
	_ = fmt.Sprintf
	_ = io.EOF
}
