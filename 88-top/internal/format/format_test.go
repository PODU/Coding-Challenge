package format

import (
	"testing"

	"github.com/example/cctop/internal/model"
)

func TestBytes(t *testing.T) {
	const gib15 = uint64(1024 * 1024 * 1024 * 3 / 2) // 1.5 GiB
	cases := []struct {
		name string
		b    uint64
		unit model.MemUnit
		want string
	}{
		{"zero auto", 0, model.UnitAuto, "0B"},
		{"1023 auto stays bytes", 1023, model.UnitAuto, "1023B"},
		{"1024 auto -> KiB", 1024, model.UnitAuto, "1.0Ki"},
		{"1536 auto -> 1.5Ki", 1536, model.UnitAuto, "1.5Ki"},
		{"1.5GiB auto", gib15, model.UnitAuto, "1.5Gi"},
		{"forced KiB", 1024, model.UnitKiB, "1.0Ki"},
		{"forced MiB", 1024 * 1024, model.UnitMiB, "1.0Mi"},
		{"forced GiB", 1024 * 1024 * 1024, model.UnitGiB, "1.0Gi"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Bytes(c.b, c.unit); got != c.want {
				t.Errorf("Bytes(%d, %v) = %q, want %q", c.b, c.unit, got, c.want)
			}
		})
	}
}

func TestTimePlus(t *testing.T) {
	cases := []struct {
		secs float64
		want string
	}{
		{0, "0:00.00"},
		{11.07, "0:11.07"},
		{62.44, "1:02.44"},
		{3661, "1:01:01"},  // 1h 1m 1s
		{11045, "3:04:05"}, // 3h 4m 5s
		{-5, "0:00.00"},    // negative guarded
	}
	for _, c := range cases {
		if got := TimePlus(c.secs); got != c.want {
			t.Errorf("TimePlus(%v) = %q, want %q", c.secs, got, c.want)
		}
	}
}

func TestPercent(t *testing.T) {
	cases := []struct {
		p    float64
		want string
	}{
		{6.34, "6.3"},
		{100, "100.0"},
		{0, "0.0"},
		{91.62, "91.6"},
		{2.17, "2.2"},
	}
	for _, c := range cases {
		if got := Percent(c.p); got != c.want {
			t.Errorf("Percent(%v) = %q, want %q", c.p, got, c.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		s     string
		width int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 3, "he…"},
		{"hello", 1, "…"},
		{"hello", 0, ""},
		{"héllo", 3, "hé…"}, // multibyte counted by rune
	}
	for _, c := range cases {
		if got := Truncate(c.s, c.width); got != c.want {
			t.Errorf("Truncate(%q,%d) = %q, want %q", c.s, c.width, got, c.want)
		}
	}
}

func TestPad(t *testing.T) {
	if got := PadLeft("42", 5); got != "   42" {
		t.Errorf("PadLeft = %q", got)
	}
	if got := PadRight("ab", 5); got != "ab   " {
		t.Errorf("PadRight = %q", got)
	}
	if got := PadLeft("toolong", 4); got != "too…" {
		t.Errorf("PadLeft truncate = %q", got)
	}
}

func TestUptime(t *testing.T) {
	cases := []struct {
		secs int64
		want string
	}{
		{0, "0:00"},
		{3*3600 + 21*60, "3:21"},
		{86400 + 3600, "1 day, 1:00"},
		{2*86400 + 5*3600 + 7*60, "2 days, 5:07"},
	}
	for _, c := range cases {
		if got := Uptime(c.secs); got != c.want {
			t.Errorf("Uptime(%d) = %q, want %q", c.secs, got, c.want)
		}
	}
}
