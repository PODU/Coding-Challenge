package ntp

import (
	"testing"
	"time"
)

// TestOffsetDelay feeds known four-timestamp vectors into the offset/delay
// math and asserts exact results.
//
//	theta = ((t1 - t0) + (t2 - t3)) / 2
//	delta = (t3 - t0) - (t2 - t1)
func TestOffsetDelay(t *testing.T) {
	base := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	ms := func(d int) time.Time { return base.Add(time.Duration(d) * time.Millisecond) }

	cases := []struct {
		name                  string
		t0, t1, t2, t3        time.Time
		wantOffset, wantDelay time.Duration
	}{
		{
			// Symmetric path, zero offset.
			name:       "zero-offset-symmetric",
			t0:         ms(0),
			t1:         ms(9),
			t2:         ms(11),
			t3:         ms(20),
			wantOffset: 0,
			wantDelay:  18 * time.Millisecond,
		},
		{
			// Positive offset (local clock behind the server).
			name:       "positive-offset",
			t0:         ms(0),
			t1:         ms(100),
			t2:         ms(150),
			t3:         ms(60),
			wantOffset: 95 * time.Millisecond,
			wantDelay:  10 * time.Millisecond,
		},
		{
			// Negative offset (local clock ahead of the server),
			// asymmetric path; offset has a half-millisecond component.
			name:       "negative-offset-asymmetric",
			t0:         ms(0),
			t1:         ms(5),
			t2:         ms(6),
			t3:         ms(40),
			wantOffset: -14500 * time.Microsecond,
			wantDelay:  39 * time.Millisecond,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CalculateOffset(c.t0, c.t1, c.t2, c.t3); got != c.wantOffset {
				t.Errorf("offset = %v, want %v", got, c.wantOffset)
			}
			if got := CalculateDelay(c.t0, c.t1, c.t2, c.t3); got != c.wantDelay {
				t.Errorf("delay = %v, want %v", got, c.wantDelay)
			}
		})
	}
}

// TestOffsetSubMicrosecond verifies the math preserves sub-millisecond
// precision down to nanoseconds.
func TestOffsetSubMicrosecond(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t0 := base
	t1 := base.Add(1234567 * time.Nanosecond)
	t2 := base.Add(1500000 * time.Nanosecond)
	t3 := base.Add(2000000 * time.Nanosecond)

	// offset = ((1234567) + (1500000 - 2000000)) / 2 = (1234567 - 500000)/2
	//        = 734567 / 2 = 367283 ns (integer division truncates .5)
	wantOffset := 367283 * time.Nanosecond
	if got := CalculateOffset(t0, t1, t2, t3); got != wantOffset {
		t.Errorf("offset = %v, want %v", got, wantOffset)
	}

	// delay = (2000000) - (1500000 - 1234567) = 2000000 - 265433 = 1734567 ns
	wantDelay := 1734567 * time.Nanosecond
	if got := CalculateDelay(t0, t1, t2, t3); got != wantDelay {
		t.Errorf("delay = %v, want %v", got, wantDelay)
	}
}
