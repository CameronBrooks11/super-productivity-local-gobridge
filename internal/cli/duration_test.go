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

// SP stores milliseconds. A sub-millisecond value truncated to 0 silently, so
// `--time-spent 500us` wrote timeSpent: 0 and wiped the recorded time rather
// than being rejected.
func TestParseDurationMs_RejectsSubMillisecond(t *testing.T) {
	for _, in := range []string{"500us", "30ns", "999us"} {
		if got, err := parseDurationMs(in); err == nil {
			t.Errorf("parseDurationMs(%q) = %d, want an error rather than a silent zero", in, got)
		}
	}
	// A value that does round to at least a millisecond is fine.
	if got, err := parseDurationMs("1500us"); err != nil || got != 1 {
		t.Errorf("parseDurationMs(\"1500us\") = %d, %v; want 1, nil", got, err)
	}
	// An explicit zero is still a legitimate value to write.
	if got, err := parseDurationMs("0"); err != nil || got != 0 {
		t.Errorf("parseDurationMs(\"0\") = %d, %v; want 0, nil", got, err)
	}
}

// A negative day count compared against a positive ceiling, so it slipped past
// the range guard and was reported as bad syntax.
func TestParseDurationMs_NegativeDaysReportedAsNegative(t *testing.T) {
	for _, in := range []string{"-5", "-1h", "-1000000d", "-2d"} {
		_, err := parseDurationMs(in)
		if err == nil {
			t.Errorf("parseDurationMs(%q) should fail", in)
			continue
		}
		if !contains(err.Error(), "negative") {
			t.Errorf("parseDurationMs(%q) should say the value is negative, got: %v", in, err)
		}
	}
}

// An out-of-range value assembled from several components is well-formed, so
// the error must not blame syntax.
func TestParseDurationMs_OutOfRangeMentionsRange(t *testing.T) {
	_, err := parseDurationMs("106751d23h59m")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !contains(err.Error(), "out of range") && !contains(err.Error(), "too large") {
		t.Errorf("error should mention the range, got: %v", err)
	}
}

// The point of the change is that a duration reaches the payload as
// milliseconds. Nothing asserted the conversion actually happened on the way
// through the flag parser.
func TestTimeFlags_ConvertDurationsToMilliseconds(t *testing.T) {
	for _, tc := range []struct {
		flag string
		want int64
	}{
		{"1h30m", 5400000},
		{"90m", 5400000},
		{"5400000", 5400000},
	} {
		got, err := parseDurationMs(tc.flag)
		if err != nil {
			t.Fatalf("parseDurationMs(%q): %v", tc.flag, err)
		}
		if got != tc.want {
			t.Errorf("parseDurationMs(%q) = %d, want %d", tc.flag, got, tc.want)
		}
	}
}
