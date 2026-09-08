package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/bridge"
)

// twoTasks mirrors testdata/fixtures/task-list-ok.json after the bridge's
// compact projection: one task with a due day and tracked time, one with
// neither. Numbers are float64 because the client unmarshals responses into
// `any`, which is what the renderer actually receives.
func twoTasks() []any {
	return []any{
		map[string]any{
			"id": "task-abc123", "title": "Review budget spreadsheet",
			"isDone": false, "dueDay": "2026-06-01",
			"timeSpent": float64(900000), "timeEstimate": float64(1800000),
		},
		map[string]any{
			"id": "task-def456", "title": "Send weekly report",
			"isDone":    true,
			"timeSpent": float64(0), "timeEstimate": float64(0),
		},
	}
}

func render(t *testing.T, op string, data any, format outputFormat) (string, string, int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := writeResult(&out, &errOut, bridge.Success(data), op, format)
	return out.String(), errOut.String(), code
}

// --- extractFormat ---

func TestExtractFormat_DefaultsToJSON(t *testing.T) {
	format, rest, err := extractFormat([]string{"list", "--limit", "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != formatJSON {
		t.Fatalf("expected json by default, got %q", format)
	}
	if strings.Join(rest, " ") != "list --limit 5" {
		t.Fatalf("args should pass through untouched, got %v", rest)
	}
}

func TestExtractFormat_RemovesFlagAndValue(t *testing.T) {
	for _, want := range []outputFormat{formatTable, formatIDs, formatJSON} {
		format, rest, err := extractFormat([]string{"list", "--format", string(want), "--limit", "5"})
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", want, err)
		}
		if format != want {
			t.Fatalf("expected %q, got %q", want, format)
		}
		// The flag must not survive into the payload parsers, which would
		// reject it as unknown.
		if got := strings.Join(rest, " "); got != "list --limit 5" {
			t.Fatalf("%s: expected the flag stripped, got %q", want, got)
		}
	}
}

func TestExtractFormat_MissingValue(t *testing.T) {
	if _, _, err := extractFormat([]string{"list", "--format"}); err == nil {
		t.Fatal("expected an error for --format with no value")
	}
}

func TestExtractFormat_UnknownValue(t *testing.T) {
	_, _, err := extractFormat([]string{"--format", "xml"})
	if err == nil {
		t.Fatal("expected an error for an unknown format")
	}
	if !strings.Contains(err.Error(), "json, table, ids") {
		t.Fatalf("the error should list the valid formats, got %q", err)
	}
}

