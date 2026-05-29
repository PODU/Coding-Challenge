package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// readerOnly hides any io.Seeker implementation so the engine takes the
// non-seekable (read-and-discard / write-zeros) code paths.
type readerOnly struct{ r io.Reader }

func (ro readerOnly) Read(p []byte) (int, error) { return ro.r.Read(p) }

func tempFile(t *testing.T, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	return p
}

func tempPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "out")
}

// ---------- size suffix parsing ----------

func TestParseSize(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"512", 512, true},
		{"0", 0, true},
		{"1c", 1, true},
		{"1w", 2, true},
		{"2b", 1024, true},
		{"1k", 1024, true},
		{"1K", 1024, true},
		{"1M", 1 << 20, true},
		{"1G", 1 << 30, true},
		{"2x512", 1024, true},
		{"2x3x4", 24, true},
		{"", 0, false},
		{"-5", 0, false},
		{"12q", 0, false},
		{"k", 0, false},
		{"9999999999999999999999", 0, false},
	}
	for _, c := range cases {
		got, err := parseSize(c.in)
		if c.ok {
			if err != nil {
				t.Errorf("parseSize(%q) unexpected error: %v", c.in, err)
			} else if got != c.want {
				t.Errorf("parseSize(%q) = %d, want %d", c.in, got, c.want)
			}
		} else if err == nil {
			t.Errorf("parseSize(%q) = %d, want error", c.in, got)
		}
	}
}

// ---------- operand parsing ----------

