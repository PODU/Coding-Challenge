package count

import (
	"path/filepath"
	"strings"
	"testing"

	"loc/internal/lang"
)

func mustLang(t *testing.T, name string) *lang.LanguageDef {
	t.Helper()
	d, ok := lang.ByName(name)
	if !ok {
		t.Fatalf("language %q not in registry", name)
	}
	return d
}

// TestFixtureCounts asserts the golden blank/comment/code counts for each
// testdata fixture and the invariant Lines == Blank+Comment+Code.
func TestFixtureCounts(t *testing.T) {
	cases := []struct {
		file                        string
		lang                        string
		lines, blank, comment, code int
	}{
		{"go_basic.go", "Go", 12, 3, 4, 5},
		{"go_string_url.go", "Go", 6, 1, 1, 4},
		{"go_rawstring.go", "Go", 7, 1, 1, 5},
		{"py_docstring.py", "Python", 10, 2, 4, 4},
		{"c_mixed.c", "C", 8, 1, 2, 5},
		{"c_unterminated.c", "C", 5, 0, 3, 2},
		{"js_quotes.js", "JavaScript", 9, 2, 3, 4},
		{"plain.txt", "Text", 6, 2, 0, 4},
		{"doc.md", "Markdown", 11, 3, 5, 3},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			def := mustLang(t, tc.lang)
			fc, err := CountFile(filepath.Join("testdata", tc.file), def)
			if err != nil {
				t.Fatalf("CountFile: %v", err)
			}
			if fc.Lines != tc.lines || fc.Blank != tc.blank || fc.Comment != tc.comment || fc.Code != tc.code {
				t.Errorf("got Lines=%d Blank=%d Comment=%d Code=%d; want Lines=%d Blank=%d Comment=%d Code=%d",
					fc.Lines, fc.Blank, fc.Comment, fc.Code, tc.lines, tc.blank, tc.comment, tc.code)
			}
			if fc.Lines != fc.Blank+fc.Comment+fc.Code {
				t.Errorf("invariant violated: %d != %d+%d+%d", fc.Lines, fc.Blank, fc.Comment, fc.Code)
			}
		})
	}
}

// classifyString is a helper that runs the classifier over an inline source
// string and returns the counts.
func classifyString(t *testing.T, langName, src string) FileCount {
	t.Helper()
	def := mustLang(t, langName)
	fc, err := CountReader(strings.NewReader(src), def)
	if err != nil {
		t.Fatalf("CountReader: %v", err)
	}
	return fc
}

// TestCommentInStringNotCounted verifies that a comment marker inside a string
// literal is treated as code, not comment (edge case #2).
func TestCommentInStringNotCounted(t *testing.T) {
	fc := classifyString(t, "Go", `x := "http://example.com" // real tail`+"\n")
	// One physical line: code (the URL string) plus a real trailing comment ->
	// counts as code per scc/cloc semantics.
	if fc.Code != 1 || fc.Comment != 0 {
		t.Errorf("got Code=%d Comment=%d; want Code=1 Comment=0", fc.Code, fc.Comment)
	}
}

// TestStringInComment verifies quotes inside a block comment stay inert (#3).
func TestStringInComment(t *testing.T) {
	fc := classifyString(t, "C", "/* he said \"hi\" and 'bye' */\n")
	if fc.Comment != 1 || fc.Code != 0 {
		t.Errorf("got Comment=%d Code=%d; want Comment=1 Code=0", fc.Comment, fc.Code)
	}
}

// TestBlockSpanningLines verifies a block comment carried across many lines (#1).
func TestBlockSpanningLines(t *testing.T) {
	src := "code()\n/* a\nb\nc */\nmore()\n"
	fc := classifyString(t, "C", src)
	// lines: code, comment, comment, comment, code
	if fc.Code != 2 || fc.Comment != 3 || fc.Blank != 0 {
		t.Errorf("got Code=%d Comment=%d Blank=%d; want 2/3/0", fc.Code, fc.Comment, fc.Blank)
	}
}

// TestMixedLine verifies code wins on a mixed code+comment line (#4).
func TestMixedLine(t *testing.T) {
	fc := classifyString(t, "C", "i++; /* set i */\n")
	if fc.Code != 1 || fc.Comment != 0 {
		t.Errorf("got Code=%d Comment=%d; want Code=1 Comment=0", fc.Code, fc.Comment)
	}
}

// TestBlockCloseThenCode verifies "*/ y = 2" counts as code (#-, close+code).
func TestBlockCloseThenCode(t *testing.T) {
	src := "/* open\nstill */ y = 2\n"
	fc := classifyString(t, "C", src)
	if fc.Comment != 1 || fc.Code != 1 {
		t.Errorf("got Comment=%d Code=%d; want Comment=1 Code=1", fc.Comment, fc.Code)
	}
}

// TestNestedBlocks verifies Rust-style nested block comments (#6).
func TestNestedBlocks(t *testing.T) {
	src := "/* a /* b */ still comment */\nlet x = 1;\n"
	fc := classifyString(t, "Rust", src)
	if fc.Comment != 1 || fc.Code != 1 {
		t.Errorf("got Comment=%d Code=%d; want Comment=1 Code=1", fc.Comment, fc.Code)
	}
}

// TestEscapedQuote verifies backslash escapes keep string state correct (#19).
func TestEscapedQuote(t *testing.T) {
	// The escaped quote does not end the string, so the trailing // stays inside
	// the string -> the whole line is code.
	fc := classifyString(t, "Go", `s := "a\" // still string"`+"\n")
	if fc.Code != 1 || fc.Comment != 0 {
		t.Errorf("got Code=%d Comment=%d; want Code=1 Comment=0", fc.Code, fc.Comment)
	}
}

// TestBlankInsideBlock verifies an all-whitespace line inside a block comment
// counts as blank, not comment (blank wins).
func TestBlankInsideBlock(t *testing.T) {
	src := "/* start\n\nend */\n"
	fc := classifyString(t, "C", src)
	if fc.Blank != 1 || fc.Comment != 2 {
		t.Errorf("got Blank=%d Comment=%d; want Blank=1 Comment=2", fc.Blank, fc.Comment)
	}
}

// TestNoTrailingNewline verifies the last line is counted without a newline (#10).
func TestNoTrailingNewline(t *testing.T) {
	fc := classifyString(t, "Go", "package main")
	if fc.Lines != 1 || fc.Code != 1 {
		t.Errorf("got Lines=%d Code=%d; want 1/1", fc.Lines, fc.Code)
	}
}

// TestCRLF verifies CRLF line endings are handled (#11).
func TestCRLF(t *testing.T) {
	fc := classifyString(t, "Go", "package main\r\n// c\r\n")
	if fc.Lines != 2 || fc.Code != 1 || fc.Comment != 1 {
		t.Errorf("got Lines=%d Code=%d Comment=%d; want 2/1/1", fc.Lines, fc.Code, fc.Comment)
	}
}

// TestUnicode verifies rune-aware scanning of multibyte content (#18).
func TestUnicode(t *testing.T) {
	fc := classifyString(t, "Go", "x := \"héllo wörld\" // café\n")
	if fc.Code != 1 || fc.Comment != 0 {
		t.Errorf("got Code=%d Comment=%d; want Code=1 Comment=0", fc.Code, fc.Comment)
	}
}

// TestEmptyFile verifies an empty input yields all-zero counts (#9).
func TestEmptyFile(t *testing.T) {
	fc := classifyString(t, "Go", "")
	if fc.Lines != 0 {
		t.Errorf("got Lines=%d; want 0", fc.Lines)
	}
}
