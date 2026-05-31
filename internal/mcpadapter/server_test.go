package mcpadapter

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/bridge"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) *Server {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	client := bridge.NewClient(ts.URL)
	service := bridge.NewService(client)
	return NewServer(service)
}

func sendRPC(t *testing.T, server *Server, reqJSON string) map[string]any {
	t.Helper()
	// Override stdin/stdout
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()

	// Create pipe for stdin
	stdinR, stdinW, _ := os.Pipe()
	os.Stdin = stdinR

	// Create pipe for stdout
	stdoutR, stdoutW, _ := os.Pipe()
	os.Stdout = stdoutW

	// Write request and close stdin
	go func() {
		stdinW.Write([]byte(reqJSON + "\n"))
		stdinW.Close()
	}()

	// Run server (reads until EOF)
	go func() {
		server.Run()
		stdoutW.Close()
	}()

	// Read response
	data, _ := io.ReadAll(stdoutR)
	stdinR.Close()

	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &resp); err != nil {
		t.Fatalf("failed to parse response: %v\nraw: %s", err, data)
	}
	return resp
}

func TestMCP_Initialize(t *testing.T) {
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	resp := sendRPC(t, server, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)

	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	serverInfo := result["serverInfo"].(map[string]any)
	if serverInfo["name"] != "sp-local-bridge" {
		t.Fatalf("expected sp-local-bridge, got %v", serverInfo["name"])
	}
}

func TestMCP_ToolsList(t *testing.T) {
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	resp := sendRPC(t, server, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)

	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	tools := result["tools"].([]any)
	if len(tools) != 16 {
		t.Fatalf("expected 16 tools, got %d", len(tools))
	}

	// Check tool names
	expected := map[string]bool{
		"health": true, "get_status": true, "list_tasks": true,
		"get_task": true, "create_task": true, "update_task": true,
		"complete_task": true, "uncomplete_task": true, "start_task": true,
		"stop_current_task": true, "get_current_task": true, "set_current_task": true,
		"archive_task": true, "restore_task": true, "list_projects": true,
		"list_tags": true,
	}
	for _, tool := range tools {
		name := tool.(map[string]any)["name"].(string)
		if !expected[name] {
			t.Fatalf("unexpected tool: %s", name)
		}
	}
}

func TestMCP_ToolsCall_Health(t *testing.T) {
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/health" {
			w.Write([]byte(`{"ok":true,"data":{"status":"up"}}`))
		} else {
			w.Write([]byte(`{"ok":true,"data":{"currentTask":null}}`))
		}
	})
	resp := sendRPC(t, server, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"health","arguments":{}}}`)

	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	if result["isError"] == true {
		t.Fatal("expected success")
	}
}

func TestMCP_ToolsCall_UnknownTool(t *testing.T) {
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	resp := sendRPC(t, server, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"nonexistent","arguments":{}}}`)

	if resp["error"] == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestMCP_Ping(t *testing.T) {
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	resp := sendRPC(t, server, `{"jsonrpc":"2.0","id":5,"method":"ping","params":{}}`)

	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
}
