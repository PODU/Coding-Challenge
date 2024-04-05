package xxd

import (
	"bytes"
	"strings"
	"testing"
)

// f1 is the challenge's sample file content: "File 1 contents.\n" (17 bytes,
// forcing a partial second line at cols=16).
var f1 = []byte("File 1 contents.\n")

// allBytes is every byte value 0x00..0xff.
func allBytes() []byte {
	b := make([]byte, 256)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

func dumpString(t *testing.T, in []byte, mutate func(*Config)) string {
	t.Helper()
	cfg := NewConfig()
	if mutate != nil {
		mutate(&cfg)
	}
	var out bytes.Buffer
	if err := Dump(bytes.NewReader(in), &out, cfg); err != nil {
		t.Fatalf("Dump error: %v", err)
	}
	return out.String()
}

// TestDumpGolden asserts byte-for-byte parity with hand-verified real-xxd
// output for the supported flag set.
func TestDumpGolden(t *testing.T) {
	cases := []struct {
		name   string
		in     []byte
		mutate func(*Config)
		want   string
	}{
		{
			name: "default",
			in:   f1,
			want: "00000000: 4669 6c65 2031 2063 6f6e 7465 6e74 732e  File 1 contents.\n" +
				"00000010: 0a                                       .\n",
		},
		{
			name:   "plain",
			in:     f1,
			mutate: func(c *Config) { c.Plain = true; c.Cols = DefaultPlainCols },
			want:   "46696c65203120636f6e74656e74732e0a\n",
		},
		{
			name:   "little-endian default group4",
			in:     f1,
			mutate: func(c *Config) { c.LittleEndian = true; c.Group = 4 },
			want: "00000000: 656c6946 63203120 65746e6f 2e73746e  File 1 contents.\n" +
				"00000010:       0a                             .\n",
		},
		{
			name:   "little-endian group2",
			in:     f1,
			mutate: func(c *Config) { c.LittleEndian = true; c.Group = 2 },
			want: "00000000: 6946 656c 3120 6320 6e6f 6574 746e 2e73  File 1 contents.\n" +
				"00000010:   0a                                     .\n",
		},
		{
			name:   "cols 8",
			in:     f1,
			mutate: func(c *Config) { c.Cols = 8 },
			want: "00000000: 4669 6c65 2031 2063  File 1 c\n" +
				"00000008: 6f6e 7465 6e74 732e  ontents.\n" +
				"00000010: 0a                   .\n",
		},
		{
			name:   "group 1",
			in:     f1,
			mutate: func(c *Config) { c.Group = 1 },
			want: "00000000: 46 69 6c 65 20 31 20 63 6f 6e 74 65 6e 74 73 2e  File 1 contents.\n" +
				"00000010: 0a                                               .\n",
		},
		{
			name:   "group 0 (no grouping)",
			in:     f1,
			mutate: func(c *Config) { c.Group = 0 },
			want: "00000000: 46696c65203120636f6e74656e74732e  File 1 contents.\n" +
				"00000010: 0a                                .\n",
		},
		{
			name:   "length 10",
			in:     f1,
			mutate: func(c *Config) { c.Length = 10 },
			want:   "00000000: 4669 6c65 2031 2063 6f6e                 File 1 con\n",
		},
		{
			name:   "seek 4",
			in:     f1,
			mutate: func(c *Config) { c.Seek = 4 },
			want:   "00000004: 2031 2063 6f6e 7465 6e74 732e 0a          1 contents..\n",
		},
		{
			name:   "seek 4 length 6",
			in:     f1,
			mutate: func(c *Config) { c.Seek = 4; c.Length = 6 },
			want:   "00000004: 2031 2063 6f6e                            1 con\n",
		},
		{
			name:   "cols 8 group 4",
			in:     f1,
			mutate: func(c *Config) { c.Cols = 8; c.Group = 4 },
			want: "00000000: 46696c65 20312063  File 1 c\n" +
				"00000008: 6f6e7465 6e74732e  ontents.\n" +
				"00000010: 0a                 .\n",
		},
		{
			name:   "cols 1",
			in:     f1,
			mutate: func(c *Config) { c.Cols = 1; c.Length = 3 },
			want: "00000000: 46  F\n" +
				"00000001: 69  i\n" +
				"00000002: 6c  l\n",
		},
		{
			name:   "allbytes first two lines",
			in:     allBytes()[:32],
			mutate: nil,
			want: "00000000: 0001 0203 0405 0607 0809 0a0b 0c0d 0e0f  ................\n" +
				"00000010: 1011 1213 1415 1617 1819 1a1b 1c1d 1e1f  ................\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dumpString(t, tc.in, tc.mutate)
			if got != tc.want {
				t.Errorf("dump mismatch\n got:\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}

func TestEmptyInput(t *testing.T) {
	if got := dumpString(t, nil, nil); got != "" {
		t.Errorf("empty input: got %q, want empty", got)
	}
}

func TestLengthZero(t *testing.T) {
	got := dumpString(t, f1, func(c *Config) { c.Length = 0 })
	if got != "" {
		t.Errorf("-l 0: got %q, want empty", got)
	}
}

func TestSeekPastEOF(t *testing.T) {
	got := dumpString(t, f1, func(c *Config) { c.Seek = 1000 })
	if got != "" {
		t.Errorf("seek past EOF: got %q, want empty", got)
	}
}

func TestPartialLastLineAlignment(t *testing.T) {
	// 17 bytes => second line has a single byte; the ASCII gutter must start at
	// the same column as a full line.
	out := dumpString(t, f1, nil)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d", len(lines))
	}
	g0 := strings.Index(lines[0], "  File")
	g1 := strings.Index(lines[1], "  .")
	if g0 != g1 {
		t.Errorf("gutter misaligned: full line at %d, partial at %d", g0, g1)
	}
}

func TestEndianSwap(t *testing.T) {
	// A clean 4-byte little-endian word: 0x01020304 stored little-endian.
	in := []byte{0x04, 0x03, 0x02, 0x01}
	got := dumpString(t, in, func(c *Config) { c.LittleEndian = true; c.Group = 4 })
	want := "00000000: 01020304                             ....\n"
	if got != want {
		t.Errorf("endian swap:\n got: %q\nwant: %q", got, want)
	}
}

func TestASCIIGutterPrintable(t *testing.T) {
	in := []byte{0x00, 0x09, 0x20, 0x41, 0x7e, 0x7f, 0xff}
	got := dumpString(t, in, func(c *Config) { c.Group = 1 })
	// printable: 0x20 ' ', 0x41 'A', 0x7e '~'; others '.'
	if !strings.HasSuffix(strings.TrimRight(got, "\n"), "  .. A~..") {
		t.Errorf("gutter mismatch: %q", got)
	}
}

func TestOffsetWidthWidening(t *testing.T) {
	if w := offsetWidth(0xff); w != 8 {
		t.Errorf("small offset width = %d, want 8", w)
	}
	if w := offsetWidth(0x1_0000_0000); w != 9 {
		t.Errorf("5-byte offset width = %d, want 9", w)
	}
}

func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"default ok", NewConfig(), false},
		{"zero cols", Config{Cols: 0, Length: -1}, true},
		{"negative group", Config{Cols: 16, Group: -1, Length: -1}, true},
		{"negative seek", Config{Cols: 16, Group: 2, Length: -1, Seek: -1}, true},
		{"-e bad group", Config{Cols: 16, Group: 3, Length: -1, LittleEndian: true}, true},
		{"-e good group", Config{Cols: 16, Group: 8, Length: -1, LittleEndian: true}, false},
		{"-e with -p", Config{Cols: 16, Group: 4, Length: -1, LittleEndian: true, Plain: true}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}
