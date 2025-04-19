package ccomment

import (
	"bytes"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

var update = flag.Bool("update", false, "regenerate golden .out files")

func dirStripper(t *testing.T, dir string) Stripper {
	t.Helper()
	name := map[string]string{
		"python": "python", "js": "js", "c": "c", "go": "go",
	}[dir]
	s, ok := For(name)
	if !ok {
		t.Fatalf("no stripper for dir %q", dir)
	}
	return s
}

// TestGolden runs every testdata/<lang>/<case>.in through the matching backend
// with default options and compares against <case>.out (regenerate with
// -update).
func TestGolden(t *testing.T) {
	ins, err := filepath.Glob("testdata/*/*.in")
	if err != nil {
		t.Fatal(err)
	}
	if len(ins) == 0 {
		t.Fatal("no golden inputs found")
	}
	opt := Options{KeepDoc: true, KeepPragma: true, BlankLines: Drop}
	for _, in := range ins {
		in := in
		t.Run(filepath.ToSlash(in), func(t *testing.T) {
			dir := filepath.Base(filepath.Dir(in))
			src, err := os.ReadFile(in)
			if err != nil {
				t.Fatal(err)
			}
			out, _, err := dirStripper(t, dir).Strip(src, opt)
			if err != nil {
				t.Fatalf("strip: %v", err)
			}
			outPath := in[:len(in)-len(".in")] + ".out"
			if *update {
				if err := os.WriteFile(outPath, out, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(outPath)
			if err != nil {
				t.Fatalf("read golden (run -update?): %v", err)
			}
			if !bytes.Equal(out, want) {
				t.Errorf("mismatch for %s\n--- got ---\n%s\n--- want ---\n%s",
					in, out, want)
			}
		})
	}
}

// stripStr is a test convenience.
func stripStr(t *testing.T, lang, src string, opt Options) string {
	t.Helper()
	s, ok := For(lang)
	if !ok {
		t.Fatalf("no stripper for %q", lang)
	}
	out, _, err := s.Strip([]byte(src), opt)
	if err != nil {
		t.Fatalf("strip %s: %v", lang, err)
	}
	return string(out)
}

var defOpt = Options{KeepDoc: true, KeepPragma: true, BlankLines: Drop}

// TestDisambiguation is the critical string-vs-comment suite: delimiters inside
// literals must survive byte-for-byte while real comments are removed.
func TestDisambiguation(t *testing.T) {
	cases := []struct {
		name, lang, in, want string
	}{
		{"py_hash_in_string", "python", `s = "# not a comment"`, `s = "# not a comment"`},
		{"py_block_in_string", "python", `s = '/* nope */'`, `s = '/* nope */'`},
		{"py_url", "python", `u = "http://x"  # c`, `u = "http://x"`},
		{"py_char_hash", "python", `c = '#'`, `c = '#'`},
		{"py_remove_trailing", "python", "x = 1  # gone\n", "x = 1\n"},
		{"py_raw", "python", `p = r"raw # not"  # gone`, `p = r"raw # not"`},

		{"c_slash_in_string", "c", `char *p = "// not a comment";`, `char *p = "// not a comment";`},
		{"c_block_in_string", "c", `char *p = "/* still a string */";`, `char *p = "/* still a string */";`},
		{"c_char_literal", "c", `char c = '#';`, `char c = '#';`},
		{"c_escaped_quote", "c", `char *p = "a\"// still string";`, `char *p = "a\"// still string";`},
		{"c_trailing", "c", "int x = 5; // gone\n", "int x = 5;\n"},
		{"c_not_self_closing", "c", "a /*/ b */ c", "a  c"},

		{"js_url", "js", `const u = "http://x"; // gone`, `const u = "http://x";`},
		{"js_block_str", "js", `const s = "/* not a comment */";`, `const s = "/* not a comment */";`},
		{"js_regex", "js", `const ok = /ab/.test(x); // gone`, `const ok = /ab/.test(x);`},
		{"js_regex_slashes", "js", `let re = /\/\//;`, `let re = /\/\//;`},
		{"js_div_then_comment", "js", `let q = a / b; // gone`, `let q = a / b;`},
		{"js_b_slash_2", "js", "let q = b // 2\n", "let q = b\n"},

		{"go_slash_in_string", "go", `s := "// not a comment"`, `s := "// not a comment"`},
		{"go_raw_string", "go", "s := `a // b /* c */`", "s := `a // b /* c */`"},
		{"go_rune", "go", `r := '/'`, `r := '/'`},
		{"go_rune_escaped", "go", `r := '\''`, `r := '\''`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stripStr(t, c.lang, c.in, defOpt)
			if got != c.want {
				t.Errorf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestPragmaPreserved(t *testing.T) {
	cases := []struct{ name, lang, in, want string }{
		{"py_noqa", "python", "import os  # noqa\n", "import os  # noqa\n"},
		{"py_type", "python", "x = 1  # type: int\n", "x = 1  # type: int\n"},
		{"py_shebang", "python", "#!/usr/bin/env python3\nx = 1\n", "#!/usr/bin/env python3\nx = 1\n"},
		{"py_normal_removed", "python", "x = 1  # normal\n", "x = 1\n"},
		{"go_generate", "go", "//go:generate x\nvar y int\n", "//go:generate x\nvar y int\n"},
		{"go_build", "go", "//go:build linux\n\npackage main\n", "//go:build linux\n\npackage main\n"},
		{"go_plusbuild", "go", "// +build linux\n\npackage main\n", "// +build linux\n\npackage main\n"},
		{"js_eslint", "js", "// eslint-disable\nlet a = 1; // gone\n", "// eslint-disable\nlet a = 1;\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stripStr(t, c.lang, c.in, defOpt)
			if got != c.want {
				t.Errorf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestGoDocPreservation(t *testing.T) {
	src := "// Doc for F.\nfunc F() {}\n\n// Detached.\n\nfunc G() {}\n"
	// The detached comment is removed; with default (non-minify) options the two
	// blank lines that surrounded it are preserved byte-faithfully.
	want := "// Doc for F.\nfunc F() {}\n\n\nfunc G() {}\n"
	got := stripStr(t, "go", src, defOpt)
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}

	// A decl-like comment inside a function body is not a doc comment.
	inner := "func F() {\n\t// not doc\n\tvar x int\n\t_ = x\n}\n"
	wantInner := "func F() {\n\tvar x int\n\t_ = x\n}\n"
	if got := stripStr(t, "go", inner, defOpt); got != wantInner {
		t.Errorf("inner: got %q want %q", got, wantInner)
	}
}

// TestGoDocOracle cross-checks the heuristic doc-comment set against go/ast's
// own Doc fields for representative sources.
func TestGoDocOracle(t *testing.T) {
	files := []string{"testdata/go/doc.in", "testdata/go/pragma.in"}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		want := astDocOffsets(t, src)
		got := heuristicDocOffsets(src)
		if !equalIntSets(got, want) {
			t.Errorf("%s: heuristic doc set %v != ast doc set %v", f, got, want)
		}
	}
}

func heuristicDocOffsets(src []byte) []int {
	g := newGoStripper()
	spans, err := g.Scan(src)
	if err != nil {
		return nil
	}
	li := newLineIndex(src, spans)
	var out []int
	for _, s := range spans {
		if li.isDoc(src, s) {
			out = append(out, s.Start)
		}
	}
	sort.Ints(out)
	return out
}

func astDocOffsets(t *testing.T, src []byte) []int {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	set := map[int]bool{}
	addGroup := func(cg *ast.CommentGroup) {
		if cg == nil {
			return
		}
		for _, c := range cg.List {
			set[fset.Position(c.Pos()).Offset] = true
		}
	}
	addGroup(file.Doc)
	for _, d := range file.Decls {
		switch n := d.(type) {
		case *ast.GenDecl:
			addGroup(n.Doc)
			for _, sp := range n.Specs {
				switch s := sp.(type) {
				case *ast.TypeSpec:
					addGroup(s.Doc)
				case *ast.ValueSpec:
					addGroup(s.Doc)
				}
			}
		case *ast.FuncDecl:
			addGroup(n.Doc)
		}
	}
	var out []int
	for k := range set {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

func equalIntSets(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestUnterminatedBlock(t *testing.T) {
	for _, lang := range []string{"c", "cpp", "js", "go"} {
		s, _ := For(lang)
		_, _, err := s.Strip([]byte("int x; /* never closed"), Options{})
		if err == nil {
			t.Errorf("%s: expected unterminated block error", lang)
			continue
		}
		var ue *UnterminatedError
		if !asUnterminated(err, &ue) {
			t.Errorf("%s: wrong error type: %v", lang, err)
		}
	}
}

func asUnterminated(err error, target **UnterminatedError) bool {
	if ue, ok := err.(*UnterminatedError); ok {
		*target = ue
		return true
	}
	return false
}

func TestBlankLinesMode(t *testing.T) {
	src := "a = 1\n# full line\nb = 2\n"
	got := stripStr(t, "python", src, Options{KeepPragma: true, BlankLines: Blank})
	want := "a = 1\n\nb = 2\n"
	if got != want {
		t.Errorf("blank mode: got %q want %q", got, want)
	}
	got = stripStr(t, "python", src, Options{KeepPragma: true, BlankLines: Drop})
	want = "a = 1\nb = 2\n"
	if got != want {
		t.Errorf("drop mode: got %q want %q", got, want)
	}
}

func TestRobustness(t *testing.T) {
	cases := []struct{ name, lang, in string }{
		{"empty", "python", ""},
		{"only_comment", "python", "# just a comment"},
		{"no_final_newline", "c", "int x; // c"},
		{"crlf", "c", "int x; // c\r\nint y;\r\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, _ := For(c.lang)
			if _, _, err := s.Strip([]byte(c.in), defOpt); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
	// CRLF preserved on a non-commented following line.
	got := stripStr(t, "c", "int x; // c\r\nint y;\r\n", defOpt)
	if !bytes.Contains([]byte(got), []byte("int y;\r\n")) {
		t.Errorf("CRLF not preserved: %q", got)
	}
}

// FuzzIdentity asserts the scanner never panics and that removing zero comments
// reproduces the input exactly (the core safety property).
func FuzzIdentity(f *testing.F) {
	seeds := []string{
		"x = 1 # c", `s = "# no"`, "a /*b*/ c", "func F(){}",
		"`raw`", "/ab/.test()", "char c = '#';", "",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	langs := []string{"python", "c", "cpp", "js", "go"}
	f.Fuzz(func(t *testing.T, in string) {
		for _, lang := range langs {
			s, _ := For(lang)
			spans, err := s.Scan([]byte(in))
			if err != nil {
				continue // unterminated block, etc.
			}
			out, _ := stripWith([]byte(in), spans, Options{}, func(Span) bool { return true })
			if !bytes.Equal(out, []byte(in)) {
				t.Fatalf("%s: keep-all changed input\nin:  %q\nout: %q", lang, in, out)
			}
		}
	})
}
