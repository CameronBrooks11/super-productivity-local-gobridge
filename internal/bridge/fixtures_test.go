package bridge

// This guard is filesystem-only — it needs no running Super Productivity — so
// it deliberately lives outside the `live` build tag. Behind that tag CI never
// ran it, and a response fixture added without a corresponding entry would have
// reached a release unchecked: the silent gap the live suite exists to close,
// reproduced in the guard against it.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// checkedFixtures is the set TestLive_FixturesDoNotInventFields covers, named
// separately so TestLive_EveryResponseFixtureIsChecked can hold it to account.
var checkedFixtures = []string{
	"task-list-ok.json",
	"task-create-ok.json",
	"task-update-ok.json",
	"project-list-ok.json",
	"tag-list-ok.json",
	"status-ok.json",
	"health-ok.json",
}

// A fixture added later is silently unchecked unless someone remembers to list
// it, which is the same kind of quiet gap this suite exists to close. This
// fails when testdata holds a response fixture the drift check does not cover.
//
// Request fixtures describe what the bridge sends, and error fixtures describe
// failures a read-only run cannot provoke, so both are excluded by name.
func TestFixtures_EveryResponseFixtureIsChecked(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading fixtures: %v", err)
	}
	checked := map[string]bool{}
	for _, name := range checkedFixtures {
		checked[name] = true
	}
	for _, e := range entries {
		name := e.Name()
		switch {
		case !strings.HasSuffix(name, ".json"),
			strings.HasSuffix(name, "-request.json"),
			strings.Contains(name, "-error"):
			continue
		}
		if !checked[name] {
			t.Errorf("%s is a response fixture but is not in checkedFixtures, so nothing verifies it against SP", name)
		}
	}
	for name := range checked {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("checkedFixtures names %s, which does not exist", name)
		}
	}
}
