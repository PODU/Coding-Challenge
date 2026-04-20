// Package interp is a tree-walking interpreter for the supported awk subset.
package interp

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"os"
	"regexp"
	"strings"

	"goawk/ast"
)

// Interp runs a parsed awk program.
type Interp struct {
	prog *ast.Program

	nr, fnr               int
	fs, ofs, ors, rs      string
	subsep, convfmt, ofmt string
	filename              string
	rstart, rlength       int

	line        string
	fields      []string
	nf          int
	fieldsValid bool

	globals map[string]value
	arrays  map[string]map[string]value

	frames []*frame

	returnValue value

	regexCache map[string]*regexp.Regexp

	stdin    io.Reader
	stdoutW  io.Writer
	output   *bufio.Writer
	errout   io.Writer
	outFiles map[string]*outStream
	inFiles  map[string]*inStream

	// Position in ARGV while reading the main input.
	argIndex        int
	curReader       *recordReader
	curCloser       io.Closer
	processedAnyArg bool
	usedStdin       bool

	rng      *rand.Rand
	prevSeed float64

	rangeActive map[*ast.Rule]bool

	exitCode int
}

type frame struct {
	scalars map[string]value
	arrays  map[string]map[string]value
	isLocal map[string]bool
}

// ctrl is how break/continue/next/exit/return propagate out of exec.
type ctrl int

const (
	ctrlNone ctrl = iota
	ctrlBreak
	ctrlContinue
	ctrlReturn
)

type nextSignal struct{}
type nextFileSignal struct{}
type exitSignal struct{ code int }
type awkError struct{ msg string }

func (e awkError) Error() string { return e.msg }

func (p *Interp) fatalf(format string, args ...any) {
	panic(awkError{msg: fmt.Sprintf(format, args...)})
}

// New creates an interpreter for the given program.
func New(prog *ast.Program) *Interp {
	p := &Interp{
		prog:       prog,
		ofs:        " ",
		ors:        "\n",
		fs:         " ",
		rs:         "\n",
		subsep:     "\034",
		convfmt:    "%.6g",
		ofmt:       "%.6g",
		globals:    map[string]value{},
		arrays:     map[string]map[string]value{},
		regexCache: map[string]*regexp.Regexp{},
		outFiles:   map[string]*outStream{},
		inFiles:    map[string]*inStream{},
		rng:        rand.New(rand.NewSource(0)),
	}
	p.arrays["ENVIRON"] = map[string]value{}
	for _, kv := range os.Environ() {
		if eq := strings.IndexByte(kv, '='); eq >= 0 {
			p.arrays["ENVIRON"][kv[:eq]] = strnum(kv[eq+1:])
		}
	}
	return p
}

// SetVarString assigns a variable from a textual value (used by -v and
// command-line var=val), processing the value as a numeric string.
func (p *Interp) SetVarString(name, val string) {
	p.setVar(name, strnum(val))
}

// Run executes the program. args are ARGV[1:]; the function returns the exit
// status.
func (p *Interp) Run(args []string, stdin io.Reader, stdout, stderr io.Writer) (code int) {
	p.stdin = stdin
	p.stdoutW = stdout
	p.output = bufio.NewWriter(stdout)
	p.errout = stderr

	// ARGV / ARGC
	argv := map[string]value{"0": str("awk")}
	for i, a := range args {
		argv[fmt.Sprint(i+1)] = strnum(a)
	}
	p.arrays["ARGV"] = argv
	p.globals["ARGC"] = num(float64(len(args) + 1))
	p.argIndex = 1

	defer func() {
		p.cleanup()
		if r := recover(); r != nil {
			if ae, ok := r.(awkError); ok {
				fmt.Fprintf(p.errout, "goawk: %s\n", ae.msg)
				code = 2
				return
			}
			panic(r)
		}
	}()

	p.exitCode = 0
	exited := p.runWithExit(func() {
		p.runBegin()
		if p.needMainLoop() {
			p.mainLoop()
		}
	})
	// END rules always run (unless a fatal error occurred).
	if !exited.fatal {
		p.runWithExit(func() { p.runEnd() })
	}
	return p.exitCode
}

type runResult struct{ fatal bool }

// runWithExit runs fn, catching an exit signal and recording the exit code.
func (p *Interp) runWithExit(fn func()) runResult {
	res := runResult{}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if es, ok := r.(exitSignal); ok {
					p.exitCode = es.code
					return
				}
				panic(r)
			}
		}()
		fn()
	}()
	return res
}

func (p *Interp) needMainLoop() bool {
	return len(p.prog.Rules) > 0 || len(p.prog.End) > 0
}

func (p *Interp) runBegin() {
	for _, blk := range p.prog.Begin {
		p.execBlock(blk)
	}
}

