package bridge

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// TokenEnvVar names the environment variable that supplies the access token.
const TokenEnvVar = "SP_API_TOKEN"

// tokenFileName is the file Super Productivity writes the token to, inside its
// Electron userData directory. Named in electron/local-rest-api.ts.
const tokenFileName = "local-rest-api-token"

// spTokenPattern is the shape SP generates: 32 characters from
// [A-Za-z0-9]. Used to reject a file that holds something else — a stale
// editor swap, a partial write — rather than sending it as a credential.
var spTokenPattern = regexp.MustCompile(`^[A-Za-z0-9]{32}$`)

// TokenSource says where a token came from, for diagnostics. It never carries
// the token itself: doctor prints this, and a secret must not reach output a
// user pastes into an issue.
type TokenSource string

const (
	// TokenSourceNone means no token was found. Correct for SP before
	// 18.19.0, which had no token at all.
	TokenSourceNone TokenSource = "none"
	// TokenSourceEnv means the token came from SP_API_TOKEN.
	TokenSourceEnv TokenSource = "environment"
	// TokenSourceFile means the token was read from SP's own token file.
	TokenSourceFile TokenSource = "file"
	// TokenSourceEnvInvalid means SP_API_TOKEN was set to something that
	// cannot be sent as a credential. Distinct from "none" because the user
	// plainly meant to supply a token, and silently ignoring it — or silently
	// using the file instead — hides the mistake they need to see.
	TokenSourceEnvInvalid TokenSource = "environment-invalid"
)

// maxTokenFileBytes bounds the token file read. A token is 32 bytes; anything
// approaching this is not one, and reading it whole to then reject it wastes
// memory proportional to whatever is on disk.
const maxTokenFileBytes = 4096

// usableAsHeaderValue reports whether a token can be sent in an HTTP header.
//
// This matters because net/http rejects an invalid header value before the
// request leaves, which turns every call — including GET /health, the one route
// that needs no token at all — into a transport error reading "is SP running?".
// A bad token must not be able to make a reachable SP look unreachable.
func usableAsHeaderValue(token string) bool {
	for i := 0; i < len(token); i++ {
		if c := token[i]; c != '\t' && (c < 0x20 || c > 0x7E) {
			return false
		}
	}
	return true
}

// spUserDataDir returns Super Productivity's Electron userData directory.
func spUserDataDir() string {
	home := homeDirForToken()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "superProductivity")
	case "windows":
		return filepath.Join(appDataDirForToken(), "superProductivity")
	default:
		// Electron honours XDG_CONFIG_HOME on Linux before falling back to
		// ~/.config, so reading the variable is not optional here: a user who
		// sets it has their store somewhere this would otherwise miss.
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "superProductivity")
		}
		return filepath.Join(home, ".config", "superProductivity")
	}
}

// flatpakUserDataDir is where a Flathub install keeps its data. Flatpak
// redirects XDG_CONFIG_HOME inside the sandbox, but the bridge runs outside it
// and sees the host's value, so the sandboxed path has to be named explicitly.
// Documented in SP's wiki, 3.06-User-Data.
func flatpakUserDataDir() string {
	return filepath.Join(homeDirForToken(), ".var", "app",
		"com.super_productivity.SuperProductivity", "config", "superProductivity")
}

// tokenSearchPaths lists the files to try, in order.
//
// Snap is deliberately absent: its path derives from SNAP_USER_COMMON, which is
// only set inside the snap's own environment, so there is nothing to resolve
// from out here. SP_API_TOKEN covers it, and the docs say so.
func tokenSearchPaths() []string {
	paths := []string{filepath.Join(spUserDataDir(), tokenFileName)}
	if runtime.GOOS == "linux" {
		paths = append(paths, filepath.Join(flatpakUserDataDir(), tokenFileName))
	}
	return paths
}

// TokenPath is the file SP writes its access token to on this platform. It is
// the first location searched, and what diagnostics name when nothing is found.
func TokenPath() string {
	return tokenSearchPaths()[0]
}

func homeDirForToken() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}

func appDataDirForToken() string {
	if ad := os.Getenv("APPDATA"); ad != "" {
		return ad
	}
	return filepath.Join(homeDirForToken(), "AppData", "Roaming")
}

// ResolveToken finds the access token to send, and reports where it came from.
//
// SP_API_TOKEN wins so a user can point the bridge at a different profile, or
// override a stale file, without editing anything.
//
// The file fallback is what makes this work under an MCP host. A host launches
// the bridge as a subprocess, so the only way to supply an environment variable
// is to write it into the host's config file — which would put a live
// credential in a file people paste into bug reports. Reading SP's own file
// keeps the secret in the one place SP already put it.
//
// Returning no token is not an error: Super Productivity before 18.19.0 has no
// token, and sending an Authorization header there is pointless but harmless.
func ResolveToken() (string, TokenSource) {
	if env := strings.TrimSpace(os.Getenv(TokenEnvVar)); env != "" {
		if !usableAsHeaderValue(env) {
			// Deliberately not falling through to the file: the user set this
			// variable on purpose, and quietly authenticating as someone else
			// would be a worse answer than reporting the problem.
			return "", TokenSourceEnvInvalid
		}
		return env, TokenSourceEnv
	}

	for _, path := range tokenSearchPaths() {
		// Absent, unreadable, a directory, or far too large all mean "no token
		// from here". None is worth failing over: the request will 401 and SP's
		// own message says where to find the token.
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Size() > maxTokenFileBytes {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		token := strings.TrimSpace(string(data))
		if !spTokenPattern.MatchString(token) {
			// A file that exists but does not hold a token is not a token.
			// Sending its contents would produce an "invalid token" 401 that
			// points at the wrong problem.
			continue
		}
		return token, TokenSourceFile
	}
	return "", TokenSourceNone
}
