package main

import "regexp"

// Action is a single editing operation applied to the pattern space.
type Action interface {
	Execute(s *State)
}

// Command is an action with an optional address. A nil Addr selects every line.
type Command struct {
	Addr   Address
	Neg    bool // the address was followed by `!`
	Action Action
}

// matches must be called exactly once per line per command, even when negated,
// because range addresses update their state on every call.
func (c *Command) matches(s *State) bool {
	m := c.Addr == nil || c.Addr.Matches(s)
	return m != c.Neg
}

// subst is the `s/regex/replacement/flags` command.
type subst struct {
	re     *regexp.Regexp
	repl   string // in Regexp.Expand template syntax
	global bool
	n      int // 1-based occurrence to start at; 0 means the first
	print  bool
}

func (sub *subst) Execute(s *State) {
	matches := sub.re.FindAllStringSubmatchIndex(s.Line, -1)
	if matches == nil {
		return
	}
	start := sub.n
	if start == 0 {
		start = 1
	}

	var b []byte
	last := 0
	replaced := false
	for i, m := range matches {
		occ := i + 1
		if occ < start || (!sub.global && occ > start) {
			continue
		}
		b = append(b, s.Line[last:m[0]]...)
		b = sub.re.ExpandString(b, sub.repl, s.Line, m)
		last = m[1]
		replaced = true
		if !sub.global {
			break
		}
	}
	if !replaced {
		return
	}
	b = append(b, s.Line[last:]...)
	s.Line = string(b)
	s.Subst = true
	if sub.print {
		s.Out.WriteString(s.Line)
		s.Out.WriteByte('\n')
	}
}

// printCmd is `p`.
type printCmd struct{}

func (printCmd) Execute(s *State) {
	s.Out.WriteString(s.Line)
	s.Out.WriteByte('\n')
}

// deleteCmd is `d`: end the cycle without auto-printing.
type deleteCmd struct{}

func (deleteCmd) Execute(s *State) {
	s.Deleted = true
}

// appendBlank is `G`. With no hold space it just appends a newline, which
// double-spaces the output.
type appendBlank struct{}

func (appendBlank) Execute(s *State) {
	s.Line += "\n"
}
