package version

import "fmt"

// Set via -ldflags at build time.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// String returns a formatted version string.
func String() string {
	return fmt.Sprintf("sp-local-bridge %s commit=%s date=%s", Version, Commit, Date)
}
