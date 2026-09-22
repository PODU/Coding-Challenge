// Package item holds an input line and everything derived from it: the text
// shown on screen, the text patterns are matched against, and any ANSI colour
// spans recovered from the input.
package item

import (
	"regexp"
	"strconv"
	"strings"

	"ccfzf/internal/algo"
)

// Delimiter splits a line into fields. A plain string is used literally
// because that is both faster and what users mean by "-d :"; anything with
// regex metacharacters is compiled.
type Delimiter struct {
	str   string
	regex *regexp.Regexp
	awk   bool // default: runs of whitespace
}

// DefaultDelimiter is AWK-style: split on whitespace runs.
func DefaultDelimiter() Delimiter { return Delimiter{awk: true} }

func ParseDelimiter(str string) Delimiter {
	str = strings.ReplaceAll(str, "\\t", "\t")
	str = strings.ReplaceAll(str, "\\n", "\n")
	if regexp.QuoteMeta(str) == str {
		return Delimiter{str: str}
	}
	rx, err := regexp.Compile(str)
	if err != nil {
		return Delimiter{str: str}
	}
	return Delimiter{regex: rx}
}

// Tokenize splits text into fields. Each token keeps the delimiter that
// followed it, so joining a subset of tokens reproduces the original spacing.
func Tokenize(text string, d Delimiter) []string {
	if d.awk {
		return awkTokens(text)
	}
	var tokens []string
	if d.regex != nil {
		begin := 0
		for _, loc := range d.regex.FindAllStringIndex(text, -1) {
			if loc[1] == loc[0] {
				continue // zero-width match, would not make progress
			}
			tokens = append(tokens, text[begin:loc[1]])
			begin = loc[1]
		}
		if begin < len(text) {
			tokens = append(tokens, text[begin:])
		}
		return tokens
	}
	if d.str == "" {
		return []string{text}
	}
	begin := 0
	for {
		i := strings.Index(text[begin:], d.str)
		if i < 0 {
			break
		}
		end := begin + i + len(d.str)
		tokens = append(tokens, text[begin:end])
		begin = end
	}
	if begin < len(text) {
		tokens = append(tokens, text[begin:])
	}
	return tokens
}

func awkTokens(text string) []string {
	var tokens []string
	begin, end := -1, -1
	inWhite := false
	for i := 0; i < len(text); i++ {
		white := text[i] == ' ' || text[i] == '\t'
		switch {
		case begin < 0:
			if !white {
				begin, end, inWhite = i, i+1, false
			}
		case inWhite:
			if white {
				end = i + 1
			} else {
				tokens = append(tokens, text[begin:end])
				begin, end, inWhite = i, i+1, false
			}
		default:
			end = i + 1
			if white {
				inWhite = true
			}
		}
	}
	if begin >= 0 && begin < end {
		tokens = append(tokens, text[begin:end])
	}
	return tokens
}

// Range is a 1-based, inclusive field range. Negative numbers count from the
// end; 0 on either side means "open".
type Range struct {
	Begin int
	End   int
}

const rangeEllipsis = 0

// ParseNth parses a comma-separated field spec such as "1,3..5,-1,2..".
func ParseNth(spec string) ([]Range, error) {
	var ranges []Range
	for _, part := range strings.Split(spec, ",") {
		r, err := parseRange(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, r)
	}
	return ranges, nil
}

func parseRange(str string) (Range, error) {
	fail := func() (Range, error) {
		return Range{}, &ParseError{"invalid field range: " + str}
	}
	switch {
	case str == "..":
		return Range{rangeEllipsis, rangeEllipsis}, nil
	case strings.HasPrefix(str, ".."):
		n, err := strconv.Atoi(str[2:])
		if err != nil || n == 0 {
			return fail()
		}
		return Range{rangeEllipsis, n}, nil
	case strings.HasSuffix(str, ".."):
		n, err := strconv.Atoi(str[:len(str)-2])
		if err != nil || n == 0 {
			return fail()
		}
		return Range{n, rangeEllipsis}, nil
	case strings.Contains(str, ".."):
		parts := strings.SplitN(str, "..", 2)
		b, err1 := strconv.Atoi(parts[0])
		e, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil || b == 0 || e == 0 {
			return fail()
		}
		return Range{b, e}, nil
	}
	n, err := strconv.Atoi(str)
	if err != nil || n == 0 {
		return fail()
	}
	return Range{n, n}, nil
}

type ParseError struct{ Msg string }

func (e *ParseError) Error() string { return e.Msg }

