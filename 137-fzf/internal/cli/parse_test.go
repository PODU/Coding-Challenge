package cli

import (
	"reflect"
	"testing"

	"ccfzf/internal/item"
	"ccfzf/internal/pattern"
)

func mustParse(t *testing.T, args ...string) *Options {
	t.Helper()
	opts, err := Parse(args)
	if err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	return opts
}

func TestDefaults(t *testing.T) {
	o := mustParse(t)
	if !o.Extended || o.Exact || o.AlgoV1 || !o.Sort || o.Tac || o.Multi != 0 {
		t.Errorf("unexpected defaults: %+v", o)
	}
	if o.Case != pattern.CaseSmart {
		t.Error("default case mode should be smart")
	}
	if !reflect.DeepEqual(o.Tiebreak, []Criterion{ByLength}) {
		t.Errorf("default tiebreak %v", o.Tiebreak)
	}
}

func TestCaseFlags(t *testing.T) {
	if mustParse(t, "-i").Case != pattern.CaseIgnore {
		t.Error("-i")
	}
	if mustParse(t, "--ignore-case").Case != pattern.CaseIgnore {
		t.Error("--ignore-case")
	}
	if mustParse(t, "+i").Case != pattern.CaseRespect {
		t.Error("+i")
	}
	if mustParse(t, "--no-ignore-case").Case != pattern.CaseRespect {
		t.Error("--no-ignore-case")
	}
	// Later options win, which is what makes FZF_DEFAULT_OPTS overridable.
	if mustParse(t, "-i", "+i").Case != pattern.CaseRespect {
		t.Error("last flag should win")
	}
}

func TestQueryForms(t *testing.T) {
	for _, args := range [][]string{
		{"-q", "foo"},
		{"-qfoo"},
		{"--query", "foo"},
		{"--query=foo"},
	} {
		if got := mustParse(t, args...).Query; got != "foo" {
			t.Errorf("%v: query = %q", args, got)
		}
	}
}

func TestQueryWithLeadingDash(t *testing.T) {
	if got := mustParse(t, "--query=-x").Query; got != "-x" {
		t.Errorf("query = %q", got)
	}
}

func TestExtendedToggles(t *testing.T) {
	if mustParse(t, "+x").Extended {
		t.Error("+x should disable extended search")
	}
	if !mustParse(t, "+x", "-x").Extended {
		t.Error("-x should re-enable it")
	}
	if !mustParse(t, "-e").Exact {
		t.Error("-e")
	}
}

func TestAlgo(t *testing.T) {
	if !mustParse(t, "--algo=v1").AlgoV1 {
		t.Error("--algo=v1")
	}
	if mustParse(t, "--algo", "v2").AlgoV1 {
		t.Error("--algo v2")
	}
	if _, err := Parse([]string{"--algo=v3"}); err == nil {
		t.Error("v3 should be rejected")
	}
}

