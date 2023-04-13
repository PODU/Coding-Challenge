package cut

import (
	"strings"
	"testing"
)

// process is a test helper: build a Cutter from cfg, run input through it, and
// return the produced output.
func process(t *testing.T, cfg Config, input string) string {
	t.Helper()
	c := NewCutter(cfg)
	var out strings.Builder
	if err := c.ProcessStream(strings.NewReader(input), &out); err != nil {
		t.Fatalf("ProcessStream error: %v", err)
	}
	return out.String()
}

func mustList(t *testing.T, spec string, complement bool) Selector {
	t.Helper()
	s, err := ParseList(spec, complement)
	if err != nil {
		t.Fatalf("ParseList(%q): %v", spec, err)
	}
	return s
}

func TestFields(t *testing.T) {
	tests := []struct {
		name      string
		list      string
		delim     byte
		outDelim  string
		outSet    bool
		onlyDelim bool
		comp      bool
		in        string
		want      string
	}{
		{name: "single field", list: "2", delim: '\t', in: "a\tb\tc\n", want: "b\n"},
		{name: "multi field", list: "1,3", delim: '\t', in: "a\tb\tc\n", want: "a\tc\n"},
		{name: "custom delim", list: "1", delim: ',', in: "a,b,c\n", want: "a\n"},
		{name: "output delim", list: "1,2", delim: ',', outDelim: ":", outSet: true, in: "a,b,c\n", want: "a:b\n"},
		{name: "missing field", list: "5", delim: ',', in: "a,b,c\n", want: "\n"},
		{name: "range to end", list: "2-", delim: ',', in: "a,b,c,d\n", want: "b,c,d\n"},
		{name: "no delim passthrough", list: "1", delim: ',', in: "abc\n", want: "abc\n"},
		{name: "no delim suppressed", list: "1", delim: ',', onlyDelim: true, in: "abc\nx,y\n", want: "x\n"},
		{name: "complement", list: "1", delim: ',', comp: true, in: "a,b,c\n", want: "b,c\n"},
		{name: "empty line", list: "1", delim: ',', in: "\n", want: "\n"},
		{name: "out of order dedup", list: "3,1,1", delim: ',', in: "a,b,c\n", want: "a,c\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Mode:        ModeFields,
				Sel:         mustList(t, tt.list, tt.comp),
				Delim:       tt.delim,
				OutDelim:    tt.outDelim,
				OutDelimSet: tt.outSet,
				OnlyDelim:   tt.onlyDelim,
			}
			if got := process(t, cfg, tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBytes(t *testing.T) {
	tests := []struct {
		name     string
		list     string
		comp     bool
		outDelim string
		outSet   bool
		in       string
		want     string
	}{
		{name: "single byte", list: "1", in: "abcdef\n", want: "a\n"},
		{name: "range", list: "2-4", in: "abcdef\n", want: "bcd\n"},
		{name: "open range", list: "3-", in: "abcdef\n", want: "cdef\n"},
		{name: "to-M", list: "-3", in: "abcdef\n", want: "abc\n"},
		{name: "list", list: "1,3-4", in: "abcdef\n", want: "acd\n"},
		{name: "complement", list: "1-2", comp: true, in: "abcdef\n", want: "cdef\n"},
		{name: "outdelim gaps", list: "1-2,4-5", outDelim: ":", outSet: true, in: "abcdef\n", want: "ab:de\n"},
		{name: "beyond line", list: "10", in: "abc\n", want: "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Mode:        ModeBytes,
				Sel:         mustList(t, tt.list, tt.comp),
				OutDelim:    tt.outDelim,
				OutDelimSet: tt.outSet,
			}
			if got := process(t, cfg, tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChars(t *testing.T) {
	tests := []struct {
		name string
		list string
		comp bool
		in   string
		want string
	}{
		{name: "ascii", list: "1-3", in: "abcdef\n", want: "abc\n"},
		{name: "multibyte cafe", list: "1-4", in: "café\n", want: "café\n"},
		{name: "multibyte slice", list: "4", in: "café\n", want: "é\n"},
		{name: "emoji", list: "2", in: "a😀b\n", want: "😀\n"},
		{name: "complement", list: "1", comp: true, in: "café\n", want: "afé\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Mode: ModeChars, Sel: mustList(t, tt.list, tt.comp)}
			if got := process(t, cfg, tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChars_InvalidUTF8(t *testing.T) {
	// 0xFF is not valid UTF-8; must not crash and must select position-wise.
	cfg := Config{Mode: ModeChars, Sel: mustList(t, "1-3", false)}
	got := process(t, cfg, "a\xffb\n")
	// Position 2 (0xff) decodes to the replacement rune.
	want := "a�b\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNewlinePreservation(t *testing.T) {
	cfg := Config{Mode: ModeBytes, Sel: mustList(t, "1-", false)}

	// Last line without trailing newline stays without one.
	if got := process(t, cfg, "abc"); got != "abc" {
		t.Errorf("no-newline last line: got %q, want %q", got, "abc")
	}
	// Trailing newline preserved.
	if got := process(t, cfg, "abc\n"); got != "abc\n" {
		t.Errorf("newline last line: got %q, want %q", got, "abc\n")
	}
	// Multiple lines, mixed.
	if got := process(t, cfg, "a\nb\nc"); got != "a\nb\nc" {
		t.Errorf("mixed: got %q, want %q", got, "a\nb\nc")
	}
}

func TestFields_SuppressNoTrailingNewlineEmitted(t *testing.T) {
	// With -s a suppressed no-delimiter line produces no output at all.
	cfg := Config{Mode: ModeFields, Sel: mustList(t, "1", false), Delim: ',', OnlyDelim: true}
	if got := process(t, cfg, "nodelim"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
