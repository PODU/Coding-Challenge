package cli

import (
	"fmt"
	"strconv"
	"strings"

	"ccfzf/internal/item"
	"ccfzf/internal/pattern"
)

// Error is a usage error. main turns it into exit code 2.
type Error struct{ Msg string }

func (e *Error) Error() string { return e.Msg }

func errf(format string, args ...any) error {
	return &Error{fmt.Sprintf(format, args...)}
}

// Parse reads the option list. Later options win, which is what makes
// FZF_DEFAULT_OPTS usable: it is parsed first and the command line overrides
// it.
func Parse(args []string) (*Options, error) {
	opts := Defaults()
	if err := parseInto(opts, args); err != nil {
		return nil, err
	}
	return opts, nil
}

// ParseWithDefaults parses the environment defaults first, then args.
func ParseWithDefaults(defaultOpts string, args []string) (*Options, error) {
	opts := Defaults()
	if strings.TrimSpace(defaultOpts) != "" {
		if err := parseInto(opts, SplitArgs(defaultOpts)); err != nil {
			return nil, errf("FZF_DEFAULT_OPTS: %s", err)
		}
	}
	if err := parseInto(opts, args); err != nil {
		return nil, err
	}
	return opts, nil
}

func parseInto(opts *Options, args []string) error {
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// A flag may carry its value as --flag=value; split it off once here.
		name, inlineValue, hasInline := arg, "", false
		if strings.HasPrefix(arg, "--") {
			if eq := strings.Index(arg, "="); eq > 0 {
				name, inlineValue, hasInline = arg[:eq], arg[eq+1:], true
			}
		}
		// Short options may carry their value attached: -qfoo.
		if len(arg) > 2 && arg[0] == '-' && arg[1] != '-' &&
			strings.ContainsRune("qfnd", rune(arg[1])) {
			name, inlineValue, hasInline = arg[:2], arg[2:], true
		}

		value := func() (string, error) {
			if hasInline {
				return inlineValue, nil
			}
			if i+1 >= len(args) {
				return "", errf("%s requires an argument", name)
			}
			i++
			return args[i], nil
		}

		var err error
		switch name {
		case "-x", "--extended":
			opts.Extended = true
		case "+x", "--no-extended":
			opts.Extended = false
		case "-e", "--exact":
			opts.Exact = true
		case "+e", "--no-exact":
			opts.Exact = false
		case "-i", "--ignore-case":
			opts.Case = pattern.CaseIgnore
		case "+i", "--no-ignore-case":
			opts.Case = pattern.CaseRespect
		case "--smart-case":
			opts.Case = pattern.CaseSmart
		case "--algo":
			var v string
			if v, err = value(); err == nil {
				switch v {
				case "v1":
					opts.AlgoV1 = true
				case "v2":
					opts.AlgoV1 = false
				default:
					err = errf("invalid algorithm: %s (want v1 or v2)", v)
				}
			}
		case "-n", "--nth":
			var v string
			if v, err = value(); err == nil {
				opts.Nth, err = item.ParseNth(v)
			}
		case "--with-nth":
			var v string
			if v, err = value(); err == nil {
				opts.WithNth, err = item.ParseNth(v)
			}
		case "-d", "--delimiter":
			var v string
			if v, err = value(); err == nil {
				opts.Delim = item.ParseDelimiter(v)
			}
		case "--tiebreak":
			var v string
			if v, err = value(); err == nil {
				opts.Tiebreak, err = parseTiebreak(v)
			}
		case "--sort":
			opts.Sort = true
		case "--no-sort":
			opts.Sort = false
		case "--tac":
			opts.Tac = true
		case "--no-tac":
			opts.Tac = false
		case "-m", "--multi":
			opts.Multi = -1 // unlimited
			if hasInline {
				var n int
				if n, err = strconv.Atoi(inlineValue); err != nil || n <= 0 {
					err = errf("invalid --multi value: %s", inlineValue)
				} else {
					opts.Multi = n
				}
			}
		case "+m", "--no-multi":
			opts.Multi = 0
		case "-q", "--query":
			opts.Query, err = value()
		case "--prompt":
			opts.Prompt, err = value()
		case "--pointer":
			opts.Pointer, err = value()
		case "--marker":
			opts.Marker, err = value()
		case "--header":
			var v string
			if v, err = value(); err == nil {
				opts.Header = append(opts.Header, strings.Split(v, "\n")...)
			}
		case "--layout":
			var v string
			if v, err = value(); err == nil {
				switch v {
				case "default":
					opts.Layout = LayoutDefault
				case "reverse":
					opts.Layout = LayoutReverse
				case "reverse-list":
					opts.Layout = LayoutReverseList
				default:
					err = errf("invalid layout: %s", v)
				}
			}
		case "--reverse":
			opts.Layout = LayoutReverse
		case "--no-reverse":
			opts.Layout = LayoutDefault
		case "--info":
			var v string
			if v, err = value(); err == nil {
				switch v {
				case "default":
					opts.Info = InfoDefault
				case "inline":
					opts.Info = InfoInline
				case "hidden":
					opts.Info = InfoHidden
				default:
					err = errf("invalid info style: %s", v)
				}
			}
		case "--no-info":
			opts.Info = InfoHidden
		case "--inline-info":
			opts.Info = InfoInline
		case "--height":
			var v string
			if v, err = value(); err == nil {
				opts.Height, err = parseHeight(v)
			}
		case "--no-height":
			opts.Height = Height{}
		case "--preview":
			opts.Preview, err = value()
		case "--preview-window":
			var v string
			if v, err = value(); err == nil {
				opts.PreviewWin, err = parsePreviewWindow(v, opts.PreviewWin)
			}
		case "--ansi":
			opts.Ansi = true
		case "--no-ansi":
			opts.Ansi = false
		case "--read0":
			opts.Read0 = true
		case "--print0":
			opts.Print0 = true
		case "--expect":
			var v string
			if v, err = value(); err == nil {
				for _, k := range strings.Split(v, ",") {
					if k = strings.TrimSpace(k); k != "" {
						opts.Expect[strings.ToLower(k)] = true
					}
				}
			}
		case "-f", "--filter":
			opts.Filter, err = value()
			opts.HasFilter = err == nil
		case "--print-query":
			opts.PrintQuery = true
		case "-1", "--select-1":
			opts.Select1 = true
		case "+1", "--no-select-1":
			opts.Select1 = false
		case "-0", "--exit-0":
			opts.Exit0 = true
		case "+0", "--no-exit-0":
			opts.Exit0 = false
		case "--help", "-h":
			opts.Help = true
		case "--version":
			opts.Version = true
		// Accepted and ignored: these exist so that an FZF_DEFAULT_OPTS written
		// for the real fzf does not abort the program over cosmetics.
		case "--no-mouse", "--no-bold", "--bold", "--cycle", "--no-cycle",
			"--border", "--no-border", "--no-clear", "--black", "--no-hscroll":
		case "--color", "--bind", "--history", "--history-size", "--min-height",
			"--scroll-off", "--margin", "--padding", "--tabstop", "--border-label":
			_, err = value()
		default:
			return errf("unknown option: %s", arg)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func parseTiebreak(spec string) ([]Criterion, error) {
	var out []Criterion
	seen := map[Criterion]bool{}
	for _, part := range strings.Split(spec, ",") {
		var c Criterion
		switch strings.TrimSpace(part) {
		case "length":
			c = ByLength
		case "begin":
			c = ByBegin
		case "end":
			c = ByEnd
		case "index":
			c = ByIndex
		default:
			return nil, errf("invalid tiebreak criterion: %s", part)
		}
		if seen[c] {
			return nil, errf("duplicate tiebreak criterion: %s", part)
		}
		seen[c] = true
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, errf("empty --tiebreak")
	}
	return out, nil
}

func parseHeight(spec string) (Height, error) {
	h := Height{Set: true}
	if strings.HasPrefix(spec, "~") {
		h.Auto = true
		spec = spec[1:]
	}
	if strings.HasSuffix(spec, "%") {
		h.Percent = true
		spec = spec[:len(spec)-1]
	}
	n, err := strconv.Atoi(spec)
	if err != nil || n <= 0 {
		return Height{}, errf("invalid height: %s", spec)
	}
	if h.Percent && n > 100 {
		n = 100
	}
	h.Size = n
	return h, nil
}

func parsePreviewWindow(spec string, base PreviewWindow) (PreviewWindow, error) {
	win := base
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		switch part {
		case "":
		case "up", "top":
			win.Position = PreviewUp
		case "down", "bottom":
			win.Position = PreviewDown
		case "left":
			win.Position = PreviewLeft
		case "right":
			win.Position = PreviewRight
		case "hidden":
			win.Hidden = true
		case "nohidden":
			win.Hidden = false
		case "wrap":
			win.Wrap = true
		case "nowrap":
			win.Wrap = false
		default:
			size := part
			percent := false
			if strings.HasSuffix(size, "%") {
				percent = true
				size = size[:len(size)-1]
			}
			n, err := strconv.Atoi(size)
			if err != nil || n < 0 {
				return base, errf("invalid preview window option: %s", part)
			}
			win.Size, win.Percent = n, percent
		}
	}
	return win, nil
}

// SplitArgs splits a string the way a shell would, honouring single and double
// quotes. Used for FZF_DEFAULT_OPTS.
func SplitArgs(s string) []string {
	var args []string
	var cur strings.Builder
	quote := rune(0)
	started := false
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			started = true
		case r == ' ' || r == '\t' || r == '\n':
			if started {
				args = append(args, cur.String())
				cur.Reset()
				started = false
			}
		default:
			cur.WriteRune(r)
			started = true
		}
	}
	if started {
		args = append(args, cur.String())
	}
	return args
}
