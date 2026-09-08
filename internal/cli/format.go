package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/CameronBrooks11/super-productivity-local-gobridge/internal/bridge"
)

// Output formats.
//
// json stays the default. The CLI's JSON output is a documented interface and
// anything already parsing it has to keep working, so the human-readable modes
// are opt-in rather than a change of default.
type outputFormat string

const (
	formatJSON  outputFormat = "json"
	formatTable outputFormat = "table"
	formatIDs   outputFormat = "ids"
)

// absent is what a column shows when the field is not on the entity. SP
// distinguishes an absent field from an empty one and so does the bridge, but a
// table has to print something, and a blank cell reads as an alignment bug.
const absent = "-"

func parseFormat(s string) (outputFormat, error) {
	switch outputFormat(s) {
	case formatJSON, formatTable, formatIDs:
		return outputFormat(s), nil
	}
	return "", fmt.Errorf("Flag --format must be one of json, table, ids; got %q", s)
}

// extractFormat pulls --format out of an argument list and returns the rest.
//
// Removing it here leaves every existing parser untouched: --format shapes
// output rather than the request, so it must never reach the payload that goes
// to SP. The last occurrence wins, which is how the other value flags in this
// CLI already behave — --query a --query b sends b.
func extractFormat(args []string) (outputFormat, []string, error) {
	format := formatJSON
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] != "--format" {
			rest = append(rest, args[i])
			continue
		}
		if i+1 >= len(args) {
			return "", nil, fmt.Errorf("Flag --format requires a value")
		}
		f, err := parseFormat(args[i+1])
		if err != nil {
			return "", nil, err
		}
		format = f
		i++
	}
	return format, rest, nil
}

// rejectFullWithFormat refuses --full alongside a non-JSON format.
//
// --full turns off the bridge's field projection, which only means anything to
// a caller reading the whole entity. The table has a fixed column set, so
// accepting the flag there would leave it silently inert — the class of thing
// this repo's review gate keeps finding. Rejecting matches how --limit 0 and a
// stray second positional are already handled.
func rejectFullWithFormat(flags map[string]json.RawMessage, format outputFormat) error {
	if format == formatJSON {
		return nil
	}
	if _, ok := flags["full"]; ok {
		return fmt.Errorf("Flag --full applies to --format json only; table and ids have a fixed shape")
	}
	return nil
}

// idlessOps are the operations whose result is not an entity and carries no id,
// so --format ids has nothing to print. Rejected before the request goes out,
// so the user gets a usage error rather than a round trip to SP followed by a
// failure.
var idlessOps = map[string]bool{
	bridge.OpBridgeHealth: true,
	bridge.OpStatusGet:    true,
}

// column is one table column: a heading, and how to read it off an entity.
type column struct {
	head string
	cell func(map[string]any) string
}

var taskColumns = []column{
	{"ID", stringCell("id")},
	{"DONE", boolCell("isDone")},
	{"TITLE", stringCell("title")},
	{"DUE", stringCell("dueDay")},
	{"SPENT/EST", spentEstCell},
}

var projectColumns = []column{
	{"ID", stringCell("id")},
	{"TITLE", stringCell("title")},
	{"ARCHIVED", boolCell("isArchived")},
}

var tagColumns = []column{
	{"ID", stringCell("id")},
	{"TITLE", stringCell("title")},
}

// columnsFor maps a list operation to its table shape.
//
// Keyed on the operation rather than sniffed from the fields present: a task
// and a project both carry id and title, and an entity missing a field would
// otherwise be sorted into the wrong table by whatever it happened to have.
func columnsFor(op string) []column {
	switch op {
	case bridge.OpTaskList:
		return taskColumns
	case bridge.OpProjectList:
		return projectColumns
	case bridge.OpTagList:
		return tagColumns
	}
	return nil
}

func stringCell(field string) func(map[string]any) string {
	return func(e map[string]any) string {
		s, ok := e[field].(string)
		if !ok || s == "" {
			return absent
		}
		return s
	}
}

func boolCell(field string) func(map[string]any) string {
	return func(e map[string]any) string {
		b, ok := e[field].(bool)
		if !ok {
			return absent
		}
		if b {
			return "yes"
		}
		return "no"
	}
}

// spentEstCell renders timeSpent and timeEstimate as one column.
//
// Zero reads as absent. SP stores 0 for a task nobody has tracked or estimated,
// which is most of them, so printing 0s/0s down the whole column would be noise
// standing in for information.
func spentEstCell(e map[string]any) string {
	return durationCell(e["timeSpent"]) + "/" + durationCell(e["timeEstimate"])
}

// durationCell formats a millisecond field from a decoded response. The client
// unmarshals into `any`, so JSON numbers arrive as float64; that is exact below
// 2^53, which in milliseconds is longer than SP will ever be asked to track.
func durationCell(v any) string {
	ms, ok := v.(float64)
	if !ok || ms <= 0 {
		return absent
	}
	return formatDurationMs(int64(ms))
}

