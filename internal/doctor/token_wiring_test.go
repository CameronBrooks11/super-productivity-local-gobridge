package doctor

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// These tests exist because the token wiring at doctor's three client
// constructions was, when added, held in place by nothing: deleting
// `.WithToken(...)` from any of them still compiled and still passed the whole
// suite. The unit tests covered the client and the resolver; nothing covered
// whether doctor handed the token to either.

const wiringToken = "Ab3xY9zQ1mN5pR7tK2wV4jL6hG8dS0cF"

// spRequiringToken mimics Super Productivity 18.19.0+: GET /health needs no
// credential, every other route demands the bearer token.
func spRequiringToken(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && r.URL.Path == "/health" {
			_, _ = w.Write([]byte(`{"ok":true,"data":{"server":"up","rendererReady":true}}`))
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+wiringToken {
			w.Header().Set("WWW-Authenticate", "Bearer")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"UNAUTHORIZED","message":"Authorization token required."}}`))
			return
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/status"):
			_, _ = w.Write([]byte(`{"ok":true,"data":{"currentTask":null,"currentTaskId":null,"taskCount":1}}`))
		case strings.HasPrefix(r.URL.Path, "/tasks"):
			// The archived pool has to be empty, or the same task appears in
			// both and --deep correctly reports it as a partially applied
			// archive — a real finding, but not the one under test here.
			if r.URL.Query().Get("source") == "archived" {
				_, _ = w.Write([]byte(`{"ok":true,"data":[]}`))
				return
			}
			// Shaped so the --deep integrity check can actually judge it: it
			// needs subTaskIds to resolve parentage, and refuses to report on
			// a task missing them.
			_, _ = w.Write([]byte(`{"ok":true,"data":[{"id":"t1","title":"One","isDone":false,"projectId":"p1","tagIds":[],"subTaskIds":[],"parentId":null}]}`))
		case strings.HasPrefix(r.URL.Path, "/projects"):
			_, _ = w.Write([]byte(`{"ok":true,"data":[{"id":"p1","title":"Project","taskIds":["t1"],"backlogTaskIds":[]}]}`))
		case strings.HasPrefix(r.URL.Path, "/tags"):
			_, _ = w.Write([]byte(`{"ok":true,"data":[]}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true,"data":[]}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// runDoctor executes Run with stdout captured, and returns the output and the
// exit code.
func runDoctor(t *testing.T, args ...string) (string, int) {
	t.Helper()

	// Without this, Run() spawns the test binary as an MCP server and waits out
	// a ten-second timeout for an answer it will never get — four times over,
	// on three platforms. On macOS the spawned process also creates ~/Library
	// under the scratch HOME, which then fails TempDir cleanup. Neither the MCP
	// self-check nor the alias check is what these tests are about.
	origExe := osExecutable
	osExecutable = func() (string, error) { return "", nil }
	t.Cleanup(func() { osExecutable = origExe })
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating a pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	code := Run(args)
	_ = w.Close()
	os.Stdout = orig
	return <-done, code
}

// isolate points token and host-config lookup at a scratch directory, so the
// developer's own token file and MCP configs cannot influence the result.
func isolate(t *testing.T, srv *httptest.Server) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir+"/.config")
	t.Setenv("APPDATA", dir+"/AppData/Roaming")
	t.Setenv("SP_BASE_URL", srv.URL)
}

func TestDoctorSendsTokenOnTheStandardChecks(t *testing.T) {
	srv := spRequiringToken(t)
	isolate(t, srv)
	t.Setenv("SP_API_TOKEN", wiringToken)

	out, _ := runDoctor(t)

	// Health is unauthenticated, so it proves nothing. Status and the task list
	// are the two that can only pass if doctor sent the token.
	for _, want := range []string{"Status check... OK", "Task list... OK"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q — doctor did not send the token on its standard client:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "Access token... set (SP_API_TOKEN)") {
		t.Errorf("token source was not reported:\n%s", out)
	}
}

func TestDoctorSendsTokenOnTheDeepClient(t *testing.T) {
	srv := spRequiringToken(t)
	isolate(t, srv)
	t.Setenv("SP_API_TOKEN", wiringToken)

	out, _ := runDoctor(t, "--deep")

	// --deep builds its own client with a longer timeout. It was a separate
	// construction and so a separate chance to miss the token.
	if strings.Contains(out, "Store integrity... SKIPPED") {
		t.Fatalf("the deep check never ran, so this asserts nothing:\n%s", out)
	}
	if !strings.Contains(out, "Store integrity... OK") {
		t.Errorf("deep client did not authenticate:\n%s", out)
	}
}

func TestDoctorJSONSendsToken(t *testing.T) {
	srv := spRequiringToken(t)
	isolate(t, srv)
	t.Setenv("SP_API_TOKEN", wiringToken)

	out, code := runDoctor(t, "--json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0; --json client did not authenticate. Output:\n%s", code, out)
	}
	var report map[string]any
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("output was not JSON (%v):\n%s", err, out)
	}
}

func TestDoctorReportsAnUnusableEnvToken(t *testing.T) {
	srv := spRequiringToken(t)
	isolate(t, srv)
	// A newline cannot go in a header value. Before this was rejected, net/http
	// failed the request before it left, and every check — including the
	// unauthenticated health probe — reported a transport error pointing at
	// "is Super Productivity running?".
	t.Setenv("SP_API_TOKEN", "bad\ntoken")

	out, _ := runDoctor(t)

	if !strings.Contains(out, "Health check... OK") {
		t.Errorf("an unusable token broke the unauthenticated health route:\n%s", out)
	}
	if !strings.Contains(out, "is set but is not a usable token") {
		t.Errorf("doctor did not say the token was unusable:\n%s", out)
	}
}
