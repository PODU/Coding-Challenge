package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runScript executes a script over input and returns the produced output.
func runScript(t *testing.T, script, input string, quiet bool) string {
	t.Helper()
	cmds, err := Parse(script)
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", script, err)
	}
	eng := &Engine{Commands: cmds, Quiet: quiet}
	var buf bytes.Buffer
	if err := eng.Run(strings.NewReader(input), &buf); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	return buf.String()
}

// --- Golden table: end-to-end behaviour mirroring the challenge steps. ----

func TestGolden(t *testing.T) {
	cases := []struct {
		name   string
		script string
		quiet  bool
		in     string
		want   string
	}{
		// Step 1: substitution + g flag.
		{"strip-quotes", `s/"//g`, false,
			"the \"quick\" \"fox\"\nno quotes\n",
			"the quick fox\nno quotes\n"},
		{"subst-first-only", `s/o/0/`, false, "foo boo\n", "f0o boo\n"},
		{"subst-global", `s/o/0/g`, false, "foo boo\n", "f00 b00\n"},

		// Nth occurrence and N+g.
		{"subst-nth", `s/o/0/2`, false, "ooooo\n", "o0ooo\n"},
		{"subst-nth-global", `s/o/0/2g`, false, "ooooo\n", "o0000\n"},

		// s///p print flag (with -n only changed lines appear).
		{"subst-p-quiet", `s/a/A/p`, true, "abc\nxyz\n", "Abc\n"},

		// Step 2: -n with line range.
		{"range-print", `2,4p`, true, "1\n2\n3\n4\n5\n", "2\n3\n4\n"},
		{"single-line-print", `3p`, true, "a\nb\nc\nd\n", "c\n"},
		{"last-line-print", `$p`, true, "a\nb\nc\n", "c\n"},

		// Step 3: regex address.
		{"regex-print", `/roads/p`, true,
			"main roads\nrivers\nback roads\n",
			"main roads\nback roads\n"},

		// Step 4: G double-spacing.
		{"double-space", `G`, false, "a\nb\n", "a\n\nb\n\n"},

		// Step 5: delete blank lines.
		{"delete-blank", `/^$/d`, false, "a\n\nb\n\nc\n", "a\nb\nc\n"},
		{"delete-line", `2d`, false, "a\nb\nc\n", "a\nc\n"},

		// Step 6: replace (in-place tested separately).
		{"replace-life", `s/Life/Code/g`, false,
			"Life is Life\n", "Code is Code\n"},

		// Backreferences and whole-match &.
		{"backref-swap", `s/\(a\)\(b\)/\2\1/`, false, "ab\n", "ba\n"},
		{"whole-match", `s/foo/[&]/`, false, "foo\n", "[foo]\n"},
		{"escaped-amp", `s/x/a\&b/`, false, "x\n", "a&b\n"},

		// Addressing: ranges, regex ranges, negation.
		{"regex-range", `/start/,/end/p`, true,
			"a\nstart\nb\nend\nc\nstart\nd\nend\ne\n",
			"start\nb\nend\nstart\nd\nend\n"},
		{"reversed-range", `4,2p`, true, "1\n2\n3\n4\n5\n", "4\n"},
		{"negate-range", `2,4!d`, false, "1\n2\n3\n4\n5\n", "2\n3\n4\n"},
		{"negate-regex", `/keep/!d`, false, "keep1\ndrop\nkeep2\n", "keep1\nkeep2\n"},

		// Multi-command via ; and newline.
		{"multi-semicolon", `1d;3d`, false, "a\nb\nc\nd\n", "b\nd\n"},
		{"multi-newline", "s/a/X/\np", true, "a\nb\n", "X\nb\n"},

		// Alternate delimiter and escaped delimiter.
		{"alt-delim", `s|a|b|`, false, "a/a\n", "b/a\n"},
		{"escaped-delim", `s/a\/b/Z/`, false, "a/b c\n", "Z c\n"},

		// Empty / no-op script behaves like cat.
		{"empty-script", ``, false, "a\nb\n", "a\nb\n"},

		// Case-insensitive flag.
		{"ci-flag", `s/abc/X/i`, false, "ABC def\n", "X def\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runScript(t, tc.script, tc.in, tc.quiet)
			if got != tc.want {
				t.Errorf("script %q\n in:   %q\n got:  %q\n want: %q",
					tc.script, tc.in, got, tc.want)
			}
		})
	}
}

