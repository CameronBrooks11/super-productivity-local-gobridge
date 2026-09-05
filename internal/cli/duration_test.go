package cli

import "testing"

func TestParseDurationMs(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		// A bare integer keeps its historical meaning, so existing scripts and
		// every documented invocation keep working unchanged.
		{"5400000", 5400000, false},
		{"0", 0, false},
		{"1", 1, false},

		{"1h30m", 5400000, false},
		{"90m", 5400000, false},
		{"1h", 3600000, false},
		{"45s", 45000, false},
		{"1.5h", 5400000, false},
		{"2d", 172800000, false},
		{"1d12h", 129600000, false},
		{" 90m ", 5400000, false},

		{"", 0, true},
		{"-5", 0, true},
		{"-1h", 0, true},
		{"bogus", 0, true},
		{"90x", 0, true},
		{"1d2d", 0, true},
		{"h", 0, true},
	}
	for _, tc := range cases {
		got, err := parseDurationMs(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseDurationMs(%q) = %d, want an error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseDurationMs(%q) errored: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseDurationMs(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// The error names both accepted forms, since the whole point is that a caller
// no longer has to know which one we want.
func TestParseDurationMs_ErrorIsActionable(t *testing.T) {
	_, err := parseDurationMs("bogus")
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"5400000", "1h30m"} {
		if !contains(err.Error(), want) {
			t.Errorf("error should show %q as an example, got: %v", want, err)
		}
	}
}

func TestFormatDurationMs(t *testing.T) {
	cases := map[int64]string{
		0:       "0",
		5400000: "1h30m",
		3600000: "1h",
		1800000: "30m",
		45000:   "45s",
	}
	for ms, want := range cases {
		if got := formatDurationMs(ms); got != want {
			t.Errorf("formatDurationMs(%d) = %q, want %q", ms, got, want)
		}
	}
}

// Anything formatDurationMs prints must parse back to the same value, so a
// displayed duration can be pasted into a command.
func TestDurationRoundTrip(t *testing.T) {
	for _, ms := range []int64{0, 45000, 1800000, 3600000, 5400000, 172800000} {
		s := formatDurationMs(ms)
		back, err := parseDurationMs(s)
		if err != nil {
			t.Errorf("formatDurationMs(%d) = %q, which does not parse: %v", ms, s, err)
			continue
		}
		if back != ms {
			t.Errorf("round trip of %d via %q gave %d", ms, s, back)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
