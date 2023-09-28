package ntp

import (
	"testing"
	"time"
)

// TestEpochConstant pins the NTP<->Unix epoch relationship: Unix second 0 is
// NTP second 2208988800.
func TestEpochConstant(t *testing.T) {
	ts := TimestampFromTime(time.Unix(0, 0).UTC())
	if got := ts.Seconds(); got != 2208988800 {
		t.Fatalf("Unix 0 -> NTP seconds = %d, want 2208988800", got)
	}
	if got := ts.Fraction(); got != 0 {
		t.Fatalf("Unix 0 -> NTP fraction = %d, want 0", got)
	}

	// And the round trip lands back on the Unix epoch.
	back := ts.Time()
	if !back.Equal(time.Unix(0, 0).UTC()) {
		t.Fatalf("round trip of Unix 0 = %v, want 1970-01-01T00:00:00Z", back)
	}
}

// TestFractionExact verifies that fractions that divide evenly into the 32-bit
// binary fraction convert exactly.
func TestFractionExact(t *testing.T) {
	cases := []struct {
		nanos    int
		wantFrac uint32
	}{
		{0, 0x00000000},
		{250000000, 0x40000000}, // 0.25 s
		{500000000, 0x80000000}, // 0.50 s
		{750000000, 0xc0000000}, // 0.75 s
	}
	for _, c := range cases {
		ts := TimestampFromTime(time.Unix(100, int64(c.nanos)).UTC())
		if got := ts.Fraction(); got != c.wantFrac {
			t.Errorf("nanos=%d: fraction = %#08x, want %#08x", c.nanos, got, c.wantFrac)
		}
	}
}

// TestRoundTripPrecision checks that an arbitrary time round-trips through the
// 64-bit NTP representation to within 1 ns.
func TestRoundTripPrecision(t *testing.T) {
	orig := time.Date(2026, 6, 26, 12, 0, 0, 123456789, time.UTC)
	got := TimestampFromTime(orig).Time()

	diff := got.Sub(orig)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Nanosecond {
		t.Fatalf("round trip diff = %v, want <= 1ns (orig=%v got=%v)", diff, orig, got)
	}
}

// TestZeroTimestamp confirms the zero time<->zero timestamp mapping.
func TestZeroTimestamp(t *testing.T) {
	if ts := TimestampFromTime(time.Time{}); ts != 0 {
		t.Errorf("zero time -> %d, want 0", ts)
	}
	if tm := Timestamp(0).Time(); !tm.IsZero() {
		t.Errorf("zero timestamp -> %v, want zero time", tm)
	}
}
