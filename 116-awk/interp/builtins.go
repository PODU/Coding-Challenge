package interp

import (
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"goawk/ast"
)

func (p *Interp) evalCall(n *ast.Call) value {
	if _, ok := p.prog.Functions[n.Name]; ok {
		return p.callUser(n)
	}
	switch n.Name {
	case "length":
		return p.bLength(n.Args)
	case "substr":
		return p.bSubstr(n.Args)
	case "index":
		return p.bIndex(n.Args)
	case "split":
		return p.bSplit(n.Args)
	case "sub":
		return p.bSubGsub(n.Args, false)
	case "gsub":
		return p.bSubGsub(n.Args, true)
	case "match":
		return p.bMatch(n.Args)
	case "sprintf":
		return p.bSprintf(n.Args)
	case "tolower":
		return str(strings.ToLower(p.arg(n.Args, 0).str(p.convfmt)))
	case "toupper":
		return str(strings.ToUpper(p.arg(n.Args, 0).str(p.convfmt)))
	case "sin":
		return num(math.Sin(p.arg(n.Args, 0).num()))
	case "cos":
		return num(math.Cos(p.arg(n.Args, 0).num()))
	case "atan2":
		return num(math.Atan2(p.arg(n.Args, 0).num(), p.arg(n.Args, 1).num()))
	case "exp":
		return num(math.Exp(p.arg(n.Args, 0).num()))
	case "log":
		return num(math.Log(p.arg(n.Args, 0).num()))
	case "sqrt":
		return num(math.Sqrt(p.arg(n.Args, 0).num()))
	case "int":
		return num(math.Trunc(p.arg(n.Args, 0).num()))
	case "rand":
		return num(p.rng.Float64())
	case "srand":
		return p.bSrand(n.Args)
	case "system":
		return p.bSystem(n.Args)
	case "close":
		return p.bClose(n.Args)
	case "fflush":
		return p.bFflush(n.Args)
	}
	p.fatalf("calling undefined function %s", n.Name)
	return uninit
}

func (p *Interp) arg(args []ast.Expr, i int) value {
	if i < len(args) {
		return p.eval(args[i])
	}
	return uninit
}

func (p *Interp) bLength(args []ast.Expr) value {
	if len(args) == 0 {
		return num(float64(len(p.line)))
	}
	if v, ok := args[0].(*ast.Var); ok && p.isArrayVar(v.Name) {
		return num(float64(len(p.resolveArray(v.Name))))
	}
	return num(float64(len(p.eval(args[0]).str(p.convfmt))))
}

// isArrayVar reports whether name currently refers to an existing array.
func (p *Interp) isArrayVar(name string) bool {
	if f := p.topFrame(); f != nil && f.isLocal[name] {
		_, ok := f.arrays[name]
		return ok
	}
	_, ok := p.arrays[name]
	return ok
}

func (p *Interp) bSubstr(args []ast.Expr) value {
	s := p.arg(args, 0).str(p.convfmt)
	m := p.arg(args, 1).num()
	hasN := len(args) >= 3
	var nLen float64
	if hasN {
		nLen = p.arg(args, 2).num()
	}
	return str(substr(s, m, nLen, hasN))
}

func substr(s string, mF, nF float64, hasN bool) string {
	length := len(s)
	m := int(math.Trunc(mF))
	if m < 1 {
		m = 1
	}
	if m > length {
		return ""
	}
	if !hasN {
		return s[m-1:]
	}
	n := int(math.Trunc(nF))
	if n < 0 {
		n = 0
	}
	end := m - 1 + n
	if end > length {
		end = length
	}
	if end < m-1 {
		end = m - 1
	}
	return s[m-1 : end]
}

func (p *Interp) bIndex(args []ast.Expr) value {
	s := p.arg(args, 0).str(p.convfmt)
	t := p.arg(args, 1).str(p.convfmt)
	return num(float64(strings.Index(s, t) + 1))
}

