package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// naiveLastLines is the obviously-correct reference: split and take the end.
func naiveLastLines(data []byte, n int) []byte {
	if n == 0 || len(data) == 0 {
		return nil
	}
	lines := bytes.SplitAfter(data, []byte("\n"))
	if len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	if n > len(lines) {
		n = len(lines)
	}
	return bytes.Join(lines[len(lines)-n:], nil)
}

func randomText(r *rand.Rand, size int) []byte {
	b := make([]byte, size)
	for i := range b {
		if r.Intn(8) == 0 {
			b[i] = '\n'
		} else {
			b[i] = 'a' + byte(r.Intn(26))
		}
	}
	return b
}

func TestLastLinesMatchesReference(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	// Sizes straddle chunk boundaries to exercise the backward scan.
	for _, size := range []int{0, 1, 2, 7, 100, chunkSize - 1, chunkSize, chunkSize + 1, 3*chunkSize + 17} {
		data := randomText(r, size)
		for _, n := range []int{0, 1, 2, 5, 10, 1000, 100000} {
			want := naiveLastLines(data, n)

			start, err := lastLinesStart(bytes.NewReader(data), int64(len(data)), int64(n))
			if err != nil {
				t.Fatal(err)
			}
			if got := data[start:]; !bytes.Equal(got, want) {
				t.Errorf("seek size=%d n=%d: got %d bytes, want %d", size, n, len(got), len(want))
			}

			var buf bytes.Buffer
			total, err := streamLastLines(bytes.NewReader(data), int64(n), &buf)
			if err != nil || total != int64(len(data)) {
				t.Fatalf("stream size=%d n=%d: total=%d err=%v", size, n, total, err)
			}
			if !bytes.Equal(buf.Bytes(), want) {
				t.Errorf("stream size=%d n=%d: got %d bytes, want %d", size, n, buf.Len(), len(want))
			}
		}
	}
}

func TestStreamLastBytes(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	data := randomText(r, 5*chunkSize+3)
	for _, n := range []int{0, 1, 4, chunkSize, 2*chunkSize + 5, len(data), len(data) + 10} {
		var buf bytes.Buffer
		if _, err := streamLastBytes(bytes.NewReader(data), int64(n), &buf); err != nil {
			t.Fatal(err)
		}
		want := data[max(0, len(data)-n):]
		if !bytes.Equal(buf.Bytes(), want) {
			t.Errorf("n=%d: got %d bytes, want %d", n, buf.Len(), len(want))
		}
	}
}

func TestFromLineAndByte(t *testing.T) {
	data := []byte("1\n2\n3\n4\n5\n")
	cases := []struct {
		k    int64
		want string
	}{{0, "1\n2\n3\n4\n5\n"}, {1, "1\n2\n3\n4\n5\n"}, {3, "3\n4\n5\n"}, {5, "5\n"}, {6, ""}, {99, ""}}
	for _, c := range cases {
		var buf bytes.Buffer
		total, err := fromLine(bytes.NewReader(data), c.k, &buf)
		if err != nil || buf.String() != c.want || total != int64(len(data)) {
			t.Errorf("fromLine(%d) = %q, %d, %v; want %q", c.k, buf.String(), total, err, c.want)
		}
	}
	var buf bytes.Buffer
	if _, err := fromByte(strings.NewReader("abcdefghij"), 4, &buf); err != nil || buf.String() != "defghij" {
		t.Errorf("fromByte(4) = %q, %v", buf.String(), err)
	}
	buf.Reset()
	if _, err := fromByte(strings.NewReader("abc"), 10, &buf); err != nil || buf.String() != "" {
		t.Errorf("fromByte past end = %q, %v", buf.String(), err)
	}
}