// Trailing-newline characteristics must be preserved.
func TestTrailingNewline(t *testing.T) {
	if got := runScript(t, `s/a/b/`, "abc", false); got != "bbc" {
		t.Errorf("missing-newline input: got %q want %q", got, "bbc")
	}
	if got := runScript(t, `s/a/b/`, "abc\n", false); got != "bbc\n" {
		t.Errorf("newline input: got %q want %q", got, "bbc\n")
	}
	if got := runScript(t, `p`, "", false); got != "" {
		t.Errorf("empty input: got %q want empty", got)
	}
}

// Zero-width matches must not loop forever.
func TestZeroWidthMatch(t *testing.T) {
	got := runScript(t, `s/x*/-/g`, "abc", false)
	if !strings.Contains(got, "-") {
		t.Errorf("zero-width s///g produced %q", got)
	}
}

// d short-circuits remaining commands for the line.
func TestDeleteShortCircuit(t *testing.T) {
	// On line 1, `d` ends the cycle, so the following `p` must not run and the
	// line must not be auto-printed; line 2 passes through normally.
	got := runScript(t, "1d;1p", "a\nb\n", false)
	if got != "b\n" {
		t.Errorf("d short-circuit: got %q want %q", got, "b\n")
	}
}

// --- Parser unit tests ---------------------------------------------------

func TestParseErrors(t *testing.T) {
	bad := []string{
		`s/a/b`,    // unterminated
		`Z`,        // unknown command
		`s//b/`,    // empty regex
		`2,`,       // missing second address
		`p extra2`, // trailing junk after p (a non-separator)
		`s/(/x/`,   // invalid regex after BRE translation? '(' is literal -> ok actually
	}
	for _, s := range bad {
		if _, err := Parse(s); err == nil {
			// `s/(/x/` is actually valid (literal paren); skip that one.
			if s == `s/(/x/` {
				continue
			}
			t.Errorf("Parse(%q) expected error, got nil", s)
		}
	}
}

func TestParseValidForms(t *testing.T) {
	good := []string{
		`s/a/b/`, `s/a/b/g`, `s/a/b/2`, `s/a/b/2g`, `s/a/b/p`,
		`3p`, `2,4p`, `$d`, `/re/p`, `/a/,/b/d`, `2,4!p`, `/x/!d`,
		`1d;2d`, "1d\n2d", `G`, `s|a|b|`, `\#a#p`,
	}
	for _, s := range good {
		if _, err := Parse(s); err != nil {
			t.Errorf("Parse(%q) unexpected error: %v", s, err)
		}
	}
}

// --- BRE translation -----------------------------------------------------

func TestBREtoRE2(t *testing.T) {
	cases := map[string]string{
		`a\(b\)c`: `a(b)c`,
		`a+b`:     `a\+b`,
		`a\+b`:    `a+b`,
		`x\{2\}`:  `x{2}`,
		`a(b)`:    `a\(b\)`,
		`[a+b]`:   `[a+b]`,
		`\.`:      `\.`,
		`a\|b`:    `a|b`,
	}
	for in, want := range cases {
		if got := breToRE2(in); got != want {
			t.Errorf("breToRE2(%q) = %q want %q", in, got, want)
		}
	}
}

// --- Address matcher state ----------------------------------------------

func TestRegexRangeRetrigger(t *testing.T) {
	got := runScript(t, `/a/,/b/p`,
		"x\na\n1\nb\ny\na\n2\nb\nz\n", true)
	want := "a\n1\nb\na\n2\nb\n"
	if got != want {
		t.Errorf("regex range re-trigger: got %q want %q", got, want)
	}
}

// --- CLI parsing ---------------------------------------------------------

