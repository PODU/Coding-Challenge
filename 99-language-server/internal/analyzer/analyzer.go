package analyzer

import (
	"strconv"
	"strings"
	"unicode/utf16"

	"greetls/internal/lsp"
)

// Diagnostics checks a .greet document. The result is never nil, so it
// encodes as [] rather than null when there are no problems.
func Diagnostics(text string) []lsp.Diagnostic {
	diags := []lsp.Diagnostic{}
	langCount := 0

	for lineNum, line := range splitLines(text) {
		// TODO is flagged anywhere, comments included.
		for idx := 0; ; {
			j := strings.Index(line[idx:], "TODO")
			if j < 0 {
				break
			}
			start := idx + j
			diags = append(diags, makeDiag(lineNum, line, start, start+4,
				lsp.SeverityInformation, "found TODO marker"))
			idx = start + 4
		}

		stripped := strings.TrimLeft(line, " \t")
		if stripped == "" || strings.HasPrefix(stripped, "#") {
			continue // blank or comment line
		}

		toks := tokenize(line)
		if len(toks) == 0 {
			continue
		}
		kw := toks[0]
		switch kw.text {
		case "greet":
			if !hasQuotedArg(toks[1:]) {
				diags = append(diags, makeDiag(lineNum, line, kw.start, len(line),
					lsp.SeverityError, `greet requires a quoted argument, e.g. greet "World"`))
			}
		case "lang":
			langCount++
			if langCount > 1 {
				diags = append(diags, makeDiag(lineNum, line, kw.start, kw.end,
					lsp.SeverityWarning, "duplicate lang directive"))
			}
		case "repeat":
			if len(toks) < 2 {
				diags = append(diags, makeDiag(lineNum, line, kw.start, kw.end,
					lsp.SeverityError, "repeat requires a positive integer argument"))
			} else if v, err := strconv.Atoi(toks[1].text); err != nil || v <= 0 {
				diags = append(diags, makeDiag(lineNum, line, toks[1].start, toks[1].end,
					lsp.SeverityError, "repeat value must be a positive integer"))
			}
		default:
			diags = append(diags, makeDiag(lineNum, line, kw.start, kw.end,
				lsp.SeverityWarning, "unknown keyword: "+kw.text))
		}
	}
	return diags
}

// Hover returns the docs for the keyword under pos, or nil if there isn't one.
func Hover(text string, pos lsp.Position) *lsp.Hover {
	lines := splitLines(text)
	if pos.Line < 0 || pos.Line >= len(lines) {
		return nil
	}
	line := lines[pos.Line]
	bytePos := byteForUTF16(line, pos.Character)

	tok, ok := tokenAt(line, bytePos)
	if !ok {
		return nil
	}
	word := keywordOf(tok.text)
	if word == "" {
		return nil
	}
	kw, ok := keywordIndex[word]
	if !ok {
		return nil
	}

	wordStart := tok.start + strings.Index(tok.text, word)
	wordEnd := wordStart + len(word)
	rng := lsp.Range{
		Start: lsp.Position{Line: pos.Line, Character: utf16Column(line, wordStart)},
		End:   lsp.Position{Line: pos.Line, Character: utf16Column(line, wordEnd)},
	}
	return &lsp.Hover{
		Contents: lsp.MarkupContent{Kind: lsp.MarkupKindMarkdown, Value: kw.Doc},
		Range:    &rng,
	}
}

// Completion offers attribute names if the word being typed starts with '@',
// and keywords otherwise.
func Completion(text string, pos lsp.Position) []lsp.CompletionItem {
	line := ""
	if lines := splitLines(text); pos.Line >= 0 && pos.Line < len(lines) {
		line = lines[pos.Line]
	}
	bytePos := byteForUTF16(line, pos.Character)
	if bytePos > len(line) {
		bytePos = len(line)
	}
	prefix := line[:bytePos]
	frag := prefix
	if i := strings.LastIndexAny(prefix, " \t"); i >= 0 {
		frag = prefix[i+1:]
	}
	if strings.HasPrefix(frag, "@") {
		return attributeItems()
	}
	return keywordItems()
}

type token struct {
	text  string
	start int // byte offsets
	end   int
}

// tokenize splits a line on blanks, keeping a quoted string (quotes included)
// as one token.
func tokenize(line string) []token {
	var toks []token
	i := 0
	for i < len(line) {
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		if i >= len(line) {
			break
		}
		start := i
		if line[i] == '"' {
			i++
			for i < len(line) && line[i] != '"' {
				i++
			}
			if i < len(line) {
				i++ // include the closing quote
			}
		} else {
			for i < len(line) && line[i] != ' ' && line[i] != '\t' {
				i++
			}
		}
		toks = append(toks, token{text: line[start:i], start: start, end: i})
	}
	return toks
}

func tokenAt(line string, bytePos int) (token, bool) {
	for _, t := range tokenize(line) {
		if bytePos >= t.start && bytePos < t.end {
			return t, true
		}
	}
	return token{}, false
}

func hasQuotedArg(toks []token) bool {
	for _, t := range toks {
		if len(t.text) >= 2 && strings.HasPrefix(t.text, `"`) && strings.HasSuffix(t.text, `"`) {
			return true
		}
	}
	return false
}

// keywordOf returns the leading word of a token: "@style=formal" -> "style".
func keywordOf(tok string) string {
	s := strings.TrimPrefix(tok, "@")
	j := 0
	for j < len(s) && isLetter(s[j]) {
		j++
	}
	return s[:j]
}

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func splitLines(text string) []string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	return lines
}

func makeDiag(lineNum int, src string, startByte, endByte int, sev lsp.DiagnosticSeverity, msg string) lsp.Diagnostic {
	return lsp.Diagnostic{
		Range: lsp.Range{
			Start: lsp.Position{Line: lineNum, Character: utf16Column(src, startByte)},
			End:   lsp.Position{Line: lineNum, Character: utf16Column(src, endByte)},
		},
		Severity: sev,
		Source:   "greetls",
		Message:  msg,
	}
}

// utf16Column converts a byte offset to an LSP column, which counts UTF-16
// code units.
func utf16Column(line string, bytePos int) int {
	if bytePos > len(line) {
		bytePos = len(line)
	}
	return len(utf16.Encode([]rune(line[:bytePos])))
}

// byteForUTF16 is the inverse of utf16Column.
func byteForUTF16(line string, col int) int {
	if col <= 0 {
		return 0
	}
	n := 0
	for i, r := range line {
		if n >= col {
			return i
		}
		n += len(utf16.Encode([]rune{r}))
	}
	return len(line)
}
