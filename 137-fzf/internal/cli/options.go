// Package cli parses the command line into the options the rest of the
// program reads.
package cli

import (
	"ccfzf/internal/item"
	"ccfzf/internal/pattern"
)

// Criterion is a tiebreaker applied when two items score the same.
type Criterion int

const (
	ByScore Criterion = iota // always first, never configurable
	ByLength
	ByBegin
	ByEnd
	ByIndex
)

type Layout int

const (
	LayoutDefault     Layout = iota // prompt at the bottom, list growing upward
	LayoutReverse                   // prompt at the top, list growing downward
	LayoutReverseList               // prompt at the bottom, list in normal order
)

type InfoStyle int

const (
	InfoDefault InfoStyle = iota // its own line under the prompt
	InfoInline                   // appended to the prompt line
	InfoHidden
)

// Height describes --height=[~]HEIGHT[%].
type Height struct {
	Set     bool
	Size    int
	Percent bool
	Auto    bool // "~": shrink to the number of lines actually needed
}

type PreviewPosition int

const (
	PreviewRight PreviewPosition = iota
	PreviewLeft
	PreviewUp
	PreviewDown
)

type PreviewWindow struct {
	Position PreviewPosition
	Size     int
	Percent  bool
	Hidden   bool
	Wrap     bool
}

type Options struct {
	// Search
	Extended bool
	Exact    bool
	Case     pattern.CaseMode
	AlgoV1   bool
	Nth      []item.Range
	WithNth  []item.Range
	Delim    item.Delimiter
	Tiebreak []Criterion

	// Results
	Sort bool
	Tac  bool

	// Interface
	Multi      int // 0 = single selection, otherwise the maximum
	Query      string
	Prompt     string
	Pointer    string
	Marker     string
	Header     []string
	Layout     Layout
	Info       InfoStyle
	Height     Height
	Preview    string
	PreviewWin PreviewWindow

	// Scripting
	Filter     string
	HasFilter  bool
	PrintQuery bool
	Select1    bool
	Exit0      bool
	Expect     map[string]bool
	Read0      bool
	Print0     bool
	Ansi       bool

	Help    bool
	Version bool
}

func Defaults() *Options {
	return &Options{
		Extended: true,
		Case:     pattern.CaseSmart,
		Delim:    item.DefaultDelimiter(),
		Tiebreak: []Criterion{ByLength},
		Sort:     true,
		Prompt:   "> ",
		Pointer:  ">",
		Marker:   ">",
		Expect:   map[string]bool{},
		PreviewWin: PreviewWindow{
			Position: PreviewRight,
			Size:     50,
			Percent:  true,
			Wrap:     false,
		},
	}
}

// PatternOptions projects the parts of Options the matcher cares about.
func (o *Options) PatternOptions() pattern.Options {
	forward := true
	for _, c := range o.Tiebreak {
		if c == ByEnd {
			// --tiebreak=end wants the last occurrence, so the greedy scan runs
			// backwards.
			forward = false
		}
	}
	return pattern.Options{
		Extended: o.Extended,
		Exact:    o.Exact,
		Case:     o.Case,
		AlgoV1:   o.AlgoV1,
		Forward:  forward,
	}
}

func (o *Options) ItemBuilder() *item.Builder {
	return &item.Builder{Delim: o.Delim, Nth: o.Nth, WithNth: o.WithNth, Ansi: o.Ansi}
}

const Usage = `usage: ccfzf [options]

Reads newline-delimited items from standard input and lets you pick one
interactively. The selection is printed to standard output.

Search
  -x, --extended         Extended-search mode (default)
  +x, --no-extended      Disable extended-search mode
  -e, --exact            Exact match instead of fuzzy
  -i, --ignore-case      Case-insensitive match
  +i, --no-ignore-case   Case-sensitive match
      --algo=TYPE        Fuzzy algorithm: v1 (fast) or v2 (optimal, default)
  -n, --nth=N[,..]       Match only these fields
      --with-nth=N[,..]  Display only these fields
  -d, --delimiter=REGEX  Field separator (default: whitespace)
      --tiebreak=CRI[,..] length, begin, end, index (default: length)

Results
      --no-sort          Keep the input order
      --tac              Reverse the input order

Interface
  -m, --multi[=MAX]      Select multiple items with TAB
  +m, --no-multi         Single selection (default)
      --height=[~]N[%]   Use only N lines instead of the whole screen
      --layout=LAYOUT    default, reverse, reverse-list
      --reverse          Shorthand for --layout=reverse
      --info=STYLE       default, inline, hidden
      --prompt=STR       Input prompt (default: '> ')
      --pointer=STR      Pointer to the current line (default: '>')
      --marker=STR       Marker for selected items (default: '>')
      --header=STR       Fixed header line, repeatable
      --preview=CMD      Preview the current item with CMD ({} is the item)
      --preview-window=POS[,SIZE[%]]
                         up, down, left, right, hidden, wrap

Input/output
      --ansi             Interpret ANSI colour codes in the input
      --read0            Input is NUL-delimited
      --print0           Output is NUL-delimited
      --expect=KEYS      Accept on these keys and print which one was used

Scripting
  -q, --query=STR        Start with this query
  -f, --filter=STR       Filter mode: print matches and exit
      --print-query      Print the query as the first output line
  -1, --select-1         Accept immediately if there is only one match
  -0, --exit-0           Exit immediately if there is no match
      --help             Show this message
      --version          Show the version

Environment
  FZF_DEFAULT_COMMAND    Command to produce the input when stdin is a terminal
  FZF_DEFAULT_OPTS       Default options, parsed before the command line

Exit codes: 0 selected, 1 no match, 2 error, 130 interrupted.
`