// The other value flags in this CLI let the last occurrence win; --format must
// not quietly disagree.
func TestExtractFormat_LastOccurrenceWins(t *testing.T) {
	format, rest, err := extractFormat([]string{"--format", "table", "--format", "ids"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != formatIDs {
		t.Fatalf("expected ids, got %q", format)
	}
	if len(rest) != 0 {
		t.Fatalf("expected both occurrences stripped, got %v", rest)
	}
}

// --- rejectFullWithFormat ---

func TestRejectFullWithFormat(t *testing.T) {
	withFull := map[string]json.RawMessage{"full": json.RawMessage("true")}
	withoutFull := map[string]json.RawMessage{"limit": json.RawMessage("5")}

	if err := rejectFullWithFormat(withFull, formatJSON); err != nil {
		t.Fatalf("--full is a json flag and must be accepted there: %v", err)
	}
	if err := rejectFullWithFormat(withoutFull, formatTable); err != nil {
		t.Fatalf("table without --full must be accepted: %v", err)
	}
	for _, f := range []outputFormat{formatTable, formatIDs} {
		if err := rejectFullWithFormat(withFull, f); err == nil {
			t.Fatalf("expected --full to be rejected with --format %s", f)
		}
	}
}

// --- table rendering ---

func TestRenderTable_Tasks(t *testing.T) {
	out, _, code := render(t, bridge.OpTaskList, twoTasks(), formatTable)
	want := "" +
		"ID           DONE  TITLE                      DUE         SPENT/EST\n" +
		"task-abc123  no    Review budget spreadsheet  2026-06-01  15m/30m\n" +
		"task-def456  yes   Send weekly report         -           -/-\n"
	if out != want {
		t.Fatalf("table mismatch\n got:\n%s\nwant:\n%s", out, want)
	}
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestRenderTable_Projects(t *testing.T) {
	items := []any{
		map[string]any{"id": "project-1", "title": "Work", "isArchived": false},
		map[string]any{"id": "project-2", "title": "Old", "isArchived": true},
	}
	out, _, _ := render(t, bridge.OpProjectList, items, formatTable)
	want := "" +
		"ID         TITLE  ARCHIVED\n" +
		"project-1  Work   no\n" +
		"project-2  Old    yes\n"
	if out != want {
		t.Fatalf("project table mismatch\n got:\n%s\nwant:\n%s", out, want)
	}
}

func TestRenderTable_Tags(t *testing.T) {
	items := []any{map[string]any{"id": "tag-urgent", "title": "urgent"}}
	out, _, _ := render(t, bridge.OpTagList, items, formatTable)
	want := "" +
		"ID          TITLE\n" +
		"tag-urgent  urgent\n"
	if out != want {
		t.Fatalf("tag table mismatch\n got:\n%s\nwant:\n%s", out, want)
	}
}

// An empty list prints its heading: "we looked and there were none" must not
// look like "the command did nothing".
func TestRenderTable_EmptyListKeepsHeading(t *testing.T) {
	out, _, _ := render(t, bridge.OpTaskList, []any{}, formatTable)
	if out != "ID  DONE  TITLE  DUE  SPENT/EST\n" {
		t.Fatalf("expected the heading alone, got %q", out)
	}
}

// Each list operation gets its own columns; picking them off the fields present
// would put a project in the task table.
func TestColumnsFor_PerOperation(t *testing.T) {
	cases := map[string][]string{
		bridge.OpTaskList:    {"ID", "DONE", "TITLE", "DUE", "SPENT/EST"},
		bridge.OpProjectList: {"ID", "TITLE", "ARCHIVED"},
		bridge.OpTagList:     {"ID", "TITLE"},
	}
	for op, want := range cases {
		cols := columnsFor(op)
		if len(cols) != len(want) {
			t.Fatalf("%s: expected %d columns, got %d", op, len(want), len(cols))
		}
		for i, w := range want {
			if cols[i].head != w {
				t.Fatalf("%s column %d: expected %q, got %q", op, i, w, cols[i].head)
			}
		}
	}
	if columnsFor(bridge.OpTaskGet) != nil {
		t.Fatal("a non-list operation must declare no columns")
	}
}

// No line may carry trailing whitespace: it breaks diffing the output and shows
// up as stray blanks when the last column is empty.
func TestRenderTable_NoTrailingWhitespace(t *testing.T) {
	items := []any{map[string]any{"id": "task-1", "title": "x", "isDone": false}}
	out, _, _ := render(t, bridge.OpTaskList, items, formatTable)
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if line != strings.TrimRight(line, " ") {
			t.Fatalf("trailing whitespace on line %q", line)
		}
	}
}

// shapeList passes a non-entity through rather than dropping it, so the table
// has to keep it visible too.
func TestRenderTable_NonEntityItemStaysVisible(t *testing.T) {
	out, _, _ := render(t, bridge.OpTaskList, []any{"surprise"}, formatTable)
	if !strings.Contains(out, `"surprise"`) {
		t.Fatalf("expected the unexpected item to survive, got %q", out)
	}
}

func TestRenderTable_UnknownListOperationFallsBackToJSON(t *testing.T) {
	out, _, _ := render(t, "made.up", []any{map[string]any{"id": "x"}}, formatTable)
	if !strings.Contains(out, `"id": "x"`) {
		t.Fatalf("expected indented JSON for an operation with no columns, got %q", out)
	}
}

// --- single-entity rendering ---

func TestRenderFields_LeadsWithIDAndTitleThenSorts(t *testing.T) {
	entity := map[string]any{
		"zebra": "last", "title": "A task", "id": "task-1", "alpha": "first",
	}
	out, _, _ := render(t, bridge.OpTaskGet, entity, formatTable)
	want := "" +
		"id     task-1\n" +
		"title  A task\n" +
		"alpha  first\n" +
		"zebra  last\n"
	if out != want {
		t.Fatalf("field order mismatch\n got:\n%s\nwant:\n%s", out, want)
	}
}

// The field view and the SPENT/EST column must not disagree about the same
// number, so both humanise it.
func TestRenderFields_HumanisesDurations(t *testing.T) {
	entity := map[string]any{"id": "task-1", "timeSpent": float64(5400000), "timeEstimate": float64(0)}
	out, _, _ := render(t, bridge.OpTaskGet, entity, formatTable)
	if !strings.Contains(out, "timeSpent     1h30m") {
		t.Fatalf("expected timeSpent humanised, got %q", out)
	}
	if !strings.Contains(out, "timeEstimate  -") {
		t.Fatalf("expected an unset estimate to read as absent, got %q", out)
	}
}

// bridge.health nests two objects; dropping them would show two empty fields.
func TestRenderFields_NestedValuesBecomeCompactJSON(t *testing.T) {
	entity := map[string]any{"health": map[string]any{"server": "up"}}
	out, _, _ := render(t, bridge.OpBridgeHealth, entity, formatTable)
	if !strings.Contains(out, `{"server":"up"}`) {
		t.Fatalf("expected nested value as compact JSON, got %q", out)
	}
}

func TestRenderFields_NullReadsAsAbsent(t *testing.T) {
	out, _, _ := render(t, bridge.OpStatusGet, map[string]any{"currentTaskId": nil}, formatTable)
	if out != "currentTaskId  -\n" {
		t.Fatalf("expected null to render as absent, got %q", out)
	}
}

// --- ids rendering ---

func TestRenderIDs_ListAndSingleEntity(t *testing.T) {
	out, _, code := render(t, bridge.OpTaskList, twoTasks(), formatIDs)
	if out != "task-abc123\ntask-def456\n" {
		t.Fatalf("expected bare ids, got %q", out)
	}
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	out, _, _ = render(t, bridge.OpTaskGet, map[string]any{"id": "task-abc123"}, formatIDs)
	if out != "task-abc123\n" {
		t.Fatalf("expected one id, got %q", out)
	}
}

// `tasks current` with nothing tracked returns null: no ids and no failure.
func TestRenderIDs_NullIsEmptyAndSucceeds(t *testing.T) {
	out, errOut, code := render(t, bridge.OpTaskGetCurrent, nil, formatIDs)
	if out != "" || code != 0 {
		t.Fatalf("expected empty success, got out=%q code=%d stderr=%q", out, code, errOut)
	}
}

// The caller is about to act on every line it reads, so a missing id fails
// rather than emitting a short list.
func TestRenderIDs_MissingIDIsAnError(t *testing.T) {
	items := []any{map[string]any{"id": "task-1"}, map[string]any{"title": "no id here"}}
	out, errOut, code := render(t, bridge.OpTaskList, items, formatIDs)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(errOut, "item 1 has no id") {
		t.Fatalf("expected the offending index named, got %q", errOut)
	}
	// The ids before the bad one must not reach stdout: a pipeline that got
	// them would act on a short list, which is the thing the error exists to
	// prevent.
	if out != "" {
		t.Fatalf("expected nothing written on failure, got %q", out)
	}
}

func TestRenderIDs_NonEntityIsAnError(t *testing.T) {
	_, errOut, code := render(t, bridge.OpTaskList, []any{"surprise"}, formatIDs)
	if code != 1 || !strings.Contains(errOut, "not an entity") {
		t.Fatalf("expected a failure naming the shape, got code=%d stderr=%q", code, errOut)
	}
}

// health and status carry no id at all, and are refused before the request goes
// out rather than after.
func TestIdlessOps_AreExactlyHealthAndStatus(t *testing.T) {
	if len(idlessOps) != 2 || !idlessOps[bridge.OpBridgeHealth] || !idlessOps[bridge.OpStatusGet] {
		t.Fatalf("expected exactly health and status, got %v", idlessOps)
	}
}

// --- json stays exactly as it was ---

func TestWriteResult_JSONUnchangedByDefault(t *testing.T) {
	out, _, code := render(t, bridge.OpTaskList, []any{map[string]any{"id": "task-1"}}, formatJSON)
	want := "[\n  {\n    \"id\": \"task-1\"\n  }\n]\n"
	if out != want {
		t.Fatalf("json output changed\n got: %q\nwant: %q", out, want)
	}
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestWriteResult_ErrorGoesToStderrInEveryFormat(t *testing.T) {
	for _, f := range []outputFormat{formatJSON, formatTable, formatIDs} {
		var out, errOut bytes.Buffer
		result := bridge.Failure("SP_UNAVAILABLE", "SP is not running.")
		code := writeResult(&out, &errOut, result, bridge.OpTaskList, f)
		if code != 1 {
			t.Fatalf("%s: expected exit 1, got %d", f, code)
		}
		if out.String() != "" {
			t.Fatalf("%s: stdout must stay empty on failure, got %q", f, out.String())
		}
		if !strings.Contains(errOut.String(), "Error [SP_UNAVAILABLE]") {
			t.Fatalf("%s: expected the error on stderr, got %q", f, errOut.String())
		}
	}
}

// `--format ids | xargs` must receive ids and nothing else, so the truncation
// note stays on stderr in every format.
func TestWriteResult_TruncationNoteStaysOnStderr(t *testing.T) {
	for _, f := range []outputFormat{formatJSON, formatTable, formatIDs} {
		var out, errOut bytes.Buffer
		result := bridge.Success([]any{map[string]any{"id": "task-1"}})
		result.Meta = map[string]any{"truncated": true, "returned": 1, "matched": 9}
		writeResult(&out, &errOut, result, bridge.OpTaskList, f)
		if !strings.Contains(errOut.String(), "showing 1 of 9") {
			t.Fatalf("%s: expected the note on stderr, got %q", f, errOut.String())
		}
		if strings.Contains(out.String(), "truncated") {
			t.Fatalf("%s: the note must not reach stdout, got %q", f, out.String())
		}
	}
}

// --- formatDurationMs ---

func TestFormatDurationMs(t *testing.T) {
	cases := map[int64]string{
		0:       "0s",
		500:     "<1s",
		1000:    "1s",
		45000:   "45s",
		1500000: "25m",
		1800000: "30m",
		3600000: "1h",
		5400000: "1h30m",
		7805000: "2h10m5s",
	}
	for ms, want := range cases {
		if got := formatDurationMs(ms); got != want {
			t.Errorf("formatDurationMs(%d): expected %q, got %q", ms, want, got)
		}
	}
}

// What the table prints must be typeable straight back into --time-estimate.
func TestFormatDurationMs_RoundTripsThroughParse(t *testing.T) {
	for _, ms := range []int64{1000, 45000, 1500000, 3600000, 5400000, 7805000} {
		text := formatDurationMs(ms)
		back, err := parseDurationMs(text)
		if err != nil {
			t.Fatalf("parseDurationMs(%q) from %d ms: %v", text, ms, err)
		}
		if back != ms {
			t.Fatalf("round trip of %d ms via %q gave %d", ms, text, back)
		}
	}
}

// --- wiring through Run ---
//
// Exit 1 means the arguments parsed and the request went out to an address
// nothing answers; exit 2 means the arguments were rejected. That is the
// distinction these assert, following the pattern in cli_extra_test.go.
// SP_BASE_URL is set on every case, including the ones that should never reach
// the network, so a regression cannot send a request to a real Super
// Productivity on the default port.

func TestRun_FormatFlagAcceptedOnEveryCommandShape(t *testing.T) {
	cases := [][]string{
		{"health", "--format", "table"},
		{"status", "--format", "table"},
		{"tasks", "list", "--format", "table"},
		{"tasks", "list", "--limit", "5", "--format", "ids"},
		{"tasks", "get", "task-1", "--format", "table"},
		{"tasks", "current", "--format", "ids"},
		{"tasks", "complete", "task-1", "--format", "table"},
		{"projects", "list", "--format", "table"},
		{"tags", "list", "--format", "ids"},
		// These two matter most: --format has to be stripped before the
		// per-command flag parsers see it, or they reject it as unknown.
		{"tasks", "add", "A title", "--format", "table"},
		{"tasks", "update", "task-1", "--title", "New", "--format", "ids"},
	}
	for _, args := range cases {
		t.Setenv("SP_BASE_URL", "http://127.0.0.1:1")
		if code := Run(args); code != 1 {
			t.Errorf("%v: expected exit 1 (parsed, SP down), got %d", args, code)
		}
	}
}

func TestRun_RejectsBadFormatEverywhere(t *testing.T) {
	cases := [][]string{
		{"health", "--format", "xml"},
		{"status", "--format"},
		{"tasks", "list", "--format", "xml"},
		{"tasks", "get", "task-1", "--format", "xml"},
		{"tasks", "add", "A title", "--format", "xml"},
		{"projects", "list", "--format", "xml"},
		{"tags", "list", "--format"},
	}
	for _, args := range cases {
		t.Setenv("SP_BASE_URL", "http://127.0.0.1:1")
		if code := Run(args); code != 2 {
			t.Errorf("%v: expected exit 2 for a bad format, got %d", args, code)
		}
	}
}

func TestRun_RejectsFullWithNonJSONFormat(t *testing.T) {
	rejected := [][]string{
		{"tasks", "list", "--full", "--format", "table"},
		{"tasks", "list", "--full", "--format", "ids"},
		{"projects", "list", "--full", "--format", "table"},
		{"tags", "list", "--full", "--format", "ids"},
	}
	for _, args := range rejected {
		t.Setenv("SP_BASE_URL", "http://127.0.0.1:1")
		if code := Run(args); code != 2 {
			t.Errorf("%v: expected exit 2, got %d", args, code)
		}
	}

	// --full keeps working on its own, and with an explicit --format json.
	for _, args := range [][]string{
		{"tasks", "list", "--full"},
		{"tasks", "list", "--full", "--format", "json"},
	} {
		t.Setenv("SP_BASE_URL", "http://127.0.0.1:1")
		if code := Run(args); code != 1 {
			t.Errorf("%v: expected exit 1 (still accepted), got %d", args, code)
		}
	}
}

// health and status are refused before any request goes out, so this case never
// touches the network at all.
func TestRun_RejectsIDsFormatOnIdlessCommands(t *testing.T) {
	for _, args := range [][]string{{"health", "--format", "ids"}, {"status", "--format", "ids"}} {
		t.Setenv("SP_BASE_URL", "http://127.0.0.1:1")
		if code := Run(args); code != 2 {
			t.Errorf("%v: expected exit 2, got %d", args, code)
		}
	}
}

// --- guards the first mutation sweep did not cover ---

func TestStringCell_EmptyAndMissingReadAsAbsent(t *testing.T) {
	items := []any{map[string]any{"id": "task-1", "title": "", "isDone": false}}
	out, _, _ := render(t, bridge.OpTaskList, items, formatTable)
	want := "" +
		"ID      DONE  TITLE  DUE  SPENT/EST\n" +
		"task-1  no    -      -    -/-\n"
	if out != want {
		t.Fatalf("an empty string and a missing field should both read as absent\n got:\n%s\nwant:\n%s", out, want)
	}
}

func TestBoolCell_MissingReadsAsAbsent(t *testing.T) {
	items := []any{map[string]any{"id": "task-1", "title": "x"}}
	out, _, _ := render(t, bridge.OpTaskList, items, formatTable)
	if !strings.Contains(out, "task-1  -     x") {
		t.Fatalf("a missing isDone should read as absent, got %q", out)
	}
}

// `tasks stop-current` answers 204 and `tasks current` answers null when
// nothing is tracked. Printing nothing would make either indistinguishable from
// a command that did nothing.
func TestRenderTable_NilResultIsVisible(t *testing.T) {
	out, _, code := render(t, bridge.OpTaskStopCurrent, nil, formatTable)
	if out != "-\n" {
		t.Fatalf("expected an absent marker, got %q", out)
	}
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

// created is 1780000000000; %g would print 1.78e+12, which is not an id, a
// timestamp anyone can read, or something that round-trips.
func TestFieldValue_LargeIntegersPrintInFull(t *testing.T) {
	out, _, _ := render(t, bridge.OpTaskGet,
		map[string]any{"id": "task-1", "created": float64(1780000000000)}, formatTable)
	if !strings.Contains(out, "created  1780000000000") {
		t.Fatalf("expected the full integer, got %q", out)
	}
}

// Byte-counted widths push a row right by one column per extra UTF-8 byte. This
// is the behaviour writeAligned's comment claims, so it needs a guard.
func TestWriteAligned_WidthIsCountedInRunesNotBytes(t *testing.T) {
	items := []any{
		map[string]any{"id": "task-1", "title": "café", "isDone": false},
		map[string]any{"id": "task-2", "title": "abcde", "isDone": false},
	}
	out, _, _ := render(t, bridge.OpTaskList, items, formatTable)
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	// "café" is 4 runes and 5 bytes. The comparison has to be in runes too:
	// strings.Index returns a byte offset, which differs between these rows
	// even when they line up on screen.
	col := func(line string) int {
		i := strings.Index(line, "-    -/-")
		if i < 0 {
			return -1
		}
		return utf8.RuneCountInString(line[:i])
	}
	if col(lines[1]) == -1 || col(lines[1]) != col(lines[2]) {
		t.Fatalf("rows misaligned by byte-counted width:\n%s", out)
	}
}

// --- edge inputs found in review ---

// notes are routinely multi-line; an unescaped break prints as a line with no
// key, which reads as a field whose name is empty.
func TestRenderFields_NewlineInAValueStaysOnOneLine(t *testing.T) {
	entity := map[string]any{"id": "task-1", "notes": "line one\nline two"}
	out, _, _ := render(t, bridge.OpTaskGet, entity, formatTable)
	if strings.Count(out, "\n") != 2 {
		t.Fatalf("expected exactly two lines, got %q", out)
	}
	if !strings.Contains(out, `"line one\nline two"`) {
		t.Fatalf("expected the break escaped, got %q", out)
	}
}

func TestRenderTable_NewlineInATitleStaysOnOneLine(t *testing.T) {
	items := []any{map[string]any{"id": "task-1", "title": "one\ntwo", "isDone": false}}
	out, _, _ := render(t, bridge.OpTaskList, items, formatTable)
	if strings.Count(out, "\n") != 2 {
		t.Fatalf("expected a heading and one row, got %q", out)
	}
}

// A stray non-entity carries its JSON in a single cell. Letting that cell into
// the width measurement stretched the ID column for every row.
func TestRenderTable_NonEntityItemDoesNotWidenColumns(t *testing.T) {
	items := []any{
		map[string]any{"id": "task-1", "title": "Real task", "isDone": false},
		"an unexpectedly long string item from SP",
	}
	out, _, _ := render(t, bridge.OpTaskList, items, formatTable)
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if !strings.HasPrefix(lines[0], "ID      DONE") {
		t.Fatalf("the stray item widened the ID column: %q", lines[0])
	}
	if !strings.Contains(out, "unexpectedly long string item") {
		t.Fatalf("the stray item should still be visible, got %q", out)
	}
}

// --format is extracted before the per-command parsers run, so it has to skip
// over a flag's value instead of reading it. Searching for the literal string
// "--format" worked before this flag existed and has to keep working.
func TestExtractFormat_DoesNotClaimAnotherFlagsValue(t *testing.T) {
	format, rest, err := extractFormat([]string{"list", "--query", "--format"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != formatJSON {
		t.Fatalf("the value of --query is not a format, got %q", format)
	}
	if strings.Join(rest, " ") != "list --query --format" {
		t.Fatalf("expected the query value preserved, got %v", rest)
	}

	// ...and a real --format after a value flag is still found.
	format, rest, err = extractFormat([]string{"list", "--query", "report", "--format", "ids"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != formatIDs {
		t.Fatalf("expected ids, got %q", format)
	}
	if strings.Join(rest, " ") != "list --query report" {
		t.Fatalf("expected the flag stripped, got %v", rest)
	}
}

func TestRun_QueryValueThatLooksLikeTheFormatFlag(t *testing.T) {
	t.Setenv("SP_BASE_URL", "http://127.0.0.1:1")
	// Exit 1 means it parsed and went out to a dead address; before the fix
	// this was a usage error.
	if code := Run([]string{"tasks", "list", "--query", "--format"}); code != 1 {
		t.Errorf("expected exit 1 (parsed, SP down), got %d", code)
	}
}
