package term

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want []Event
	}{
		{"q", []Event{{Key: KeyRune, Rune: 'q'}}},
		{"é", []Event{{Key: KeyRune, Rune: 'é'}}},
		{"\x1b[A\x1b[B\x1b[C\x1b[D", []Event{{Key: KeyUp}, {Key: KeyDown}, {Key: KeyRight}, {Key: KeyLeft}}},
		{"\x1bOH\x1b[1~\x1b[H", []Event{{Key: KeyHome}, {Key: KeyHome}, {Key: KeyHome}}},
		{"\r\t\x7f\x03", []Event{{Key: KeyEnter}, {Key: KeyTab}, {Key: KeyBackspace}, {Key: KeyCtrlC}}},
		{"\x1b", []Event{{Key: KeyEsc}}},
		{"\x1b[3~\x1b[15~\x1b[Z", []Event{{Key: KeyDelete}, {Key: KeyF5}, {Key: KeyBacktab}}},
		{"\x1b[1;5A", []Event{{Key: KeyUp}}},
		{"\x1b[<0;10;5M\x1b[<0;10;5m", []Event{
			{Key: KeyMouse, X: 9, Y: 4, Button: 0, Press: true},
			{Key: KeyMouse, X: 9, Y: 4, Button: 0, Press: false}}},
		{"\x1b[<64;1;1M", []Event{{Key: KeyMouse, Button: 64, Press: true}}},
	}
	for _, tt := range tests {
		if got := Parse([]byte(tt.in)); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q: got %+v want %+v", tt.in, got, tt.want)
		}
	}
}

func TestCanvasText(t *testing.T) {
	c := NewCanvas(10, 2)
	if n := c.Text(1, 0, "héllo wörld", Default, 6); n != 6 {
		t.Fatalf("used %d", n)
	}
	if c.Row(0) != " héllo    " {
		t.Fatalf("%q", c.Row(0))
	}
	c.Text(0, 1, "日本語", Default, 5) // the third character would need 2 more columns
	if c.Row(1) != "日本      " {
		t.Fatalf("%q", c.Row(1))
	}
	c.Text(0, 0, "a\x1b[2Jb", Default, 10)
	if strings.Contains(c.Row(0), "\x1b") {
		t.Fatal("control characters must be neutralised")
	}
}

func TestTruncate(t *testing.T) {
	if Truncate("node_modules", 6) != "node_…" || Truncate("abc", 5) != "abc" || Truncate("abc", 0) != "" {
		t.Fatal(Truncate("node_modules", 6))
	}
}

func TestRendererDiff(t *testing.T) {
	c := NewCanvas(4, 2)
	var r Renderer
	first := string(r.Frame(c))
	if !strings.HasPrefix(first, "\x1b[0m\x1b[2J") {
		t.Fatalf("%q", first)
	}
	if out := r.Frame(c); len(out) != 0 {
		t.Fatalf("no change should send nothing: %q", out)
	}
	c.Set(2, 1, 'x', Style{Fg: 196, Bg: -1, Bold: true})
	c.Set(3, 1, 'y', Style{Fg: 196, Bg: -1, Bold: true})
	if out := string(r.Frame(c)); out != "\x1b[2;3H\x1b[0;1;38;5;196mxy" {
		t.Fatalf("%q", out)
	}
}