// writeResult renders a Result in the requested format and returns the exit
// code. It takes both streams so tests can read what each one got; printResult
// wires it to the process streams.
func writeResult(out, errOut io.Writer, result bridge.Result, op string, format outputFormat) int {
	if !result.OK {
		fmt.Fprintf(errOut, "Error [%s]: %s\n", result.Error.Code, result.Error.Message)
		return 1
	}

	switch format {
	case formatTable:
		renderTable(out, op, result.Data)
	case formatIDs:
		if err := renderIDs(out, result.Data); err != nil {
			fmt.Fprintf(errOut, "Error: %s\n", err)
			return 1
		}
	default:
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		enc.Encode(result.Data)
	}

	// stderr, so piping the output stays clean while a human still learns the
	// list was cut. Without it a truncated list looks like a complete one. It
	// stays on stderr in every format for that same reason: --format ids | xargs
	// has to receive ids and nothing else.
	if truncated, _ := result.Meta["truncated"].(bool); truncated {
		fmt.Fprintf(errOut, "Note: truncated — showing %v of %v matching items.\n",
			result.Meta["returned"], result.Meta["matched"])
	}
	return 0
}

func renderTable(w io.Writer, op string, data any) {
	switch v := data.(type) {
	case nil:
		return
	case []any:
		renderRows(w, columnsFor(op), v)
	case map[string]any:
		renderFields(w, v)
	default:
		// Neither an entity nor a list of them. Passing it through as JSON keeps
		// a surprise in the response visible instead of silently dropped.
		writeJSON(w, v)
	}
}

// renderRows prints a list as a heading plus one line per item.
//
// An empty list prints the heading and nothing else: "we looked and there were
// none" and "the command did nothing" should not look the same.
func renderRows(w io.Writer, cols []column, items []any) {
	if len(cols) == 0 {
		writeJSON(w, items)
		return
	}

	head := make([]string, len(cols))
	for i, c := range cols {
		head[i] = c.head
	}
	rows := [][]string{head}

	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			// shapeList passes a non-entity through rather than dropping it, so
			// the table does too — as a one-cell row carrying its JSON, which is
			// ragged on purpose and prints unpadded.
			rows = append(rows, []string{compactJSON(item)})
			continue
		}
		row := make([]string, len(cols))
		for i, c := range cols {
			row[i] = c.cell(obj)
		}
		rows = append(rows, row)
	}
	writeAligned(w, rows)
}

// renderFields prints a single entity as aligned KEY  VALUE lines.
//
// id and title lead because they are what identifies the thing; everything else
// is sorted, so the output is stable run to run rather than following Go's
// randomised map order.
func renderFields(w io.Writer, obj map[string]any) {
	var keys []string
	for k := range obj {
		if k == "id" || k == "title" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var lead []string
	for _, k := range []string{"id", "title"} {
		if _, ok := obj[k]; ok {
			lead = append(lead, k)
		}
	}
	keys = append(lead, keys...)

	rows := make([][]string, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, []string{k, fieldValue(k, obj[k])})
	}
	writeAligned(w, rows)
}

// fieldValue renders one field of a single-entity view.
//
// timeSpent and timeEstimate are humanised the same way the table's SPENT/EST
// column is, so the two views do not disagree about the same number. Nested
// values become compact JSON on one line: bridge.health returns health and
// status as nested objects, and a key/value view that dropped them would show
// two empty fields.
func fieldValue(key string, v any) string {
	if key == "timeSpent" || key == "timeEstimate" {
		return durationCell(v)
	}
	switch t := v.(type) {
	case nil:
		return absent
	case string:
		if t == "" {
			return absent
		}
		return t
	case bool:
		if t {
			return "yes"
		}
		return "no"
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	return compactJSON(v)
}

// renderIDs prints one id per line and nothing else, so the output pipes
// straight into xargs.
//
// An item with no id is an error rather than a blank line or a skipped row: the
// caller is about to act on every line it reads, and a silently short list is
// worse there than a failure.
func renderIDs(w io.Writer, data any) error {
	switch v := data.(type) {
	case nil:
		return nil
	case []any:
		for i, item := range v {
			obj, ok := item.(map[string]any)
			if !ok {
				return fmt.Errorf("--format ids: item %d is not an entity", i)
			}
			id, ok := obj["id"].(string)
			if !ok || id == "" {
				return fmt.Errorf("--format ids: item %d has no id", i)
			}
			fmt.Fprintln(w, id)
		}
		return nil
	case map[string]any:
		id, ok := v["id"].(string)
		if !ok || id == "" {
			return fmt.Errorf("--format ids: the result has no id")
		}
		fmt.Fprintln(w, id)
		return nil
	default:
		return fmt.Errorf("--format ids: the result is not an entity")
	}
}

// writeAligned prints rows as columns separated by two spaces, each column
// sized to its widest cell.
//
// Nothing is truncated. Sizing to content keeps the output deterministic and
// needs no terminal width, which in pure stdlib would cost a per-OS ioctl
// behind build tags across the Linux/macOS/Windows matrix for a cosmetic gain.
// Width is counted in runes, so a non-ASCII title lines up; it does not account
// for double-width glyphs.
//
// Rows may be ragged. A row shorter than the heading prints what it has, which
// is how a non-entity item stays visible instead of being dropped. The last
// cell of every row is never padded, so no line carries trailing whitespace.
func writeAligned(w io.Writer, rows [][]string) {
	widths := map[int]int{}
	for _, r := range rows {
		for i, cell := range r {
			if n := utf8.RuneCountInString(cell); n > widths[i] {
				widths[i] = n
			}
		}
	}
	var b strings.Builder
	for _, r := range rows {
		b.Reset()
		for i, cell := range r {
			b.WriteString(cell)
			if i < len(r)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-utf8.RuneCountInString(cell)+2))
			}
		}
		fmt.Fprintln(w, b.String())
	}
}

func writeJSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func compactJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}