func TestParseOperandsPrecedence(t *testing.T) {
	cfg, err := ParseOperands([]string{"ibs=100", "obs=200", "bs=4k"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Ibs != 4096 || cfg.Obs != 4096 {
		t.Errorf("bs= should override ibs/obs: got ibs=%d obs=%d", cfg.Ibs, cfg.Obs)
	}
	if !cfg.BsMode {
		t.Error("bs= should set BsMode")
	}

	cfg, err = ParseOperands([]string{"ibs=3", "obs=5"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Ibs != 3 || cfg.Obs != 5 || cfg.BsMode {
		t.Errorf("ibs/obs independent: got ibs=%d obs=%d bsMode=%v", cfg.Ibs, cfg.Obs, cfg.BsMode)
	}
}

func TestParseOperandsErrors(t *testing.T) {
	bad := [][]string{
		{"-x"},
		{"frobnicate"},
		{"bogus=1"},
		{"bs=1", "bs=2"},
		{"bs=0"},
		{"bs=-4"},
		{"bs=3z"},
		{"conv=ucase,lcase"},
		{"conv=wat"},
		{"status=loud"},
		{"iflag=teleport"},
	}
	for _, args := range bad {
		if _, err := ParseOperands(args); err == nil {
			t.Errorf("ParseOperands(%v) = nil error, want error", args)
		}
	}
}

// ---------- block copy engine ----------

func runFile(t *testing.T, args []string) (*Stats, int, error) {
	t.Helper()
	cfg, err := ParseOperands(args)
	if err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	var stderr bytes.Buffer
	return Run(cfg, bytes.NewReader(nil), &bytes.Buffer{}, &stderr)
}

func TestIdentityCopy(t *testing.T) {
	data := []byte("the quick brown fox jumps over the lazy dog 0123456789")
	in := tempFile(t, data)
	out := tempPath(t)
	if _, code, err := runFile(t, []string{"if=" + in, "of=" + out}); err != nil || code != 0 {
		t.Fatalf("run: code=%d err=%v", code, err)
	}
	got, _ := os.ReadFile(out)
	if !bytes.Equal(got, data) {
		t.Errorf("identity copy mismatch: got %q want %q", got, data)
	}
}

func TestBsCount(t *testing.T) {
	in := tempFile(t, []byte("0123456789ABCDEF"))
	out := tempPath(t)
	if _, code, err := runFile(t, []string{"if=" + in, "of=" + out, "bs=4", "count=3"}); err != nil || code != 0 {
		t.Fatalf("run: %d %v", code, err)
	}
	got, _ := os.ReadFile(out)
	if !bytes.Equal(got, []byte("0123456789AB")) {
		t.Errorf("bs=4 count=3 got %q", got)
	}
}

func TestSkip(t *testing.T) {
	in := tempFile(t, []byte("0123456789"))
	out := tempPath(t)
	if _, code, err := runFile(t, []string{"if=" + in, "of=" + out, "bs=4", "skip=2"}); err != nil || code != 0 {
		t.Fatalf("run: %d %v", code, err)
	}
	got, _ := os.ReadFile(out)
	if !bytes.Equal(got, []byte("89")) {
		t.Errorf("bs=4 skip=2 got %q want %q", got, "89")
	}
}

func TestSeekPreservesLeading(t *testing.T) {
	out := tempPath(t)
	if err := os.WriteFile(out, bytes.Repeat([]byte("X"), 20), 0o644); err != nil {
		t.Fatal(err)
	}
	in := tempFile(t, []byte("ABCD"))
	if _, code, err := runFile(t, []string{"if=" + in, "of=" + out, "bs=4", "seek=2"}); err != nil || code != 0 {
		t.Fatalf("run: %d %v", code, err)
	}
	got, _ := os.ReadFile(out)
	want := []byte("XXXXXXXXABCD")
	if !bytes.Equal(got, want) {
		t.Errorf("seek got %q want %q", got, want)
	}
}

func TestSkipSeekCountCombined(t *testing.T) {
	// input: 24 bytes; ibs=obs=4; skip=2 (offset 8), count=3 -> bytes 8..19
	in := tempFile(t, []byte("0123456789ABCDEFGHIJKLMN"))
	out := tempPath(t)
	if err := os.WriteFile(out, bytes.Repeat([]byte("."), 16), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, code, err := runFile(t, []string{"if=" + in, "of=" + out, "bs=4", "skip=2", "seek=1", "count=3"}); err != nil || code != 0 {
		t.Fatalf("run: %d %v", code, err)
	}
	got, _ := os.ReadFile(out)
	// seek=1 -> first 4 bytes preserved; then bytes 8..19 = "89ABCDEFGHIJ"; truncated to 16
	want := []byte("...." + "89ABCDEFGHIJ")
	if !bytes.Equal(got, want) {
		t.Errorf("combined got %q want %q", got, want)
	}
}

func TestIbsNeObs(t *testing.T) {
	data := []byte("0123456789ABCDE") // 15 bytes
	in := tempFile(t, data)
	out := tempPath(t)
	stats, code, err := runFile(t, []string{"if=" + in, "of=" + out, "ibs=3", "obs=5"})
	if err != nil || code != 0 {
		t.Fatalf("run: %d %v", code, err)
	}
	got, _ := os.ReadFile(out)
	if !bytes.Equal(got, data) {
		t.Errorf("ibs!=obs mismatch got %q", got)
	}
	if stats.inFull.Load() != 5 || stats.inPartial.Load() != 0 {
		t.Errorf("records in = %d+%d, want 5+0", stats.inFull.Load(), stats.inPartial.Load())
	}
	if stats.outFull.Load() != 3 || stats.outPartial.Load() != 0 {
		t.Errorf("records out = %d+%d, want 3+0", stats.outFull.Load(), stats.outPartial.Load())
	}
}

func TestCountPastEOF(t *testing.T) {
	data := []byte("0123456789")
	in := tempFile(t, data)
	out := tempPath(t)
	stats, code, err := runFile(t, []string{"if=" + in, "of=" + out, "bs=4", "count=100"})
	if err != nil || code != 0 {
		t.Fatalf("run: %d %v", code, err)
	}
	got, _ := os.ReadFile(out)
	if !bytes.Equal(got, data) {
		t.Errorf("count past EOF mismatch got %q", got)
	}
	if stats.inFull.Load() != 2 || stats.inPartial.Load() != 1 {
		t.Errorf("records in = %d+%d, want 2+1", stats.inFull.Load(), stats.inPartial.Load())
	}
}

func TestNonSeekableSkip(t *testing.T) {
	cfg, _ := ParseOperands([]string{"bs=4", "skip=1"})
	in := readerOnly{bytes.NewReader([]byte("0123456789"))}
	var out bytes.Buffer
	if _, code, err := Run(cfg, in, &out, &bytes.Buffer{}); err != nil || code != 0 {
		t.Fatalf("run: %d %v", code, err)
	}
	if got := out.String(); got != "456789" {
		t.Errorf("non-seekable skip got %q want %q", got, "456789")
	}
}

// ---------- conversions ----------

func TestConvFunctions(t *testing.T) {
	b := []byte("Hello, 123!")
	ucase(b)
	if string(b) != "HELLO, 123!" {
		t.Errorf("ucase = %q", b)
	}
	b = []byte("Hello, 123!")
	lcase(b)
	if string(b) != "hello, 123!" {
		t.Errorf("lcase = %q", b)
	}
	b = []byte("ABCDE")
	swab(b)
	if string(b) != "BADCE" {
		t.Errorf("swab = %q want BADCE", b)
	}
	// swab round-trip on even data
	orig := []byte("ABCD")
	b = append([]byte(nil), orig...)
	swab(b)
	swab(b)
	if !bytes.Equal(b, orig) {
		t.Errorf("swab round-trip = %q", b)
	}
}

func TestConvUcaseFile(t *testing.T) {
	in := tempFile(t, []byte("Hello, World 99"))
	out := tempPath(t)
	if _, code, err := runFile(t, []string{"if=" + in, "of=" + out, "conv=ucase"}); err != nil || code != 0 {
		t.Fatalf("run: %d %v", code, err)
	}
	got, _ := os.ReadFile(out)
	if string(got) != "HELLO, WORLD 99" {
		t.Errorf("conv=ucase got %q", got)
	}
}

func TestConvSwabCombo(t *testing.T) {
	// swab then ucase on "abcd" -> "badc" -> "BADC"
	in := tempFile(t, []byte("abcd"))
	out := tempPath(t)
	if _, code, err := runFile(t, []string{"if=" + in, "of=" + out, "conv=swab,ucase"}); err != nil || code != 0 {
		t.Fatalf("run: %d %v", code, err)
	}
	got, _ := os.ReadFile(out)
	if string(got) != "BADC" {
		t.Errorf("conv=swab,ucase got %q want BADC", got)
	}
}

func TestConvNotrunc(t *testing.T) {
	out := tempPath(t)
	if err := os.WriteFile(out, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := tempFile(t, []byte("ABC"))
	if _, code, err := runFile(t, []string{"if=" + in, "of=" + out, "bs=3", "conv=notrunc"}); err != nil || code != 0 {
		t.Fatalf("run: %d %v", code, err)
	}
	got, _ := os.ReadFile(out)
	if string(got) != "ABC3456789" {
		t.Errorf("conv=notrunc got %q want ABC3456789", got)
	}
}

func TestConvSync(t *testing.T) {
	in := tempFile(t, []byte("ABCDEF")) // 6 bytes, ibs=4 -> "ABCD","EF"
	out := tempPath(t)
	stats, code, err := runFile(t, []string{"if=" + in, "of=" + out, "bs=4", "conv=sync"})
	if err != nil || code != 0 {
		t.Fatalf("run: %d %v", code, err)
	}
	got, _ := os.ReadFile(out)
	want := []byte("ABCDEF\x00\x00")
	if !bytes.Equal(got, want) {
		t.Errorf("conv=sync got %q want %q", got, want)
	}
	if len(got)%4 != 0 {
		t.Errorf("conv=sync output not block-aligned: len=%d", len(got))
	}
	if stats.outFull.Load() != 2 {
		t.Errorf("conv=sync outFull=%d want 2", stats.outFull.Load())
	}
}

func TestConvSparse(t *testing.T) {
	data := append(append([]byte("AAAA"), make([]byte, 4)...), []byte("BBBB")...) // 12 bytes, middle NUL block
	in := tempFile(t, data)
	out := tempPath(t)
	if _, code, err := runFile(t, []string{"if=" + in, "of=" + out, "bs=4", "conv=sparse"}); err != nil || code != 0 {
		t.Fatalf("run: %d %v", code, err)
	}
	got, _ := os.ReadFile(out)
	if !bytes.Equal(got, data) {
		t.Errorf("conv=sparse mismatch got %q want %q", got, data)
	}
}

// ---------- stats / status ----------

func TestRecordAccounting(t *testing.T) {
	// exact multiple of ibs -> N+0
	in := tempFile(t, bytes.Repeat([]byte("x"), 12))
	out := tempPath(t)
	stats, _, err := runFile(t, []string{"if=" + in, "of=" + out, "bs=4"})
	if err != nil {
		t.Fatal(err)
	}
	if stats.inFull.Load() != 3 || stats.inPartial.Load() != 0 {
		t.Errorf("exact multiple records in = %d+%d want 3+0", stats.inFull.Load(), stats.inPartial.Load())
	}
	if stats.bytes.Load() != 12 {
		t.Errorf("bytes copied = %d want 12", stats.bytes.Load())
	}

	// trailing partial -> N+1
	in = tempFile(t, bytes.Repeat([]byte("x"), 13))
	stats, _, err = runFile(t, []string{"if=" + in, "of=" + tempPath(t), "bs=4"})
	if err != nil {
		t.Fatal(err)
	}
	if stats.inFull.Load() != 3 || stats.inPartial.Load() != 1 {
		t.Errorf("partial records in = %d+%d want 3+1", stats.inFull.Load(), stats.inPartial.Load())
	}
}

func TestSummaryStatusLevels(t *testing.T) {
	s := NewStats()
	s.inFull.Store(2)
	s.inPartial.Store(1)
	s.outFull.Store(2)
	s.outPartial.Store(1)
	s.bytes.Store(100)

	if got := s.Summary(StatusNone); got != "" {
		t.Errorf("status=none summary = %q want empty", got)
	}

	noxfer := s.Summary(StatusNoxfer)
	if strings.Count(noxfer, "\n") != 2 || strings.Contains(noxfer, "bytes copied") {
		t.Errorf("status=noxfer summary = %q", noxfer)
	}
	if !strings.Contains(noxfer, "2+1 records in") || !strings.Contains(noxfer, "2+1 records out") {
		t.Errorf("status=noxfer missing record lines: %q", noxfer)
	}

	full := s.Summary(StatusDefault)
	if !strings.Contains(full, "2+1 records in") ||
		!strings.Contains(full, "2+1 records out") ||
		!strings.Contains(full, "100 bytes copied") {
		t.Errorf("default summary = %q", full)
	}
}

// slowReader yields one byte per Read after a short delay, so a ticker-driven
// progress goroutine has time to fire at least once.
type slowReader struct {
	data []byte
	pos  int
}

func (s *slowReader) Read(p []byte) (int, error) {
	if s.pos >= len(s.data) {
		return 0, io.EOF
	}
	time.Sleep(2 * time.Millisecond)
	p[0] = s.data[s.pos]
	s.pos++
	return 1, nil
}

func TestStatusProgress(t *testing.T) {
	old := progressInterval
	progressInterval = 1 * time.Millisecond
	defer func() { progressInterval = old }()

	cfg, _ := ParseOperands([]string{"bs=1", "status=progress"})
	in := &slowReader{data: bytes.Repeat([]byte("z"), 30)}
	var out, errBuf bytes.Buffer
	if _, code, err := Run(cfg, in, &out, &errBuf); err != nil || code != 0 {
		t.Fatalf("run: %d %v", code, err)
	}
	if out.Len() != 30 {
		t.Errorf("progress copy produced %d bytes want 30", out.Len())
	}
	if !strings.Contains(errBuf.String(), "bytes copied") {
		t.Errorf("status=progress emitted no progress line: %q", errBuf.String())
	}
}

// ---------- stdin / stdout ----------

func TestStdinStdoutUcase(t *testing.T) {
	cfg, _ := ParseOperands([]string{"conv=ucase"})
	in := readerOnly{strings.NewReader("hello")}
	var out, errBuf bytes.Buffer
	stats, code, err := Run(cfg, in, &out, &errBuf)
	if err != nil || code != 0 {
		t.Fatalf("run: %d %v", code, err)
	}
	if out.String() != "HELLO" {
		t.Errorf("stdin ucase got %q", out.String())
	}
	if stats.inPartial.Load() != 1 {
		t.Errorf("records in partial = %d want 1", stats.inPartial.Load())
	}
}

func TestStdinPassthroughBs1(t *testing.T) {
	cfg, _ := ParseOperands([]string{"bs=1"})
	payload := "byte-exact"
	in := readerOnly{strings.NewReader(payload)}
	var out bytes.Buffer
	stats, code, err := Run(cfg, in, &out, &bytes.Buffer{})
	if err != nil || code != 0 {
		t.Fatalf("run: %d %v", code, err)
	}
	if out.String() != payload {
		t.Errorf("bs=1 passthrough got %q", out.String())
	}
	if stats.inFull.Load() != int64(len(payload)) || stats.inPartial.Load() != 0 {
		t.Errorf("bs=1 records in = %d+%d", stats.inFull.Load(), stats.inPartial.Load())
	}
}

func TestEmptyInput(t *testing.T) {
	cfg, _ := ParseOperands(nil)
	in := readerOnly{strings.NewReader("")}
	var out bytes.Buffer
	stats, code, err := Run(cfg, in, &out, &bytes.Buffer{})
	if err != nil || code != 0 {
		t.Fatalf("run: %d %v", code, err)
	}
	if out.Len() != 0 {
		t.Errorf("empty input produced output %q", out.String())
	}
	if stats.inFull.Load() != 0 || stats.inPartial.Load() != 0 || stats.bytes.Load() != 0 {
		t.Errorf("empty input stats not zero: %d+%d, %d bytes",
			stats.inFull.Load(), stats.inPartial.Load(), stats.bytes.Load())
	}
	summary := stats.Summary(StatusDefault)
	if !strings.Contains(summary, "0+0 records in") || !strings.Contains(summary, "0 bytes copied") {
		t.Errorf("empty summary = %q", summary)
	}
}
