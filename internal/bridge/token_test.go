package bridge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// validToken is the shape SP generates: 32 characters from [A-Za-z0-9].
const validToken = "Ab3xY9zQ1mN5pR7tK2wV4jL6hG8dS0cF"

// isolateHome points token lookup at a scratch directory. XDG_CONFIG_HOME has
// to be cleared too: Electron prefers it on Linux, so leaving the developer's
// own value set would make the test read their real token file.
func isolateHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("APPDATA", filepath.Join(dir, "AppData", "Roaming"))
	t.Setenv(TokenEnvVar, "")
	return dir
}

func writeTokenFile(t *testing.T, contents string) {
	t.Helper()
	path := TokenPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating the token directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("writing the token file: %v", err)
	}
}

func TestResolveToken_NoneConfigured(t *testing.T) {
	isolateHome(t)
	token, source := ResolveToken()
	if token != "" {
		t.Errorf("token = %q, want empty", token)
	}
	// Not an error: Super Productivity before 18.19.0 has no token at all.
	if source != TokenSourceNone {
		t.Errorf("source = %q, want %q", source, TokenSourceNone)
	}
}

func TestResolveToken_FromEnv(t *testing.T) {
	isolateHome(t)
	t.Setenv(TokenEnvVar, validToken)
	token, source := ResolveToken()
	if token != validToken {
		t.Errorf("token = %q, want %q", token, validToken)
	}
	if source != TokenSourceEnv {
		t.Errorf("source = %q, want %q", source, TokenSourceEnv)
	}
}

func TestResolveToken_FromFile(t *testing.T) {
	isolateHome(t)
	writeTokenFile(t, validToken)
	token, source := ResolveToken()
	if token != validToken {
		t.Errorf("token = %q, want %q", token, validToken)
	}
	if source != TokenSourceFile {
		t.Errorf("source = %q, want %q", source, TokenSourceFile)
	}
}

func TestResolveToken_EnvBeatsFile(t *testing.T) {
	// The override exists so a stale file cannot win. If the file won, a user
	// told to "set SP_API_TOKEN" would see no change and have nothing to try.
	isolateHome(t)
	writeTokenFile(t, "Zz0000000000000000000000000000000"[:32])
	t.Setenv(TokenEnvVar, validToken)
	token, source := ResolveToken()
	if token != validToken {
		t.Errorf("token = %q, want the environment's %q", token, validToken)
	}
	if source != TokenSourceEnv {
		t.Errorf("source = %q, want %q", source, TokenSourceEnv)
	}
}

func TestResolveToken_TrimsWhitespace(t *testing.T) {
	// The file SP writes ends with a newline, and a token pasted into a shell
	// profile often carries spaces. Both must produce the bare credential:
	// "Bearer <token>\n" is a malformed header value.
	for _, tc := range []struct{ name, raw string }{
		{"trailing newline", validToken + "\n"},
		{"trailing CRLF", validToken + "\r\n"},
		{"surrounding spaces", "  " + validToken + "  "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateHome(t)
			writeTokenFile(t, tc.raw)
			token, source := ResolveToken()
			if token != validToken {
				t.Errorf("token = %q, want %q", token, validToken)
			}
			if source != TokenSourceFile {
				t.Errorf("source = %q, want %q", source, TokenSourceFile)
			}
		})
	}
}

