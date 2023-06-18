package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// newTestShell returns a Shell whose IO is captured in buffers and whose
// history is backed by a temp file, so tests never touch the real terminal or
// the user's ~/.ccsh_history.
func newTestShell(t *testing.T) (*Shell, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errb bytes.Buffer
	sh := &Shell{
		Stdin:   strings.NewReader(""),
		Stdout:  &out,
		Stderr:  &errb,
		History: NewHistory(filepath.Join(t.TempDir(), "history")),
	}
	wd, _ := os.Getwd()
	sh.Cwd = wd
	return sh, &out, &errb
}

// quoteArg wraps a path in double quotes so the lexer keeps it as one word
// even if it contains spaces (common in Windows temp paths).
func quoteArg(s string) string { return `"` + s + `"` }

func readFileTrim(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// shellCmd builds an argv that runs s through the platform command processor,
// so pipeline tests work on Windows (cmd /c) and Unix (sh -c) alike.
func shellCmd(s string) []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd", "/c", s}
	}
	return []string{"sh", "-c", s}
}

func tokenValues(toks []Token) []string {
	out := make([]string, len(toks))
	for i, t := range toks {
		out[i] = t.Value
	}
	return out
}

func TestTokenizeBareWords(t *testing.T) {
	toks, err := Tokenize("ls -la /tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := tokenValues(toks)
	want := []string{"ls", "-la", "/tmp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestTokenizeQuotesAndEscapes(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{`echo "a  b"`, []string{"echo", "a  b"}},
		{`echo 'a|b'`, []string{"echo", "a|b"}},
		{`echo a\ b`, []string{"echo", "a b"}},
		{`echo "say \"hi\""`, []string{"echo", `say "hi"`}},
		{`pre"in"post`, []string{"preinpost"}},
	}
	for _, c := range cases {
		toks, err := Tokenize(c.in)
		if err != nil {
			t.Fatalf("%q: unexpected error: %v", c.in, err)
		}
		if got := tokenValues(toks); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%q: got %v want %v", c.in, got, c.want)
		}
	}
}

func TestTokenizeOperators(t *testing.T) {
	toks, err := Tokenize("a|b >c >>d <e")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantTypes := []TokenType{
		TokenWord, TokenPipe, TokenWord,
		TokenRedirectOut, TokenWord,
		TokenRedirectAppend, TokenWord,
		TokenRedirectIn, TokenWord,
	}
	if len(toks) != len(wantTypes) {
		t.Fatalf("got %d tokens (%s), want %d", len(toks), describe(toks), len(wantTypes))
	}
	for i, ty := range wantTypes {
		if toks[i].Type != ty {
			t.Fatalf("token %d: got %s want %s", i, toks[i].Type, ty)
		}
	}
}

