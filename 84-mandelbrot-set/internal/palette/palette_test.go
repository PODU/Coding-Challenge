package palette

import (
	"image/color"
	"testing"
)

func TestNewUnknownName(t *testing.T) {
	if _, err := New("plaid", color.RGBA{}, false); err == nil {
		t.Fatalf("expected error for unknown palette name")
	}
}

func TestInsideColorForInSetPoints(t *testing.T) {
	inside := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	for _, name := range Names() {
		p, err := New(name, inside, false)
		if err != nil {
			t.Fatal(err)
		}
		got := p.Color(1000, false, 0, 1000)
		if got != inside {
			t.Fatalf("palette %s: in-set color = %v, want %v", name, got, inside)
		}
	}
}

func TestEscapedPointsOpaque(t *testing.T) {
	for _, name := range Names() {
		p, _ := New(name, color.RGBA{A: 255}, false)
		for _, it := range []int{1, 100, 500, 999} {
			c := p.Color(it, true, 1e6, 1000)
			if c.A != 255 {
				t.Fatalf("palette %s iter %d: alpha=%d, want 255", name, it, c.A)
			}
		}
	}
}

func TestParseHexColor(t *testing.T) {
	c, err := ParseHexColor("#FF8000")
	if err != nil {
		t.Fatal(err)
	}
	if c.R != 0xFF || c.G != 0x80 || c.B != 0x00 || c.A != 255 {
		t.Fatalf("ParseHexColor(#FF8000) = %v", c)
	}
	for _, bad := range []string{"", "000000", "#FFF", "#GGGGGG", "#1234567"} {
		if _, err := ParseHexColor(bad); err == nil {
			t.Fatalf("ParseHexColor(%q) should fail", bad)
		}
	}
}

func TestSmoothDeterministic(t *testing.T) {
	p, _ := New("fire", color.RGBA{A: 255}, true)
	a := p.Color(123, true, 5e4, 1000)
	b := p.Color(123, true, 5e4, 1000)
	if a != b {
		t.Fatalf("smooth color not deterministic: %v vs %v", a, b)
	}
}