func (p *Interp) runEnd() {
	for _, blk := range p.prog.End {
		p.execBlock(blk)
	}
}

func (p *Interp) mainLoop() {
	for {
		rec, ok := p.nextMainRecord()
		if !ok {
			break
		}
		p.nr++
		p.fnr++
		p.setRecord(rec)
		p.runRules()
	}
}

func (p *Interp) runRules() {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case nextSignal:
				return
			case nextFileSignal:
				if p.curCloser != nil {
					p.curCloser.Close()
				}
				p.curReader = nil
				p.curCloser = nil
				return
			default:
				panic(r)
			}
		}
	}()
	for _, rule := range p.prog.Rules {
		if p.matchRule(rule) {
			if rule.Action == nil {
				p.writeOutput(p.line + p.ors)
			} else {
				p.execBlock(rule.Action)
			}
		}
	}
}

func (p *Interp) matchRule(rule *ast.Rule) bool {
	switch pat := rule.Pattern.(type) {
	case nil:
		return true
	case *ast.ExprPattern:
		return p.eval(pat.X).boolean()
	case *ast.RangePattern:
		return p.matchRange(rule, pat)
	}
	return false
}

// matchRange evaluates a start,stop range pattern for the current record.
func (p *Interp) matchRange(rule *ast.Rule, pat *ast.RangePattern) bool {
	if p.rangeActive == nil {
		p.rangeActive = map[*ast.Rule]bool{}
	}
	if !p.rangeActive[rule] {
		if !p.eval(pat.Start).boolean() {
			return false
		}
		p.rangeActive[rule] = true
	}
	// Checked on the starting line too: one line can open and close a range.
	if p.eval(pat.Stop).boolean() {
		p.rangeActive[rule] = false
	}
	return true
}

func (p *Interp) cleanup() {
	if p.output != nil {
		p.output.Flush()
	}
	for _, s := range p.outFiles {
		s.Close()
	}
	for _, s := range p.inFiles {
		s.Close()
	}
}

func (p *Interp) topFrame() *frame {
	if len(p.frames) == 0 {
		return nil
	}
	return p.frames[len(p.frames)-1]
}

func (p *Interp) getVar(name string) value {
	if f := p.topFrame(); f != nil && f.isLocal[name] {
		return f.scalars[name]
	}
	switch name {
	case "NR":
		return num(float64(p.nr))
	case "NF":
		p.ensureSplit()
		return num(float64(p.nf))
	case "FNR":
		return num(float64(p.fnr))
	case "FS":
		return str(p.fs)
	case "OFS":
		return str(p.ofs)
	case "ORS":
		return str(p.ors)
	case "RS":
		return str(p.rs)
	case "SUBSEP":
		return str(p.subsep)
	case "FILENAME":
		return str(p.filename)
	case "RSTART":
		return num(float64(p.rstart))
	case "RLENGTH":
		return num(float64(p.rlength))
	case "CONVFMT":
		return str(p.convfmt)
	case "OFMT":
		return str(p.ofmt)
	}
	if v, ok := p.globals[name]; ok {
		return v
	}
	return uninit
}

func (p *Interp) setVar(name string, v value) {
	if f := p.topFrame(); f != nil && f.isLocal[name] {
		f.scalars[name] = v
		return
	}
	switch name {
	case "NR":
		p.nr = int(v.num())
		return
	case "NF":
		p.setNF(int(v.num()))
		return
	case "FNR":
		p.fnr = int(v.num())
		return
	case "FS":
		p.fs = v.str(p.convfmt)
		return
	case "OFS":
		p.ofs = v.str(p.convfmt)
		return
	case "ORS":
		p.ors = v.str(p.convfmt)
		return
	case "RS":
		p.rs = v.str(p.convfmt)
		return
	case "SUBSEP":
		p.subsep = v.str(p.convfmt)
		return
	case "FILENAME":
		p.filename = v.str(p.convfmt)
		return
	case "RSTART":
		p.rstart = int(v.num())
		return
	case "RLENGTH":
		p.rlength = int(v.num())
		return
	case "CONVFMT":
		p.convfmt = v.str(p.convfmt)
		return
	case "OFMT":
		p.ofmt = v.str(p.convfmt)
		return
	}
	p.globals[name] = v
}

// resolveArray returns the underlying map for an array variable, creating it
// if necessary.
func (p *Interp) resolveArray(name string) map[string]value {
	if f := p.topFrame(); f != nil && f.isLocal[name] {
		m := f.arrays[name]
		if m == nil {
			m = map[string]value{}
			f.arrays[name] = m
		}
		return m
	}
	m := p.arrays[name]
	if m == nil {
		m = map[string]value{}
		p.arrays[name] = m
	}
	return m
}

func (p *Interp) writeOutput(s string) {
	p.output.WriteString(s)
}
