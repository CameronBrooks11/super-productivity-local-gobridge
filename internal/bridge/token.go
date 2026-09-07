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
)

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

// TokenPath is the file SP writes its access token to on this platform.
func TokenPath() string {
	return filepath.Join(spUserDataDir(), tokenFileName)
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
		return env, TokenSourceEnv
	}

	data, err := os.ReadFile(TokenPath())
	if err != nil {
		// Absent, unreadable, or a directory. All mean "no token from here",
		// and none is worth failing over: the request will 401 and SP's own
		// message says where to find the token.
		return "", TokenSourceNone
	}

	token := strings.TrimSpace(string(data))
	if !spTokenPattern.MatchString(token) {
		// A file that exists but does not hold a token is not a token. Sending
		// its contents would produce an "invalid token" 401 that points at the
		// wrong problem.
		return "", TokenSourceNone
	}
	return token, TokenSourceFile
}