func TestResolveToken_RejectsFileThatIsNotAToken(t *testing.T) {
	// Sending the contents of some other file produces "invalid token", which
	// points at the wrong problem. Treat it as no token instead.
	for _, tc := range []struct{ name, contents string }{
		{"empty", ""},
		{"whitespace only", "   \n"},
		{"too short", "abc123"},
		{"too long", validToken + "extra"},
		{"punctuation", "not-a-valid-token-aaaaaaaaaaaaaaa"},
		{"json", `{"token":"` + validToken + `"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateHome(t)
			writeTokenFile(t, tc.contents)
			token, source := ResolveToken()
			if token != "" {
				t.Errorf("token = %q, want empty for contents %q", token, tc.contents)
			}
			if source != TokenSourceNone {
				t.Errorf("source = %q, want %q", source, TokenSourceNone)
			}
		})
	}
}

func TestResolveToken_IgnoresAnOversizedFile(t *testing.T) {
	// The bound is a memory guard, and a guard with no observable effect is
	// untestable and therefore unmaintained. This is the case that makes it
	// observable: padding is trimmed, so without the size check this file
	// would yield a perfectly valid token after reading all of it into memory.
	isolateHome(t)
	padding := strings.Repeat(" ", maxTokenFileBytes*2)
	writeTokenFile(t, padding+validToken+padding)

	token, source := ResolveToken()
	if token != "" {
		t.Errorf("token = %q, want empty: the file is past the size bound", token)
	}
	if source != TokenSourceNone {
		t.Errorf("source = %q, want %q", source, TokenSourceNone)
	}
}

func TestResolveToken_RejectsUnusableEnvValue(t *testing.T) {
	// net/http refuses to send an invalid header value, failing the request
	// before it leaves — including GET /health, which needs no token at all.
	// A bad token must not make a reachable SP look unreachable.
	for _, tc := range []struct{ name, value string }{
		{"newline", "bad\ntoken"},
		{"carriage return", "bad\rtoken"},
		{"non-ascii", "tökenwithaccents1234567890abcdef"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateHome(t)
			writeTokenFile(t, validToken)
			t.Setenv(TokenEnvVar, tc.value)

			token, source := ResolveToken()
			if token != "" {
				t.Errorf("token = %q, want empty", token)
			}
			// Not TokenSourceNone, and not the file's token: the user set the
			// variable deliberately, so silently authenticating as someone
			// else would hide the mistake they need to see.
			if source != TokenSourceEnvInvalid {
				t.Errorf("source = %q, want %q", source, TokenSourceEnvInvalid)
			}
		})
	}
}

func TestResolveToken_BlankEnvFallsThroughToFile(t *testing.T) {
	// An exported-but-empty variable is what a shell profile leaves behind when
	// the value is removed. It must not shadow a working file.
	isolateHome(t)
	writeTokenFile(t, validToken)
	t.Setenv(TokenEnvVar, "   ")
	token, source := ResolveToken()
	if token != validToken {
		t.Errorf("token = %q, want the file's %q", token, validToken)
	}
	if source != TokenSourceFile {
		t.Errorf("source = %q, want %q", source, TokenSourceFile)
	}
}

func TestResolveToken_FindsFlatpakInstall(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Flatpak is Linux-only")
	}
	// Flatpak redirects XDG_CONFIG_HOME inside its sandbox. The bridge runs
	// outside, sees the host's value, and so finds nothing at the standard
	// path — which is every Flathub user, and CI only runs Linux.
	home := isolateHome(t)
	flatpak := filepath.Join(home, ".var", "app",
		"com.super_productivity.SuperProductivity", "config", "superProductivity")
	if err := os.MkdirAll(flatpak, 0o755); err != nil {
		t.Fatalf("creating the flatpak directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(flatpak, "local-rest-api-token"), []byte(validToken), 0o600); err != nil {
		t.Fatalf("writing the token file: %v", err)
	}

	token, source := ResolveToken()
	if token != validToken {
		t.Errorf("token = %q, want the Flatpak install's %q", token, validToken)
	}
	if source != TokenSourceFile {
		t.Errorf("source = %q, want %q", source, TokenSourceFile)
	}
}

func TestResolveToken_StandardPathBeatsFlatpak(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Flatpak is Linux-only")
	}
	// A machine with both installs has two stores and two tokens. The
	// non-sandboxed one is what TokenPath() reports and what diagnostics name,
	// so it has to be the one actually used.
	home := isolateHome(t)
	writeTokenFile(t, validToken)

	flatpak := filepath.Join(home, ".var", "app",
		"com.super_productivity.SuperProductivity", "config", "superProductivity")
	if err := os.MkdirAll(flatpak, 0o755); err != nil {
		t.Fatalf("creating the flatpak directory: %v", err)
	}
	other := "Zz9yX8wV7uT6sR5qP4oN3mL2kJ1iH0gF"
	if err := os.WriteFile(filepath.Join(flatpak, "local-rest-api-token"), []byte(other), 0o600); err != nil {
		t.Fatalf("writing the token file: %v", err)
	}

	token, _ := ResolveToken()
	if token != validToken {
		t.Errorf("token = %q, want the standard path's %q, not the Flatpak one", token, validToken)
	}
}

func TestUsableAsHeaderValue(t *testing.T) {
	// Tested directly because the environment cannot carry every rejected
	// byte: setenv refuses a value containing NUL, so that case is
	// unreachable through ResolveToken and would be a test of nothing.
	for _, tc := range []struct {
		name  string
		value string
		want  bool
	}{
		{"a real token", validToken, true},
		{"newline", "abc\ndef", false},
		{"carriage return", "abc\rdef", false},
		{"null byte", "abc\x00def", false},
		{"DEL", "abc\x7fdef", false},
		{"non-ascii", "abcödef", false},
		{"tab is allowed", "abc\tdef", true},
		{"empty", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := usableAsHeaderValue(tc.value); got != tc.want {
				t.Errorf("usableAsHeaderValue(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestTokenPath_HonoursXDGOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG_CONFIG_HOME is a Linux path convention")
	}
	dir := t.TempDir()
	t.Setenv("HOME", filepath.Join(dir, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	got := TokenPath()
	want := filepath.Join(dir, "xdg", "superProductivity", "local-rest-api-token")
	if got != want {
		t.Errorf("TokenPath() = %q, want %q", got, want)
	}
}

// --- the header the client actually sends ---

// captureAuth runs one request against a stub and reports the Authorization
// header it received, plus whether the header was present at all.
func captureAuth(t *testing.T, token string) (string, bool) {
	t.Helper()
	var got string
	var present bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, present = r.Header.Get("Authorization"), r.Header.Values("Authorization") != nil
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"data":{}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL).WithToken(token)
	if res := client.Status(context.Background()); !res.OK {
		t.Fatalf("status call failed: %+v", res.Error)
	}
	return got, present
}

func TestClient_SendsBearerToken(t *testing.T) {
	got, present := captureAuth(t, validToken)
	if !present {
		t.Fatal("no Authorization header was sent")
	}
	if want := "Bearer " + validToken; got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
}

func TestClient_SendsNoHeaderWithoutToken(t *testing.T) {
	// Not an empty header: "Authorization: Bearer " is a malformed credential,
	// and SP reports it as an invalid token rather than a missing one — which
	// sends the user looking for a stale token they never had.
	got, present := captureAuth(t, "")
	if present || got != "" {
		t.Errorf("Authorization = %q (present=%v), want no header at all", got, present)
	}
}

func TestClient_DoesNotFollowRedirectsWithTheToken(t *testing.T) {
	// SP's API issues no redirects, so following one can only carry the
	// credential somewhere it was not meant to go. Go already strips the header
	// across hosts; a same-host redirect to another port is the gap this closes.
	var redirectTargetSawAuth bool
	var redirectTargetHit bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectTargetHit = true
		redirectTargetSawAuth = r.Header.Get("Authorization") != ""
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"data":{"leaked":true}}`))
	}))
	defer target.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusFound)
	}))
	defer origin.Close()

	client := NewClient(origin.URL).WithToken(validToken)
	res := client.Status(context.Background())

	if redirectTargetHit {
		t.Errorf("the redirect was followed; target saw Authorization: %v", redirectTargetSawAuth)
	}
	// A 302 is not a success envelope, so this must not read as an OK result.
	if res.OK {
		t.Errorf("a redirect was reported as success: %+v", res.Data)
	}
}

func TestClient_SendsTokenOnWritesToo(t *testing.T) {
	// Every route except GET /health is authenticated, so a token attached only
	// to reads would leave every mutation failing.
	var methods []string
	var authed []bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		authed = append(authed, r.Header.Get("Authorization") == "Bearer "+validToken)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"data":{"id":"t1"}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL).WithToken(validToken)
	ctx := context.Background()
	client.ListTasks(ctx, nil)
	client.CreateTask(ctx, map[string]any{"title": "x"})
	client.UpdateTask(ctx, "t1", map[string]any{"title": "y"})

	if len(methods) != 3 {
		t.Fatalf("expected 3 requests, got %d (%v)", len(methods), methods)
	}
	for i, ok := range authed {
		if !ok {
			t.Errorf("request %d (%s) carried no valid Authorization header", i, methods[i])
		}
	}
}
