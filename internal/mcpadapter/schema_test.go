package mcpadapter

import (
	"testing"

	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/bridge"
)

// The published schema is a promise: a client that validates against it and
// passes should not then be rejected by the bridge. That promise has broken
// twice on this surface — once when limit's minimum said 0 while validation
// required 1, and again when the fix reached only one of the three tools
// because the others are indented differently and the edit was
// whitespace-sensitive. A test comparing schema to enforcement catches both.
func TestSchemas_PagingBoundsMatchValidation(t *testing.T) {
	s := NewServer(bridge.NewService(bridge.NewClient("http://127.0.0.1:1")))

	listTools := map[string]bool{"list_tasks": true, "list_projects": true, "list_tags": true}
	seen := map[string]bool{}

	for _, tool := range s.tools {
		name := tool.Name
		if !listTools[name] {
			continue
		}
		seen[name] = true
		props, _ := tool.InputSchema["properties"].(map[string]any)

		for field, wantMin := range map[string]int{"limit": 1, "offset": 0} {
			spec, ok := props[field].(map[string]any)
			if !ok {
				t.Errorf("%s: schema is missing %q", name, field)
				continue
			}
			if got, _ := spec["minimum"].(int); got != wantMin {
				t.Errorf("%s.%s: schema minimum is %v, but validation requires %d", name, field, spec["minimum"], wantMin)
			}
			if got, _ := spec["maximum"].(int); got != bridge.MaxListLimit {
				t.Errorf("%s.%s: schema maximum is %v, but validation rejects above %d", name, field, spec["maximum"], bridge.MaxListLimit)
			}
		}
		if _, ok := props["full"]; !ok {
			t.Errorf("%s: schema is missing \"full\"", name)
		}
	}
	for name := range listTools {
		if !seen[name] {
			t.Errorf("%s is not registered, so its schema was never checked", name)
		}
	}
}