func TestTiebreak(t *testing.T) {
	o := mustParse(t, "--tiebreak=begin,index")
	if !reflect.DeepEqual(o.Tiebreak, []Criterion{ByBegin, ByIndex}) {
		t.Errorf("got %v", o.Tiebreak)
	}
	for _, bad := range []string{"--tiebreak=nope", "--tiebreak=begin,begin", "--tiebreak="} {
		if _, err := Parse([]string{bad}); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

// --tiebreak=end means the last occurrence should win, so the greedy scan has
// to run backwards.
func TestTiebreakEndFlipsScanDirection(t *testing.T) {
	if mustParse(t, "--tiebreak=end").PatternOptions().Forward {
		t.Error("--tiebreak=end should scan backwards")
	}
	if !mustParse(t).PatternOptions().Forward {
		t.Error("default should scan forwards")
	}
}

func TestMulti(t *testing.T) {
	if mustParse(t, "-m").Multi != -1 {
		t.Error("-m should be unlimited")
	}
	if mustParse(t, "--multi=3").Multi != 3 {
		t.Error("--multi=3")
	}
	if mustParse(t, "-m", "+m").Multi != 0 {
		t.Error("+m should turn it off")
	}
	if _, err := Parse([]string{"--multi=0"}); err == nil {
		t.Error("--multi=0 should be rejected")
	}
}

func TestNthAndDelimiter(t *testing.T) {
	o := mustParse(t, "-d", ":", "-n", "2..3", "--with-nth=1")
	if !reflect.DeepEqual(o.Nth, []item.Range{{Begin: 2, End: 3}}) {
		t.Errorf("nth %v", o.Nth)
	}
	if !reflect.DeepEqual(o.WithNth, []item.Range{{Begin: 1, End: 1}}) {
		t.Errorf("with-nth %v", o.WithNth)
	}
	tokens := item.Tokenize("a:b:c", o.Delim)
	if len(tokens) != 3 {
		t.Errorf("delimiter did not take effect: %q", tokens)
	}
	if _, err := Parse([]string{"-n", "0"}); err == nil {
		t.Error("field 0 should be rejected")
	}
}

func TestHeight(t *testing.T) {
	cases := map[string]Height{
		"40%":  {Set: true, Size: 40, Percent: true},
		"20":   {Set: true, Size: 20},
		"~50%": {Set: true, Size: 50, Percent: true, Auto: true},
		"200%": {Set: true, Size: 100, Percent: true},
	}
	for spec, want := range cases {
		got := mustParse(t, "--height="+spec).Height
		if got != want {
			t.Errorf("--height=%s: got %+v, want %+v", spec, got, want)
		}
	}
	for _, bad := range []string{"--height=0", "--height=x", "--height=-5"} {
		if _, err := Parse([]string{bad}); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestLayoutAndInfo(t *testing.T) {
	if mustParse(t, "--reverse").Layout != LayoutReverse {
		t.Error("--reverse")
	}
	if mustParse(t, "--layout=reverse-list").Layout != LayoutReverseList {
		t.Error("--layout=reverse-list")
	}
	if mustParse(t, "--info=inline").Info != InfoInline {
		t.Error("--info=inline")
	}
	if mustParse(t, "--info=hidden").Info != InfoHidden {
		t.Error("--info=hidden")
	}
	if _, err := Parse([]string{"--layout=sideways"}); err == nil {
		t.Error("bad layout should be rejected")
	}
}

func TestPreviewWindow(t *testing.T) {
	o := mustParse(t, "--preview", "cat {}", "--preview-window=up,30%")
	if o.Preview != "cat {}" {
		t.Errorf("preview %q", o.Preview)
	}
	want := PreviewWindow{Position: PreviewUp, Size: 30, Percent: true}
	if o.PreviewWin != want {
		t.Errorf("got %+v, want %+v", o.PreviewWin, want)
	}
	if !mustParse(t, "--preview-window=hidden").PreviewWin.Hidden {
		t.Error("hidden")
	}
	if got := mustParse(t, "--preview-window=left,20").PreviewWin; got.Percent || got.Size != 20 {
		t.Errorf("absolute size: %+v", got)
	}
	if _, err := Parse([]string{"--preview-window=diagonal"}); err == nil {
		t.Error("bad preview window should be rejected")
	}
}

func TestExpect(t *testing.T) {
	o := mustParse(t, "--expect=ctrl-v,ctrl-x", "--expect=alt-enter")
	for _, k := range []string{"ctrl-v", "ctrl-x", "alt-enter"} {
		if !o.Expect[k] {
			t.Errorf("missing expect key %q", k)
		}
	}
}

func TestFilterMode(t *testing.T) {
	o := mustParse(t, "-f", "main")
	if !o.HasFilter || o.Filter != "main" {
		t.Errorf("got %q %v", o.Filter, o.HasFilter)
	}
	// An empty filter is still a filter: it means "print everything".
	o = mustParse(t, "--filter=")
	if !o.HasFilter || o.Filter != "" {
		t.Errorf("got %q %v", o.Filter, o.HasFilter)
	}
}

func TestScriptingFlags(t *testing.T) {
	o := mustParse(t, "--print-query", "-1", "-0", "--read0", "--print0", "--ansi", "--no-sort", "--tac")
	if !o.PrintQuery || !o.Select1 || !o.Exit0 || !o.Read0 || !o.Print0 || !o.Ansi || o.Sort || !o.Tac {
		t.Errorf("%+v", o)
	}
}

func TestHeaderAccumulates(t *testing.T) {
	o := mustParse(t, "--header", "one", "--header=two\nthree")
	if !reflect.DeepEqual(o.Header, []string{"one", "two", "three"}) {
		t.Errorf("got %q", o.Header)
	}
}

func TestUnknownOption(t *testing.T) {
	if _, err := Parse([]string{"--nope"}); err == nil {
		t.Error("unknown option should be an error")
	}
	if _, err := Parse([]string{"-q"}); err == nil {
		t.Error("missing argument should be an error")
	}
}

func TestIgnoredCosmeticOptions(t *testing.T) {
	// Options that only affect the real fzf's looks are accepted so that an
	// existing FZF_DEFAULT_OPTS does not abort the program.
	if _, err := Parse([]string{"--color=dark", "--border", "--bind", "ctrl-a:abort"}); err != nil {
		t.Errorf("should be tolerated: %v", err)
	}
}

func TestSplitArgs(t *testing.T) {
	got := SplitArgs(`--height 40% --prompt "pick > " --query='a b'`)
	want := []string{"--height", "40%", "--prompt", "pick > ", "--query=a b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEnvDefaultsAreOverriddenByArgs(t *testing.T) {
	o, err := ParseWithDefaults("--height=40% --reverse", []string{"--layout=default"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.Height.Set || o.Height.Size != 40 {
		t.Error("env height should survive")
	}
	if o.Layout != LayoutDefault {
		t.Error("command line should override the environment")
	}
}

func TestEnvDefaultsErrorIsReported(t *testing.T) {
	if _, err := ParseWithDefaults("--bogus", nil); err == nil {
		t.Error("a broken FZF_DEFAULT_OPTS should be reported")
	}
}
