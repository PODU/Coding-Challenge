package cli

import (
	"errors"
	"testing"

	"ccwc/internal/counter"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		want      Options
		wantHelp  bool
		wantUsage bool
	}{
		{"no args", nil, Options{}, false, false},
		{"single c", []string{"-c", "f.txt"}, Options{Bytes: true, Files: []string{"f.txt"}}, false, false},
		{"single l", []string{"-l"}, Options{Lines: true}, false, false},
		{"all separate", []string{"-l", "-w", "-m", "-c"}, Options{Lines: true, Words: true, Chars: true, Bytes: true}, false, false},
		// Bundled flags expand to multiple flags.
		{"bundled lw", []string{"-lw", "f"}, Options{Lines: true, Words: true, Files: []string{"f"}}, false, false},
		{"bundled cl order invariant", []string{"-cl"}, Options{Bytes: true, Lines: true}, false, false},
		{"bundled wl", []string{"-wl"}, Options{Words: true, Lines: true}, false, false},
		// "-" is a filename (stdin), not a flag.
		{"dash file", []string{"-l", "-"}, Options{Lines: true, Files: []string{"-"}}, false, false},
		// "--" stops option parsing.
		{"double dash", []string{"--", "-weird-name"}, Options{Files: []string{"-weird-name"}}, false, false},
		{"multiple files", []string{"a", "b", "c"}, Options{Files: []string{"a", "b", "c"}}, false, false},
		// Help.
		{"help short", []string{"-h"}, Options{}, true, false},
		{"help long", []string{"--help"}, Options{}, true, false},
		// Unknown flag is a usage error.
		{"unknown flag", []string{"-x"}, Options{}, false, true},
		{"unknown in bundle", []string{"-lz"}, Options{}, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.args)
			switch {
			case tt.wantHelp:
				if !errors.Is(err, ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
				return
			case tt.wantUsage:
				var ue *UsageError
				if !errors.As(err, &ue) {
					t.Fatalf("expected UsageError, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !optionsEqual(got, tt.want) {
				t.Errorf("Parse(%v) = %+v, want %+v", tt.args, got, tt.want)
			}
		})
	}
}

func optionsEqual(a, b Options) bool {
	if a.Lines != b.Lines || a.Words != b.Words || a.Chars != b.Chars || a.Bytes != b.Bytes {
		return false
	}
	if len(a.Files) != len(b.Files) {
		return false
	}
	for i := range a.Files {
		if a.Files[i] != b.Files[i] {
			return false
		}
	}
	return true
}

func TestApplyDefaults(t *testing.T) {
	o := Options{}
	o.ApplyDefaults()
	if !(o.Lines && o.Words && o.Bytes) || o.Chars {
		t.Errorf("default should be lines+words+bytes, got %+v", o)
	}

	o2 := Options{Chars: true}
	o2.ApplyDefaults()
	if o2.Lines || o2.Words || o2.Bytes {
		t.Errorf("explicit flag should disable defaults, got %+v", o2)
	}
}

func TestDigits(t *testing.T) {
	cases := map[int64]int{0: 1, 1: 1, 9: 1, 10: 2, 99: 2, 100: 3, 298: 3, 342190: 6}
	for n, want := range cases {
		if got := digits(n); got != want {
			t.Errorf("digits(%d) = %d, want %d", n, got, want)
		}
	}
}

func TestFieldWidth(t *testing.T) {
	tests := []struct {
		name       string
		selected   int
		numInputs  int
		sizesKnown bool
		totalBytes int64
		want       int
	}{
		// Single count + single file -> width 1 (no padding).
		{"single count single file", 1, 1, true, 342190, 1},
		// Multiple counts, single regular file -> digits of total bytes.
		{"default single file", 3, 1, true, 298, 3},
		// Single count, multiple regular files -> byte-digit width.
		{"single count multi file", 1, 2, true, 26, 2},
		// stdin with multiple counts -> default width 7.
		{"stdin multi count", 2, 1, false, 0, 7},
		// stdin single count -> width 1.
		{"stdin single count", 1, 1, false, 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FieldWidth(tt.selected, tt.numInputs, tt.sizesKnown, tt.totalBytes); got != tt.want {
				t.Errorf("FieldWidth = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFormatRow(t *testing.T) {
	tests := []struct {
		name  string
		opts  Options
		c     counter.Counts
		label string
		width int
		want  string
	}{
		// Mirrors the article's default test.txt output (BSD width 7).
		{
			"default columns width7",
			Options{Lines: true, Words: true, Bytes: true},
			counter.Counts{Lines: 7145, Words: 58164, Bytes: 342190},
			"test.txt", 7,
			"   7145   58164  342190 test.txt",
		},
		// GNU single-count single-file: no padding, no leading space.
		{
			"single count no pad",
			Options{Bytes: true},
			counter.Counts{Bytes: 24},
			"a.txt", 1,
			"24 a.txt",
		},
		// GNU default single file (width from byte digits).
		{
			"gnu default single",
			Options{Lines: true, Words: true, Bytes: true},
			counter.Counts{Lines: 149, Words: 0, Bytes: 298},
			"many.txt", 3,
			"149   0 298 many.txt",
		},
		// Canonical column order is lines, words, chars, bytes regardless of
		// the order flags were typed.
		{
			"canonical order all",
			Options{Lines: true, Words: true, Chars: true, Bytes: true},
			counter.Counts{Lines: 1, Words: 2, Chars: 3, Bytes: 4},
			"f", 1,
			"1 2 3 4 f",
		},
		// Stdin: empty label omits the filename column.
		{
			"stdin no label",
			Options{Lines: true},
			counter.Counts{Lines: 3},
			"", 1,
			"3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatRow(tt.opts, tt.c, tt.label, tt.width); got != tt.want {
				t.Errorf("FormatRow = %q, want %q", got, tt.want)
			}
		})
	}
}
