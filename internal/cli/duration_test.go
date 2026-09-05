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

// Days expand to hours, and a large day count used to render in exponent
// notation that ParseDuration cannot read — so a well-formed value was
// reported as bad syntax, sending the reader to fix something already correct.
func TestParseDurationMs_LargeDayCounts(t *testing.T) {
	for _, in := range []string{"100d", "1000d", "10000d"} {
		if _, err := parseDurationMs(in); err != nil {
			t.Errorf("parseDurationMs(%q) should succeed, got: %v", in, err)
		}
	}
	// Past time.Duration's int64-nanosecond range the value is well-formed but
	// out of range, and the error should say so rather than blaming syntax.
	_, err := parseDurationMs("1000000d")
	if err == nil {
		t.Fatal("expected an error for an out-of-range duration")
	}
	if !contains(err.Error(), "too large") {
		t.Errorf("error should say the value is out of range, got: %v", err)
	}
}
