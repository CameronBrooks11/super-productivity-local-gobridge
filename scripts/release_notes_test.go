// Package scripts holds tests for the shell scripts in this directory. There is
// no Go code here to test; these drive the scripts themselves, because
// release-notes.sh decides what the published release page says and a silent
// failure there is invisible until someone reads the release.
package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const sampleChangelog = `# Changelog

## [Unreleased]

## [0.3.2] - 2026-09-07

### Fixed

- the newest thing (#60)

## [0.3.1] - 2026-09-05

### Added

- the older thing (#12)

[0.3.2]: https://example.invalid/compare/v0.3.1...v0.3.2
`

// run invokes the script against a changelog written for the test, and reports
// stdout, stderr and the exit code.
func run(t *testing.T, changelog string, args ...string) (string, string, int) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("release-notes.sh needs a POSIX shell; the release job runs on ubuntu-latest")
	}

	path := filepath.Join(t.TempDir(), "CHANGELOG.md")
	if changelog != "" {
		if err := os.WriteFile(path, []byte(changelog), 0o644); err != nil {
			t.Fatalf("writing the changelog fixture: %v", err)
		}
	}

	cmd := exec.Command("./release-notes.sh", args...)
	cmd.Env = append(os.Environ(), "CHANGELOG_FILE="+path)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	code := 0
	var exitErr *exec.ExitError
	if err != nil {
		if !asExitError(err, &exitErr) {
			t.Fatalf("running the script: %v", err)
		}
		code = exitErr.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}

func asExitError(err error, target **exec.ExitError) bool {
	e, ok := err.(*exec.ExitError)
	if ok {
		*target = e
	}
	return ok
}

func TestExtractsOnlyTheRequestedSection(t *testing.T) {
	stdout, stderr, code := run(t, sampleChangelog, "v0.3.2")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "the newest thing (#60)") {
		t.Errorf("the requested section's content is missing:\n%s", stdout)
	}
	// The next version's content bleeding in is the failure that would put the
	// previous release's notes on this one.
	if strings.Contains(stdout, "the older thing") {
		t.Errorf("content from [0.3.1] leaked into the [0.3.2] notes:\n%s", stdout)
	}
	if strings.Contains(stdout, "[0.3.1]") {
		t.Errorf("the next heading was not treated as the end of the section:\n%s", stdout)
	}
	// The link definitions sit after the last section; reaching them means the
	// scan ran to the end of the file.
	if strings.Contains(stdout, "example.invalid") {
		t.Errorf("the scan ran past the section into the link definitions:\n%s", stdout)
	}
}

func TestVersionHeadingIsNotRepeatedInTheBody(t *testing.T) {
	stdout, stderr, code := run(t, sampleChangelog, "v0.3.2")
	// Without this the test passes on empty stdout, which is what a broken
	// script produces -- it would then be asserting nothing.
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if strings.TrimSpace(stdout) == "" {
		t.Fatal("stdout was empty; there is nothing to assert about")
	}
	if strings.Contains(stdout, "## [0.3.2]") {
		t.Errorf("the version heading should be dropped; the release page shows it already:\n%s", stdout)
	}
}

func TestEmptySectionIsAnError(t *testing.T) {
	// The dangerous case: goreleaser accepts an empty notes file and publishes a
	// release with a blank body, so a successful-looking run ships nothing.
	stdout, stderr, code := run(t, sampleChangelog, "Unreleased")
	if code == 0 {
		t.Errorf("an empty section exited 0; goreleaser would publish a blank release body")
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout should be empty on failure, got: %q", stdout)
	}
	if !strings.Contains(stderr, "no notes found") {
		t.Errorf("stderr should say why, got: %q", stderr)
	}
}

func TestUnknownVersionIsAnError(t *testing.T) {
	_, stderr, code := run(t, sampleChangelog, "9.9.9")
	if code == 0 {
		t.Errorf("an unknown version exited 0")
	}
	if !strings.Contains(stderr, "9.9.9") {
		t.Errorf("stderr should name the version asked for, got: %q", stderr)
	}
}

func TestMissingChangelogIsAnError(t *testing.T) {
	_, stderr, code := run(t, "", "0.3.2")
	if code == 0 {
		t.Errorf("a missing changelog exited 0")
	}
	if !strings.Contains(stderr, "no changelog") {
		t.Errorf("stderr should say the changelog is missing, got: %q", stderr)
	}
}

func TestTagAndBareVersionAgree(t *testing.T) {
	// The workflow passes GITHUB_REF_NAME, which carries the v; the heading does
	// not. If these ever disagree the release notes come out empty.
	withV, _, codeV := run(t, sampleChangelog, "v0.3.2")
	bare, _, codeBare := run(t, sampleChangelog, "0.3.2")
	if codeV != 0 || codeBare != 0 {
		t.Fatalf("exit codes: v-prefixed = %d, bare = %d, want 0 and 0", codeV, codeBare)
	}
	if withV != bare {
		t.Errorf("v0.3.2 and 0.3.2 produced different notes:\n--- v0.3.2 ---\n%s\n--- 0.3.2 ---\n%s", withV, bare)
	}
}

func TestWrongArgumentCountIsUsageError(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"no arguments", nil},
		{"two versions", []string{"0.3.2", "0.3.1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, code := run(t, sampleChangelog, tc.args...)
			if code != 2 {
				t.Errorf("exit = %d, want 2", code)
			}
			if !strings.Contains(stderr, "Usage:") {
				t.Errorf("stderr should show usage, got: %q", stderr)
			}
		})
	}
}
