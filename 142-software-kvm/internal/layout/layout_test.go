package layout

import (
	"strings"
	"testing"
)

const lShaped = `
# An L: desktop in the corner, laptop to its right, tablet below it.
section: screens
	desktop:
	laptop:
		halfDuplexCapsLock = true
	tablet:
end

section: aliases
	laptop:
		laptop.local
end

section: links
	desktop:
		right = laptop
		down  = tablet
	laptop:
		left = desktop
	tablet:
		up = Desktop      # names are case-insensitive
end

section: options
	relativeMouseMoves = true
	keepAlive = 3000
end
`

func TestParseLShaped(t *testing.T) {
	c, err := Parse(strings.NewReader(lShaped))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(c.Screens(), ","); got != "desktop,laptop,tablet" {
		t.Fatalf("screens: %s", got)
	}
	cases := []struct {
		from string
		d    Direction
		want string
	}{
		{"desktop", Right, "laptop"},
		{"desktop", Down, "tablet"},
		{"laptop", Left, "desktop"},
		{"tablet", Up, "desktop"},
		{"laptop", Down, ""},
		{"desktop", Left, ""},
	}
	for _, tc := range cases {
		got, _ := c.Neighbor(tc.from, tc.d)
		if got != tc.want {
			t.Errorf("%s %s: got %q want %q", tc.from, tc.d, got, tc.want)
		}
	}
	if n, ok := c.Canonical("LAPTOP.local"); !ok || n != "laptop" {
		t.Fatalf("alias: %q %v", n, ok)
	}
	if v, _ := c.ScreenOption("laptop", "halfduplexcapslock"); v != "true" {
		t.Fatalf("screen option: %q", v)
	}
	if !c.Bool("relativeMouseMoves", false) || c.Bool("missing", false) {
		t.Fatal("options")
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"unknown link target": "section: screens\na:\nend\nsection: links\na:\nright = ghost\nend",
		"unknown link source": "section: screens\na:\nend\nsection: links\nghost:\nright = a\nend",
		"duplicate screen":    "section: screens\na:\nA:\nend",
		"bad edge":            "section: screens\na:\nb:\nend\nsection: links\na:\nsideways = b\nend",
		"self link":           "section: screens\na:\nend\nsection: links\na:\nleft = a\nend",
		"range":               "section: screens\na:\nb:\nend\nsection: links\na:\nright(0,50) = b\nend",
		"missing end":         "section: screens\na:\n",
		"unknown section":     "section: hotkeys\nend",
		"no section":          "a:",
		"no screens":          "section: options\nx = 1\nend",
		"alias unknown":       "section: screens\na:\nend\nsection: aliases\nghost:\nx\nend",
		"alias clash":         "section: screens\na:\nb:\nend\nsection: aliases\na:\nb\nend",
	}
	for name, cfg := range cases {
		if _, err := Parse(strings.NewReader(cfg)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestErrorMentionsLineAndScreen(t *testing.T) {
	_, err := Parse(strings.NewReader("section: screens\na:\nend\nsection: links\na:\nright = ghost\nend"))
	if err == nil || !strings.Contains(err.Error(), "line 6") || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("error not helpful: %v", err)
	}
}

func TestOpposite(t *testing.T) {
	for _, d := range []Direction{Left, Right, Up, Down} {
		if d.Opposite().Opposite() != d || d.Opposite() == d {
			t.Fatalf("%s", d)
		}
	}
}