func TestTokenizeUnterminatedQuote(t *testing.T) {
	if _, err := Tokenize(`echo "oops`); err == nil {
		t.Fatal("expected unterminated-quote error, got nil")
	}
	if _, err := Tokenize(`echo 'oops`); err == nil {
		t.Fatal("expected unterminated-quote error, got nil")
	}
	if _, err := Tokenize(`echo trailing\`); err == nil {
		t.Fatal("expected trailing-backslash error, got nil")
	}
}

func TestParseSingleCommand(t *testing.T) {
	toks, _ := Tokenize("ls -la")
	pl, err := Parse(toks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pl.Commands) != 1 {
		t.Fatalf("got %d commands, want 1", len(pl.Commands))
	}
	if !reflect.DeepEqual(pl.Commands[0].Argv, []string{"ls", "-la"}) {
		t.Fatalf("argv = %v", pl.Commands[0].Argv)
	}
}

func TestParseThreeStagePipeline(t *testing.T) {
	toks, _ := Tokenize("a | b -x | c")
	pl, err := Parse(toks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pl.Commands) != 3 {
		t.Fatalf("got %d stages, want 3", len(pl.Commands))
	}
	if pl.Commands[1].Argv[0] != "b" || pl.Commands[1].Argv[1] != "-x" {
		t.Fatalf("stage 2 argv = %v", pl.Commands[1].Argv)
	}
}

func TestParseRedirects(t *testing.T) {
	toks, _ := Tokenize("cat <in >out >>log")
	pl, err := Parse(toks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cmd := pl.Commands[0]
	if !reflect.DeepEqual(cmd.Argv, []string{"cat"}) {
		t.Fatalf("argv = %v", cmd.Argv)
	}
	want := []Redirect{
		{Type: TokenRedirectIn, Target: "in"},
		{Type: TokenRedirectOut, Target: "out"},
		{Type: TokenRedirectAppend, Target: "log"},
	}
	if !reflect.DeepEqual(cmd.Redirs, want) {
		t.Fatalf("redirs = %v want %v", cmd.Redirs, want)
	}
}

func TestParseSyntaxErrors(t *testing.T) {
	cases := []string{"ls |", "| wc", "ls | | wc", "cat >"}
	for _, c := range cases {
		toks, lexErr := Tokenize(c)
		if lexErr != nil {
			continue // already rejected by lexer, also acceptable
		}
		if _, err := Parse(toks); err == nil {
			t.Fatalf("%q: expected syntax error, got nil", c)
		}
	}
}

func TestParseRejectsBackground(t *testing.T) {
	toks, _ := Tokenize("sleep 5 &")
	if _, err := Parse(toks); err == nil {
		t.Fatal("expected background-not-supported error")
	}
}

func TestBuiltinPwdReflectsCd(t *testing.T) {
	orig, _ := os.Getwd()
	defer os.Chdir(orig)

	sh, out, errb := newTestShell(t)

	tmp := t.TempDir()
	wantDir, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		wantDir = tmp
	}

	if st, _ := sh.Execute("cd " + quoteArg(tmp)); st != 0 {
		t.Fatalf("cd failed (status %d): %s", st, errb.String())
	}
	out.Reset()
	if st, _ := sh.Execute("pwd"); st != 0 {
		t.Fatalf("pwd failed: %s", errb.String())
	}

	gotDir := strings.TrimSpace(out.String())
	gotResolved, err := filepath.EvalSymlinks(gotDir)
	if err != nil {
		gotResolved = gotDir
	}
	if !strings.EqualFold(gotResolved, wantDir) {
		t.Fatalf("pwd = %q, want %q", gotResolved, wantDir)
	}
}

func TestBuiltinCdDash(t *testing.T) {
	orig, _ := os.Getwd()
	defer os.Chdir(orig)

	sh, _, _ := newTestShell(t)
	start, _ := filepath.EvalSymlinks(t.TempDir())
	other, _ := filepath.EvalSymlinks(t.TempDir())

	if st, _ := sh.Execute("cd " + quoteArg(start)); st != 0 {
		t.Fatal("cd start failed")
	}
	if st, _ := sh.Execute("cd " + quoteArg(other)); st != 0 {
		t.Fatal("cd other failed")
	}
	// cd - should return to "start".
	if st, _ := sh.Execute("cd -"); st != 0 {
		t.Fatal("cd - failed")
	}
	got, _ := filepath.EvalSymlinks(sh.Cwd)
	if !strings.EqualFold(got, start) {
		t.Fatalf("after cd - cwd = %q, want %q", got, start)
	}
}

func TestBuiltinCdBadDir(t *testing.T) {
	sh, _, errb := newTestShell(t)
	st, _ := sh.Execute("cd " + quoteArg(filepath.Join(t.TempDir(), "does-not-exist")))
	if st == 0 {
		t.Fatal("expected non-zero status for missing dir")
	}
	if !strings.Contains(errb.String(), "cd:") {
		t.Fatalf("expected cd error message, got %q", errb.String())
	}
}

func TestBuiltinExitStatus(t *testing.T) {
	sh, _, _ := newTestShell(t)
	st, quit := sh.Execute("exit 42")
	if !quit {
		t.Fatal("exit should request quit")
	}
	if st != 42 {
		t.Fatalf("exit status = %d, want 42", st)
	}
}

func TestBuiltinHistory(t *testing.T) {
	sh, out, _ := newTestShell(t)
	_ = sh.History.Add("first")
	_ = sh.History.Add("second")

	sh.Execute("history")
	listing := out.String()
	if !strings.Contains(listing, "first") || !strings.Contains(listing, "second") {
		t.Fatalf("history listing missing entries: %q", listing)
	}

	sh.Execute("history -c")
	if len(sh.History.Entries()) != 0 {
		t.Fatal("history -c did not clear entries")
	}
}

func TestHistoryRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".ccsh_history")
	h := NewHistory(path)
	_ = h.Add("alpha")
	_ = h.Add("beta")
	_ = h.Add("beta") // consecutive duplicate, should be suppressed
	_ = h.Add("gamma")

	h2 := NewHistory(path)
	if err := h2.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	got := h2.Entries()
	want := []string{"alpha", "beta", "gamma"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reloaded history = %v, want %v", got, want)
	}
}

func TestHistoryMissingFileTolerated(t *testing.T) {
	h := NewHistory(filepath.Join(t.TempDir(), "nope", "missing"))
	if err := h.Load(); err != nil {
		t.Fatalf("missing history file should not error, got %v", err)
	}
	if len(h.Entries()) != 0 {
		t.Fatal("expected empty history")
	}
}

func TestUnknownCommand(t *testing.T) {
	sh, _, errb := newTestShell(t)
	sh.Stdin = strings.NewReader("")
	st, _ := sh.Execute("definitely-not-a-real-command-xyz")
	if st != statusNotFound {
		t.Fatalf("status = %d, want %d", st, statusNotFound)
	}
	if !strings.Contains(errb.String(), "command not found") {
		t.Fatalf("expected not-found message, got %q", errb.String())
	}
	// The shell must keep working afterwards.
	if _, quit := sh.Execute("exit"); !quit {
		t.Fatal("shell did not survive unknown command")
	}
}

func TestPipelineExecution(t *testing.T) {
	sh, out, errb := newTestShell(t)
	sh.Stdin = strings.NewReader("")

	// echo hello | <filter for "hello">  -> the second stage only emits the
	// line if the pipe delivered it, proving stdout->stdin wiring works.
	var filter string
	if runtime.GOOS == "windows" {
		filter = "findstr hello"
	} else {
		filter = "grep hello"
	}
	pl := &Pipeline{Commands: []Command{
		{Argv: shellCmd("echo hello")},
		{Argv: shellCmd(filter)},
	}}

	st := sh.RunPipeline(pl)
	if st != 0 {
		t.Fatalf("pipeline status = %d, stderr=%q", st, errb.String())
	}
	if got := strings.TrimSpace(out.String()); got != "hello" {
		t.Fatalf("pipeline output = %q, want %q", got, "hello")
	}
}

func TestThreeStagePipelineExecution(t *testing.T) {
	sh, out, errb := newTestShell(t)
	sh.Stdin = strings.NewReader("")

	var filter string
	if runtime.GOOS == "windows" {
		filter = "findstr o"
	} else {
		filter = "grep o"
	}
	pl := &Pipeline{Commands: []Command{
		{Argv: shellCmd("echo hello")},
		{Argv: shellCmd(filter)},
		{Argv: shellCmd(filter)},
	}}

	if st := sh.RunPipeline(pl); st != 0 {
		t.Fatalf("pipeline status = %d, stderr=%q", st, errb.String())
	}
	if got := strings.TrimSpace(out.String()); got != "hello" {
		t.Fatalf("3-stage output = %q, want %q", got, "hello")
	}
}

func TestPipelineWithRedirect(t *testing.T) {
	sh, _, errb := newTestShell(t)
	sh.Stdin = strings.NewReader("")
	outFile := filepath.Join(t.TempDir(), "out.txt")

	pl := &Pipeline{Commands: []Command{
		{Argv: shellCmd("echo redirected")},
		{
			Argv:   shellCmd(catCmd()),
			Redirs: []Redirect{{Type: TokenRedirectOut, Target: outFile}},
		},
	}}

	if st := sh.RunPipeline(pl); st != 0 {
		t.Fatalf("status = %d, stderr=%q", st, errb.String())
	}
	data, err := readFileTrim(outFile)
	if err != nil {
		t.Fatalf("read out file: %v", err)
	}
	if data != "redirected" {
		t.Fatalf("redirect file content = %q, want %q", data, "redirected")
	}
}

// catCmd returns a command that reproduces single-line stdin on stdout on the
// host platform (sort is a stable pass-through for a single line on Windows).
func catCmd() string {
	if runtime.GOOS == "windows" {
		return "sort"
	}
	return "cat"
}