func (p *Interp) bSplit(args []ast.Expr) value {
	s := p.arg(args, 0).str(p.convfmt)
	v, ok := args[1].(*ast.Var)
	if !ok {
		p.fatalf("split: second argument must be an array")
	}
	arr := p.resolveArray(v.Name)
	for k := range arr {
		delete(arr, k)
	}
	var parts []string
	if len(args) >= 3 {
		fs := p.fieldSepArg(args[2])
		parts = p.splitWith(s, fs)
	} else {
		parts = p.splitWith(s, p.fs)
	}
	for i, part := range parts {
		arr[strconv.Itoa(i+1)] = strnum(part)
	}
	return num(float64(len(parts)))
}

// fieldSepArg returns the field separator string for split's 3rd argument. A
// regex literal is treated as its source pattern.
func (p *Interp) fieldSepArg(e ast.Expr) string {
	if r, ok := e.(*ast.Regex); ok {
		return r.Pattern
	}
	return p.eval(e).str(p.convfmt)
}

// splitWith splits s using separator fs (handles empty=per-char).
func (p *Interp) splitWith(s, fs string) []string {
	if s == "" {
		return nil
	}
	if fs == "" {
		out := make([]string, 0, len(s))
		for _, r := range s {
			out = append(out, string(r))
		}
		return out
	}
	return p.splitString(s, fs, false)
}

func (p *Interp) bMatch(args []ast.Expr) value {
	s := p.arg(args, 0).str(p.convfmt)
	re := p.evalRegexExpr(args[1])
	loc := re.FindStringIndex(s)
	if loc == nil {
		p.rstart = 0
		p.rlength = -1
		return num(0)
	}
	p.rstart = loc[0] + 1
	p.rlength = loc[1] - loc[0]
	return num(float64(p.rstart))
}

func (p *Interp) bSprintf(args []ast.Expr) value {
	if len(args) == 0 {
		return str("")
	}
	format := p.eval(args[0]).str(p.convfmt)
	vals := make([]value, len(args)-1)
	for i := 1; i < len(args); i++ {
		vals[i-1] = p.eval(args[i])
	}
	return str(p.sprintf(format, vals))
}

func (p *Interp) bSubGsub(args []ast.Expr, global bool) value {
	re := p.evalRegexExpr(args[0])
	repl := p.arg(args, 1).str(p.convfmt)
	var target ast.Expr = &ast.Field{Index: &ast.Num{Value: 0}}
	if len(args) >= 3 {
		target = args[2]
	}
	orig := p.evalLvalue(target).str(p.convfmt)
	result, count := substitute(re, orig, repl, global)
	if count > 0 {
		p.assignTo(target, str(result))
	}
	return num(float64(count))
}

// substitute performs sub/gsub replacement, handling & and \& in the
// replacement text and empty-match advancement.
func substitute(re *regexp.Regexp, s, repl string, global bool) (string, int) {
	var b strings.Builder
	count := 0
	pos := 0
	for pos <= len(s) {
		loc := re.FindStringIndex(s[pos:])
		if loc == nil {
			break
		}
		start := pos + loc[0]
		end := pos + loc[1]
		b.WriteString(s[pos:start])
		b.WriteString(expandRepl(repl, s[start:end]))
		count++
		if end == start {
			// empty match: emit one character and advance
			if end < len(s) {
				b.WriteByte(s[end])
			}
			pos = end + 1
		} else {
			pos = end
		}
		if !global {
			break
		}
	}
	if pos < len(s) {
		b.WriteString(s[pos:])
	} else if pos == len(s) {
		// nothing remaining
	}
	if count == 0 {
		return s, 0
	}
	return b.String(), count
}

func expandRepl(repl, matched string) string {
	var b strings.Builder
	for i := 0; i < len(repl); i++ {
		c := repl[i]
		if c == '\\' && i+1 < len(repl) {
			nxt := repl[i+1]
			if nxt == '&' {
				b.WriteByte('&')
				i++
				continue
			}
			if nxt == '\\' {
				b.WriteByte('\\')
				i++
				continue
			}
			b.WriteByte('\\')
			continue
		}
		if c == '&' {
			b.WriteString(matched)
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func (p *Interp) bSrand(args []ast.Expr) value {
	prev := p.prevSeed
	var seed float64
	if len(args) >= 1 {
		seed = p.eval(args[0]).num()
	} else {
		seed = float64(time.Now().Unix())
	}
	p.prevSeed = seed
	p.rng = rand.New(rand.NewSource(int64(seed)))
	return num(prev)
}