func TestParseArgs(t *testing.T) {
	t.Run("script-as-operand", func(t *testing.T) {
		o, err := ParseArgs([]string{"s/a/b/", "file.txt"})
		if err != nil {
			t.Fatal(err)
		}
		if o.Script != "s/a/b/" || len(o.Files) != 1 || o.Files[0] != "file.txt" {
			t.Errorf("got %+v", o)
		}
	})
	t.Run("quiet-and-e", func(t *testing.T) {
		o, err := ParseArgs([]string{"-n", "-e", "1p", "-e", "2p", "f"})
		if err != nil {
			t.Fatal(err)
		}
		if !o.Quiet || o.Script != "1p\n2p" || len(o.Files) != 1 {
			t.Errorf("got %+v", o)
		}
	})
	t.Run("bundled-ne", func(t *testing.T) {
		o, err := ParseArgs([]string{"-ne", "1p", "f"})
		if err != nil {
			t.Fatal(err)
		}
		if !o.Quiet || o.Script != "1p" {
			t.Errorf("got %+v", o)
		}
	})
	t.Run("inplace-suffix", func(t *testing.T) {
		o, err := ParseArgs([]string{"-i.bak", "s/a/b/", "f"})
		if err != nil {
			t.Fatal(err)
		}
		if !o.InPlace || o.BackupSuffix != ".bak" {
			t.Errorf("got %+v", o)
		}
	})
	t.Run("double-dash", func(t *testing.T) {
		o, err := ParseArgs([]string{"-n", "--", "-weird-script"})
		if err != nil {
			t.Fatal(err)
		}
		if o.Script != "-weird-script" {
			t.Errorf("got script %q", o.Script)
		}
	})
	t.Run("unknown-option", func(t *testing.T) {
		if _, err := ParseArgs([]string{"-Z"}); err == nil {
			t.Error("expected error for -Z")
		}
	})
}

// --- In-place editing ----------------------------------------------------

func TestRunInPlace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")
	if err := os.WriteFile(path, []byte("Life is Life\nmore Life\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmds, _ := Parse(`s/Life/Code/g`)
	eng := &Engine{Commands: cmds}

	t.Run("no-backup", func(t *testing.T) {
		if err := eng.RunInPlace(path, ""); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(path)
		if string(got) != "Code is Code\nmore Code\n" {
			t.Errorf("in-place got %q", got)
		}
	})

	t.Run("with-backup", func(t *testing.T) {
		p2 := filepath.Join(dir, "b.txt")
		os.WriteFile(p2, []byte("Life\n"), 0o644)
		if err := eng.RunInPlace(p2, ".bak"); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(p2)
		if string(got) != "Code\n" {
			t.Errorf("edited got %q", got)
		}
		bak, err := os.ReadFile(p2 + ".bak")
		if err != nil {
			t.Fatalf("backup not created: %v", err)
		}
		if string(bak) != "Life\n" {
			t.Errorf("backup got %q", bak)
		}
	})
}

// --- Full run() entry point ---------------------------------------------

func TestRunEntryPoint(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"-n", "2,4p"}, strings.NewReader("1\n2\n3\n4\n5\n"), &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, errb.String())
	}
	if out.String() != "2\n3\n4\n" {
		t.Errorf("got %q", out.String())
	}
}

func TestRunMissingFile(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"p", "no-such-file-xyz.txt"}, strings.NewReader(""), &out, &errb)
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
}

func TestRunBadScript(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"Z"}, strings.NewReader(""), &out, &errb)
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
}

func TestRunInPlaceStdinError(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"-i", "s/a/b/"}, strings.NewReader("a\n"), &out, &errb)
	if code != 2 {
		t.Errorf("expected exit 2 for -i without files, got %d", code)
	}
}

// Fuzz: the parser must never panic — it either parses or returns an error.
func FuzzParse(f *testing.F) {
	for _, s := range []string{`s/a/b/g`, `2,4!p`, `/re/d`, `G`, `s|a|b|2g`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = Parse(s)
	})
}
