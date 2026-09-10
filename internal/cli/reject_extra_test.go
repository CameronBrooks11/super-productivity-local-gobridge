package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

// These commands used to read the one argument they wanted and ignore the rest,
// so a mistyped flag ran the command as though it had not been typed:
// `tasks archive <id> --formatt table` archived the task and exited 0. The
// property that matters is not the exit code on its own — it is that the
// request never goes out, because for the write commands the request is the
// damage.

// countingSP stands in for Super Productivity and records whether it was asked
// for anything at all.
func countingSP(t *testing.T, calls *int64) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(calls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"data":{"id":"t1","title":"stub","isDone":false}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("SP_BASE_URL", srv.URL)
}

// silence keeps the command's own output out of the test log.
func silence(t *testing.T) {
	t.Helper()
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("opening %s: %v", os.DevNull, err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = devNull, devNull
	t.Cleanup(func() {
		os.Stdout, os.Stderr = oldOut, oldErr
		_ = devNull.Close()
	})
}

// everyGuardedCommand is every command shape that consumes a fixed number of
// arguments. Listed explicitly rather than derived, so adding a subcommand
// without a guard shows up here as a missing row rather than as silence.
var everyGuardedCommand = [][]string{
	{"health"},
	{"status"},
	{"tasks", "get", "t1"},
	{"tasks", "complete", "t1"},
	{"tasks", "uncomplete", "t1"},
	{"tasks", "start", "t1"},
	{"tasks", "archive", "t1"},
	{"tasks", "restore", "t1"},
	{"tasks", "set-current", "t1"},
	{"tasks", "current"},
	{"tasks", "stop-current"},
	{"tasks", "clear-current"},
}

func TestRejectsMistypedFlagWithoutMakingARequest(t *testing.T) {
	for _, base := range everyGuardedCommand {
		name := strings.Join(base, " ")
		t.Run(name, func(t *testing.T) {
			var calls int64
			countingSP(t, &calls)
			silence(t)

			code := Run(append(append([]string{}, base...), "--formatt", "table"))

			if code != 2 {
				t.Errorf("exit = %d, want 2", code)
			}
			// The write commands are why this matters: an exit code of 2 after
			// the task was already archived would still be a task archived.
			if got := atomic.LoadInt64(&calls); got != 0 {
				t.Errorf("%d request(s) reached Super Productivity; want 0", got)
			}
		})
	}
}

func TestRejectsExtraPositionalWithoutMakingARequest(t *testing.T) {
	for _, base := range everyGuardedCommand {
		name := strings.Join(base, " ")
		t.Run(name, func(t *testing.T) {
			var calls int64
			countingSP(t, &calls)
			silence(t)

			code := Run(append(append([]string{}, base...), "surprise"))

			if code != 2 {
				t.Errorf("exit = %d, want 2", code)
			}
			if got := atomic.LoadInt64(&calls); got != 0 {
				t.Errorf("%d request(s) reached Super Productivity; want 0", got)
			}
		})
	}
}

func TestValidFormsStillReachSuperProductivity(t *testing.T) {
	// The other half. A guard that refused everything would pass the two tests
	// above and break the tool.
	for _, base := range everyGuardedCommand {
		for _, suffix := range [][]string{nil, {"--format", "table"}} {
			name := strings.Join(append(append([]string{}, base...), suffix...), " ")
			t.Run(name, func(t *testing.T) {
				var calls int64
				countingSP(t, &calls)
				silence(t)

				code := Run(append(append([]string{}, base...), suffix...))

				if code != 0 {
					t.Errorf("exit = %d, want 0", code)
				}
				if got := atomic.LoadInt64(&calls); got == 0 {
					t.Error("no request was made; the command did not run")
				}
			})
		}
	}
}

func TestRejectExtraArgs_NamesFlagsAndPositionalsDifferently(t *testing.T) {
	// A user who mistyped a flag and one who passed a stray word have different
	// problems, and the message should say which it saw.
	if err := rejectExtraArgs("tasks get", []string{"get", "t1", "--formatt"}, 2); err == nil {
		t.Fatal("a leftover flag was accepted")
	} else if !strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("message does not name it as a flag: %q", err)
	}
	if err := rejectExtraArgs("tasks get", []string{"get", "t1", "extra"}, 2); err == nil {
		t.Fatal("a leftover positional was accepted")
	} else if !strings.Contains(err.Error(), "unexpected argument") {
		t.Errorf("message does not name it as an argument: %q", err)
	}
	if err := rejectExtraArgs("tasks get", []string{"get", "t1"}, 2); err != nil {
		t.Errorf("a correct invocation was rejected: %v", err)
	}
}