func TestParseArgs(t *testing.T) {
	cases := []struct {
		args  []string
		check func(*config) bool
	}{
		{[]string{"f"}, func(c *config) bool { return c.count == 10 && c.unit == unitLines && c.files[0] == "f" }},
		{[]string{}, func(c *config) bool { return len(c.files) == 1 && c.files[0] == "-" }},
		{[]string{"-n", "5", "f"}, func(c *config) bool { return c.count == 5 && !c.fromStart }},
		{[]string{"-n+5"}, func(c *config) bool { return c.count == 5 && c.fromStart }},
		{[]string{"--lines=+7"}, func(c *config) bool { return c.count == 7 && c.fromStart }},
		{[]string{"-c", "4"}, func(c *config) bool { return c.count == 4 && c.unit == unitBytes }},
		{[]string{"--bytes", "2K"}, func(c *config) bool { return c.count == 2048 && c.unit == unitBytes }},
		{[]string{"-c", "1kB"}, func(c *config) bool { return c.count == 1000 }},
		{[]string{"-7"}, func(c *config) bool { return c.count == 7 && c.unit == unitLines }},
		{[]string{"-fqn", "3"}, func(c *config) bool { return c.follow && c.headers == headerNever && c.count == 3 }},
		{[]string{"-F"}, func(c *config) bool { return c.follow && c.followName && c.retry }},
		{[]string{"--follow=name"}, func(c *config) bool { return c.follow && c.followName && !c.retry }},
		{[]string{"-v", "-s", "0.5"}, func(c *config) bool { return c.headers == headerAlways && c.interval.Milliseconds() == 500 }},
		{[]string{"--pid=42", "--poll"}, func(c *config) bool { return c.pid == 42 && c.poll }},
		{[]string{"--", "-n"}, func(c *config) bool { return c.files[0] == "-n" }},
	}
	for _, c := range cases {
		cfg, err := parseArgs(c.args)
		if err != nil || !c.check(cfg) {
			t.Errorf("parseArgs(%q) = %+v, %v", c.args, cfg, err)
		}
	}
	for _, bad := range [][]string{{"-n", "x"}, {"-n"}, {"-z"}, {"--nope"}, {"--follow=bogus"}, {"-s", "-1"}, {"--pid=0"}} {
		if _, err := parseArgs(bad); err == nil {
			t.Errorf("parseArgs(%q) succeeded, want error", bad)
		}
	}
}

// runTail runs cctail over args (relative to dir) and returns stdout, stderr
// and the exit status.
func runTail(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()
	cfg, err := parseArgs(args)
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range cfg.files {
		if f != "-" {
			cfg.files[i] = filepath.Join(dir, f)
		}
	}
	var out, errb bytes.Buffer
	code := run(t.Context(), cfg, nil, &out, &errb)
	return strings.ReplaceAll(out.String(), dir+string(filepath.Separator), ""),
		strings.ReplaceAll(errb.String(), dir+string(filepath.Separator), ""), code
}

func seqText(from, to int) string {
	var sb strings.Builder
	for i := from; i <= to; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	return sb.String()
}

func writeFile(t *testing.T, path, s string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunChallengeSteps(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.log"), seqText(1, 15))
	writeFile(t, filepath.Join(dir, "test2.log"), seqText(100, 115))
	writeFile(t, filepath.Join(dir, "bytes.log"), "abcdefghij")

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"step1 default", []string{"test.log"}, seqText(6, 15)},
		{"step2 -n 5", []string{"-n", "5", "test.log"}, seqText(11, 15)},
		{"step2 -n 20", []string{"-n", "20", "test.log"}, seqText(1, 15)},
		{"step3 -n +5", []string{"-n", "+5", "test.log"}, seqText(5, 15)},
		{"step4 -c 4", []string{"-c", "4", "bytes.log"}, "ghij"},
		{"step7 multi", []string{"-n", "2", "test.log", "test2.log"},
			"==> test.log <==\n14\n15\n\n==> test2.log <==\n114\n115\n"},
		{"step13 quiet", []string{"-q", "-n", "1", "test.log", "test2.log"}, "15\n115\n"},
	}
	for _, c := range cases {
		out, errs, code := runTail(t, dir, c.args...)
		if out != c.want || code != 0 || errs != "" {
			t.Errorf("%s: got %q (stderr %q, code %d), want %q", c.name, out, errs, code, c.want)
		}
	}

	out, errs, code := runTail(t, dir, "-n", "1", "nope.log", "test.log")
	if code != 1 || !strings.Contains(errs, "cannot open 'nope.log' for reading: No such file or directory") ||
		out != "==> test.log <==\n15\n" {
		t.Errorf("missing file: out=%q err=%q code=%d", out, errs, code)
	}
}

func TestBigFileIsSeekedNotScanned(t *testing.T) {
	// A ~6 MB file through a reader that counts bytes read: tailing 10 lines
	// must touch only the final chunk.
	data := []byte(seqText(1, 800000))
	cr := &countingReaderAt{r: bytes.NewReader(data)}
	start, err := lastLinesStart(cr, int64(len(data)), 10)
	if err != nil {
		t.Fatal(err)
	}
	if string(data[start:]) != seqText(799991, 800000) {
		t.Fatalf("wrong lines: %q", data[start:])
	}
	if cr.n > chunkSize {
		t.Errorf("read %d bytes to find 10 lines; want <= %d", cr.n, chunkSize)
	}
}

type countingReaderAt struct {
	r *bytes.Reader
	n int
}

func (c *countingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	n, err := c.r.ReadAt(p, off)
	c.n += n
	return n, err
}

func TestFollowIgnoredForPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.WriteString(seqText(1, 3))
	w.Close()
	cfg, _ := parseArgs([]string{"-f", "-n", "2"})
	var out, errb bytes.Buffer
	code := run(t.Context(), cfg, r, &out, &errb)
	if code != 0 || out.String() != "2\n3\n" || errb.Len() != 0 {
		t.Errorf("got %q, stderr %q, code %d", out.String(), errb.String(), code)
	}
}