// Transform joins the tokens selected by ranges.
func Transform(tokens []string, ranges []Range) string {
	if len(ranges) == 0 {
		return strings.Join(tokens, "")
	}
	var sb strings.Builder
	n := len(tokens)
	for _, r := range ranges {
		begin, end := r.Begin, r.End
		if begin == rangeEllipsis && end == rangeEllipsis {
			begin, end = 1, n
		} else {
			if begin == rangeEllipsis {
				begin = 1
			} else if begin < 0 {
				begin = n + begin + 1
			}
			if end == rangeEllipsis {
				end = n
			} else if end < 0 {
				end = n + end + 1
			}
		}
		begin = max(begin, 1)
		end = min(end, n)
		for i := begin; i <= end; i++ {
			sb.WriteString(tokens[i-1])
		}
	}
	// Trailing delimiters belong to the tokens, but a field the user asked for
	// should not drag the separator that followed it into the match text.
	return strings.TrimRight(sb.String(), " \t")
}

// Span is a run of display characters that carried an SGR sequence in the
// input. Offsets are rune indices into the stripped text.
type Span struct {
	Start int
	End   int
	Code  string // the SGR parameters, e.g. "1;31"
}

var ansiRegex = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]|\x1b\\][^\x07\x1b]*(\x07|\x1b\\\\)|\x1b[@-Z\\\\-_]")

// StripAnsi removes escape sequences, returning the visible text and the
// colour spans it carried. Rather than decode SGR parameters into a colour
// model, the parameter string is kept verbatim and replayed at render time -
// which means 256-colour and true-colour input survives untouched.
func StripAnsi(text string) (string, []Span) {
	if !strings.Contains(text, "\x1b") {
		return text, nil
	}
	var sb strings.Builder
	var spans []Span
	runeLen := 0
	current := ""
	start := 0
	last := 0

	flush := func(upto int) {
		chunk := text[last:upto]
		sb.WriteString(chunk)
		runeLen += len([]rune(chunk))
		last = upto
	}
	closeSpan := func() {
		if current != "" && runeLen > start {
			spans = append(spans, Span{start, runeLen, current})
		}
	}

	for _, loc := range ansiRegex.FindAllStringIndex(text, -1) {
		flush(loc[0])
		seq := text[loc[0]:loc[1]]
		last = loc[1]
		if !strings.HasPrefix(seq, "\x1b[") || !strings.HasSuffix(seq, "m") {
			continue // cursor movement and friends: drop, no colour change
		}
		params := seq[2 : len(seq)-1]
		closeSpan()
		if params == "" || params == "0" {
			current = ""
		} else {
			current = params
		}
		start = runeLen
	}
	flush(len(text))
	closeSpan()
	return sb.String(), spans
}

// Item is one line of input.
type Item struct {
	Index int32
	// Raw is what gets printed when the item is selected.
	Raw string
	// display is what the user sees (--with-nth applied, ANSI stripped).
	display string
	// match is what patterns run against (--nth applied).
	match algo.Chars
	// matchOffset maps a match position back to a display position, or is -1
	// when --nth selected something that is not one contiguous run.
	matchOffset int
	Colors      []Span
}

func (i *Item) Display() string   { return i.display }
func (i *Item) Match() algo.Chars { return i.match }
func (i *Item) MatchOffset() int  { return i.matchOffset }

// Builder turns raw lines into Items according to the field and colour
// options, which are fixed for the whole run.
type Builder struct {
	Delim   Delimiter
	Nth     []Range
	WithNth []Range
	Ansi    bool
}

func (b *Builder) Build(index int32, raw string) *Item {
	it := &Item{Index: index, Raw: raw}
	display := raw
	if b.Ansi {
		stripped, spans := StripAnsi(raw)
		display = stripped
		it.Colors = spans
	}
	if len(b.WithNth) > 0 {
		// The recorded colour spans refer to the untransformed text, so they
		// cannot survive a rearrangement of the fields.
		display = Transform(Tokenize(display, b.Delim), b.WithNth)
		it.Colors = nil
	}
	it.display = display

	matchText := display
	if len(b.Nth) > 0 {
		matchText = Transform(Tokenize(display, b.Delim), b.Nth)
	}
	if matchText == display {
		it.match = algo.NewChars(display)
		it.matchOffset = 0
	} else {
		it.match = algo.NewChars(matchText)
		if idx := strings.Index(display, matchText); idx >= 0 && matchText != "" {
			it.matchOffset = len([]rune(display[:idx]))
		} else {
			it.matchOffset = -1
		}
	}
	return it
}
