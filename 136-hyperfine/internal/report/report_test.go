package report

import (
	"bytes"
	"strings"
	"testing"

	"cchyperfine/internal/bench"
	"cchyperfine/internal/stats"
)

func result(name string, times ...float64) *bench.Result {
	r := &bench.Result{Job: bench.Job{Command: name, Name: name}, Times: times}
	for range times {
		r.ExitCodes = append(r.ExitCodes, 0)
	}
	r.Wall = stats.Summarize(times)
	return r
}

func TestUnitFor(t *testing.T) {
	cases := []struct {
		in   float64
		want Unit
	}{
		{2.5, Second},
		{1.0, Second},
		{0.3, Millisecond},
		{0.001, Millisecond},
		{0.0009, Microsecond},
	}
	for _, c := range cases {
		if got := UnitFor(c.in); got != c.want {
			t.Errorf("UnitFor(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestUnitFormatPrecision(t *testing.T) {
	if got := Second.Format(1.23456); got != "1.235" {
		t.Errorf("seconds format = %q, want \"1.235\"", got)
	}
	if got := Millisecond.Format(0.3047); got != "304.7" {
		t.Errorf("millisecond format = %q, want \"304.7\"", got)
	}
	if got := Microsecond.Format(0.0000123); got != "12.3" {
		t.Errorf("microsecond format = %q, want \"12.3\"", got)
	}
}

func TestParseUnit(t *testing.T) {
	if u, ok := ParseUnit("microsecond"); !ok || u != Microsecond {
		t.Errorf("ParseUnit(microsecond) = %v %v", u, ok)
	}
	if _, ok := ParseUnit(""); ok {
		t.Error("an empty unit should not be treated as forced")
	}
}

func TestPadKeepsOversizedValuesSeparated(t *testing.T) {
	if got := Pad("1.2", 8); got != "     1.2" {
		t.Errorf("Pad = %q", got)
	}
	if got := Pad("1234567890", 8); got != " 1234567890" {
		t.Errorf("oversized Pad = %q, want a leading space", got)
	}
}

func TestCompareUsesFastestAsBaseline(t *testing.T) {
	fast := result("fast", 1.0, 1.0, 1.0)
	slow := result("slow", 2.0, 2.0, 2.0)
	baseline, others := Compare([]*bench.Result{slow, fast}, "")
	if baseline != fast {
		t.Fatalf("baseline = %v, want the fastest command", baseline.Name)
	}
	if len(others) != 1 || others[0].Result != slow {
		t.Fatalf("others = %+v", others)
	}
	if got := others[0].Ratio; got < 1.999 || got > 2.001 {
		t.Errorf("ratio = %v, want 2", got)
	}
}

func TestCompareHonoursExplicitReference(t *testing.T) {
	fast := result("fast", 1.0, 1.0)
	slow := result("slow", 2.0, 2.0)
	baseline, others := Compare([]*bench.Result{fast, slow}, "slow")
	if baseline != slow {
		t.Fatalf("baseline = %v, want the explicit reference", baseline.Name)
	}
	if got := others[0].Ratio; got < 0.499 || got > 0.501 {
		t.Errorf("ratio relative to the reference = %v, want 0.5", got)
	}
}

func TestSortResults(t *testing.T) {
	b := result("b", 1.0)
	a := result("a", 2.0)
	in := []*bench.Result{b, a}

	if got := SortResults(in, "auto"); got[0] != b {
		t.Error("auto should preserve the given order")
	}
	if got := SortResults(in, "command"); got[0] != a {
		t.Error("command sort should order by name")
	}
	if got := SortResults(in, "mean-time"); got[0] != b {
		t.Error("mean-time sort should put the fastest first")
	}
	if in[0] != b {
		t.Error("SortResults must not reorder its argument")
	}
}

func TestResultOutputShape(t *testing.T) {
	var buf bytes.Buffer
	r := &Reporter{Out: &buf, Style: Style{Unicode: true}}
	res := result("sleep 0.3", 0.3026, 0.3047, 0.3084)
	r.Header(0, res.Name)
	r.Result(res)

	out := buf.String()
	for _, want := range []string{"Benchmark 1: sleep 0.3", "Time (mean ± σ):", "Range (min … max):", "3 runs", "ms"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("colour escapes emitted despite Color being false")
	}
}

func TestSingleRunUsesAbsoluteLabel(t *testing.T) {
	var buf bytes.Buffer
	r := &Reporter{Out: &buf, Style: Style{Unicode: true}}
	r.Result(result("cmd", 0.5))
	out := buf.String()
	if !strings.Contains(out, "Time (abs ≡):") {
		t.Errorf("single run should use the absolute-time label:\n%s", out)
	}
	if strings.Contains(out, "Range") {
		t.Errorf("single run should not print a range:\n%s", out)
	}
}

func TestBasicStyleAvoidsUnicode(t *testing.T) {
	var buf bytes.Buffer
	r := &Reporter{Out: &buf, Style: Style{}}
	r.Result(result("cmd", 0.5, 0.6))
	out := buf.String()
	for _, bad := range []string{"±", "σ", "…"} {
		if strings.Contains(out, bad) {
			t.Errorf("ASCII output contains %q:\n%s", bad, out)
		}
	}
}

func TestSummaryNeedsTwoResults(t *testing.T) {
	var buf bytes.Buffer
	r := &Reporter{Out: &buf, Style: Style{}}
	r.Summary([]*bench.Result{result("only", 1.0)}, "")
	if buf.Len() != 0 {
		t.Errorf("a single benchmark should produce no summary, got:\n%s", buf.String())
	}
}

func TestSummaryLines(t *testing.T) {
	var buf bytes.Buffer
	r := &Reporter{Out: &buf, Style: Style{Unicode: true}}
	r.Summary([]*bench.Result{result("slow", 2.0, 2.0), result("fast", 1.0, 1.0)}, "")
	out := buf.String()
	if !strings.Contains(out, "fast ran") {
		t.Errorf("summary should name the baseline:\n%s", out)
	}
	if !strings.Contains(out, "2.00") || !strings.Contains(out, "times faster than slow") {
		t.Errorf("summary missing the comparison line:\n%s", out)
	}
}

func TestColorStyleEmitsEscapes(t *testing.T) {
	var buf bytes.Buffer
	r := &Reporter{Out: &buf, Style: Style{Color: true, Unicode: true}}
	r.Result(result("cmd", 0.5, 0.6))
	if !strings.Contains(buf.String(), "\x1b[") {
		t.Error("colour style emitted no escape sequences")
	}
}

func TestStyleFor(t *testing.T) {
	if s := StyleFor("none", nil); s.Color || s.Unicode || s.Progress {
		t.Errorf("style none = %+v, want everything disabled", s)
	}
	if s := StyleFor("full", nil); !s.Color || !s.Unicode || !s.Progress {
		t.Errorf("style full = %+v, want everything enabled", s)
	}
	if s := StyleFor("auto", nil); s.Progress {
		t.Errorf("auto with no terminal should not draw a progress bar: %+v", s)
	}
}

func TestBarIsInertWhenDisabled(t *testing.T) {
	var buf bytes.Buffer
	b := NewBar(&buf, Style{})
	b.Start(10)
	b.Advance(5, 10, 1)
	b.Finish()
	if buf.Len() != 0 {
		t.Errorf("disabled bar wrote %q", buf.String())
	}
}

func TestBarDrawsAndErases(t *testing.T) {
	var buf bytes.Buffer
	b := NewBar(&buf, Style{Progress: true, Unicode: false})
	b.Start(4)
	b.Advance(2, 4, 2)
	drawn := buf.String()
	if !strings.Contains(drawn, "2/4") {
		t.Errorf("bar did not show progress: %q", drawn)
	}
	b.Finish()
	if !strings.HasSuffix(buf.String(), "\r") {
		t.Error("Finish should erase the bar and return the cursor")
	}
}

func TestShortDuration(t *testing.T) {
	cases := map[float64]string{
		5.25:   "5.2s",
		125:    "2m05s",
		3725:   "1h02m",
		0:      "0.0s",
		59.94:  "59.9s",
		3599.9: "59m59s",
	}
	for in, want := range cases {
		if got := shortDuration(in); got != want {
			t.Errorf("shortDuration(%v) = %q, want %q", in, got, want)
		}
	}
}
