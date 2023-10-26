package cron

import (
	"strings"
	"testing"
)

// bitsOf builds a bitset from explicit values, for expectations.
func bitsOf(vals ...int) uint64 {
	var b uint64
	for _, v := range vals {
		b |= uint64(1) << uint(v)
	}
	return b
}

func rangeBits(lo, hi int) uint64 {
	var b uint64
	for i := lo; i <= hi; i++ {
		b |= uint64(1) << uint(i)
	}
	return b
}

func TestParseValid(t *testing.T) {
	all60 := rangeBits(0, 59)
	all24 := rangeBits(0, 23)
	allDom := rangeBits(1, 31)
	allMonth := rangeBits(1, 12)
	allDow := rangeBits(0, 6)

	tests := []struct {
		name string
		expr string
		min  uint64
		hour uint64
		dom  uint64
		mon  uint64
		dow  uint64
		domR bool
		dowR bool
	}{
		{"all stars", "* * * * *", all60, all24, allDom, allMonth, allDow, false, false},
		{"single values", "0 9 * * *", bitsOf(0), bitsOf(9), allDom, allMonth, allDow, false, false},
		{"minute step", "*/15 * * * *", bitsOf(0, 15, 30, 45), all24, allDom, allMonth, allDow, false, false},
		{"list", "0 0,12 * * *", bitsOf(0), bitsOf(0, 12), allDom, allMonth, allDow, false, false},
		{"range", "0 9-17 * * *", bitsOf(0), rangeBits(9, 17), allDom, allMonth, allDow, false, false},
		{"range step", "0 0 1-15/5 * *", bitsOf(0), bitsOf(0), bitsOf(1, 6, 11), allMonth, allDow, true, false},
		{"start step", "5/10 * * * *", bitsOf(5, 15, 25, 35, 45, 55), all24, allDom, allMonth, allDow, false, false},
		{"named month", "0 0 1 JAN *", bitsOf(0), bitsOf(0), bitsOf(1), bitsOf(1), allDow, true, false},
		{"named dow", "0 9 * * SAT", bitsOf(0), bitsOf(9), allDom, allMonth, bitsOf(6), false, true},
		{"named dow lower", "0 9 * * sat", bitsOf(0), bitsOf(9), allDom, allMonth, bitsOf(6), false, true},
		{"named range", "0 22 * * MON-FRI", bitsOf(0), bitsOf(22), allDom, allMonth, bitsOf(1, 2, 3, 4, 5), false, true},
		{"named month range", "0 0 1 JAN-MAR *", bitsOf(0), bitsOf(0), bitsOf(1), bitsOf(1, 2, 3), allDow, true, false},
		{"dow 7 is sunday", "0 0 * * 7", bitsOf(0), bitsOf(0), allDom, allMonth, bitsOf(0), false, true},
		{"combined list range step", "1,3,5-10/2 * * * *", bitsOf(1, 3, 5, 7, 9), all24, allDom, allMonth, allDow, false, false},
		{"macro daily", "@daily", bitsOf(0), bitsOf(0), allDom, allMonth, allDow, false, false},
		{"macro hourly", "@hourly", bitsOf(0), all24, allDom, allMonth, allDow, false, false},
		{"macro weekly", "@weekly", bitsOf(0), bitsOf(0), allDom, allMonth, bitsOf(0), false, true},
		{"macro yearly", "@yearly", bitsOf(0), bitsOf(0), bitsOf(1), bitsOf(1), allDow, true, false},
		{"extra whitespace", "  0   9 \t *  * * ", bitsOf(0), bitsOf(9), allDom, allMonth, allDow, false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := Parse(tc.expr)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tc.expr, err)
			}
			if s.Minute != tc.min {
				t.Errorf("minute = %b, want %b", s.Minute, tc.min)
			}
			if s.Hour != tc.hour {
				t.Errorf("hour = %b, want %b", s.Hour, tc.hour)
			}
			if s.Dom != tc.dom {
				t.Errorf("dom = %b, want %b", s.Dom, tc.dom)
			}
			if s.Month != tc.mon {
				t.Errorf("month = %b, want %b", s.Month, tc.mon)
			}
			if s.Dow != tc.dow {
				t.Errorf("dow = %b, want %b", s.Dow, tc.dow)
			}
			if s.DomRestricted != tc.domR {
				t.Errorf("domRestricted = %v, want %v", s.DomRestricted, tc.domR)
			}
			if s.DowRestricted != tc.dowR {
				t.Errorf("dowRestricted = %v, want %v", s.DowRestricted, tc.dowR)
			}
		})
	}
}

func TestParseInvalid(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantSub string
	}{
		{"empty", "", "empty cron expression"},
		{"too few", "* * * *", "expected 5 fields, got 4"},
		{"too many", "* * * * * *", "expected 5 fields, got 6"},
		{"minute out of range", "60 * * * *", "out of range"},
		{"hour out of range", "* 24 * * *", "out of range"},
		{"dom zero", "* * 0 * *", "out of range"},
		{"month out of range", "* * * 13 *", "out of range"},
		{"dow out of range", "* * * * 8", "out of range"},
		{"reversed range", "* * * * 5-1", "reversed"},
		{"zero step", "*/0 * * * *", "step must be >= 1"},
		{"negative step", "*/-1 * * * *", "step must be >= 1"},
		{"non-numeric step", "*/x * * * *", "non-numeric step"},
		{"empty list element", "1,,3 * * * *", "empty list element"},
		{"trailing comma", "1,3, * * * *", "empty list element"},
		{"non-numeric token", "abc * * * *", "invalid value"},
		{"unknown month", "* * * FOO *", "invalid value"},
		{"unknown dow", "* * * * FUN", "invalid value"},
		{"unsupported L", "* * L * *", "unsupported syntax"},
		{"unsupported W", "* * 15W * *", "unsupported syntax"},
		{"unsupported ?", "* * ? * *", "unsupported syntax"},
		{"reboot macro", "@reboot", "not supported"},
		{"unknown macro", "@frequently", "unknown macro"},
		{"macro with fields", "@daily echo hi", "does not take additional fields"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.expr)
			if err == nil {
				t.Fatalf("Parse(%q) = nil error, want error containing %q", tc.expr, tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("Parse(%q) error = %q, want substring %q", tc.expr, err.Error(), tc.wantSub)
			}
		})
	}
}
