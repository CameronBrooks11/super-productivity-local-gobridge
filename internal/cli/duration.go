package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseDurationMs converts a CLI time value to milliseconds.
//
// Super Productivity stores durations in milliseconds, so setting a 1h30m
// estimate meant typing 5400000 — easy to get wrong by an order of magnitude
// and impossible to read back.
//
// A bare integer is still milliseconds, so every existing invocation and script
// keeps working. Anything with a unit suffix is parsed as a duration:
//
//	--time-estimate 5400000   -> 5400000ms (unchanged)
//	--time-estimate 1h30m     -> 5400000ms
//	--time-estimate 90m       -> 5400000ms
//
// Days are accepted as a convenience since time.ParseDuration stops at hours,
// and a multi-day estimate is otherwise an awkward number of hours.
func parseDurationMs(s string) (int64, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0, fmt.Errorf("empty duration")
	}

	// A bare integer keeps its historical meaning: milliseconds.
	if ms, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		if ms < 0 {
			return 0, fmt.Errorf("duration must not be negative")
		}
		return ms, nil
	}

	expanded, err := expandDays(trimmed)
	if err != nil {
		return 0, err
	}

	d, err := time.ParseDuration(expanded)
	if err != nil {
		return 0, fmt.Errorf("not a duration: use milliseconds (5400000) or a unit suffix (1h30m, 90m, 2d)")
	}
	if d < 0 {
		return 0, fmt.Errorf("duration must not be negative")
	}
	return d.Milliseconds(), nil
}

// expandDays rewrites a leading "<n>d" as hours, since time.ParseDuration has no
// day unit. Only a leading day component is supported, which is how durations
// are written in practice ("2d4h", never "4h2d").
func expandDays(s string) (string, error) {
	i := strings.IndexByte(s, 'd')
	if i < 0 {
		return s, nil
	}
	// "d" inside a unit we already understand (e.g. none today) or trailing
	// text after the day marker that itself contains "d" is not supported.
	if strings.ContainsRune(s[i+1:], 'd') {
		return "", fmt.Errorf("only one day component is supported, e.g. 2d4h")
	}
	days, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return "", fmt.Errorf("not a duration: use milliseconds (5400000) or a unit suffix (1h30m, 90m, 2d)")
	}
	// %g switches to exponent notation for large values, which ParseDuration
	// cannot read — so a well-formed "1000000d" was reported as malformed
	// syntax, sending the reader to fix the wrong thing.
	// time.Duration is int64 nanoseconds, so it tops out around 292 years. A
	// value past that is well-formed but out of range, and saying "not a
	// duration" would send the reader to fix syntax that is already correct.
	const maxHours = 2562047.0
	if days*24 > maxHours {
		return "", fmt.Errorf("duration is too large: the maximum is about %d days", int(maxHours)/24)
	}
	hours := strconv.FormatFloat(days*24, 'f', -1, 64)
	rest := s[i+1:]
	if rest == "" {
		return hours + "h", nil
	}
	return hours + "h" + rest, nil
}
