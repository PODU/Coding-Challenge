package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// runVM compiles src at the given opt level and runs it over the VM, returning
// captured stdout.
func runVM(t *testing.T, src string, input string, level OptLevel, eof EOFMode) (string, error) {
	t.Helper()
	prog, err := Compile([]byte(src), level)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	tape := NewTape(30000, false)
	vm := NewVM(prog, tape, strings.NewReader(input), &out, eof)
	err = vm.Run()
	return out.String(), err
}

// runNaive runs src over the naive interpreter, returning captured stdout.
func runNaive(t *testing.T, src string, input string, eof EOFMode) (string, error) {
	t.Helper()
	var out bytes.Buffer
	tape := NewTape(30000, false)
	it := NewInterpreter(tape, strings.NewReader(input), &out, eof)
	err := it.Run([]byte(src))
	return out.String(), err
}

// allOptLevels is the set of optimization levels exercised by equivalence tests.
var allOptLevels = []OptLevel{OptNone, OptBasic, OptFull}

func mustReadProgram(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("programs/" + name)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return string(b)
}

// --- Canonical programs: byte-exact output ---------------------------------

func TestCanonicalPrograms(t *testing.T) {
	helloSrc := mustReadProgram(t, "hello.bf")
	hello6Src := mustReadProgram(t, "hello6.bf")

	cases := []struct {
		name  string
		src   string
		input string
		want  string
	}{
		{"HelloWorld", helloSrc, "", "Hello World!\n"},
		{"Hello6", hello6Src, "", "Hello!"},
		{"CatBasic", ",[.,]", "meow 123\n", "meow 123\n"},
		{"CatEmpty", ",[.,]", "", ""},
		// The classic cat uses 0 as its terminator, so binary data must be
		// non-zero to round-trip; a leading NUL would stop it immediately.
		{"CatBinary", ",[.,]", "\x01\x02\xfe\xff", "\x01\x02\xfe\xff"},
		{"EchoArithmetic", ",+.", "A", "B"}, // read 'A'(65), +1 -> 'B'
		{"EmptyProgram", "", "", ""},
		{"CommentsIgnored", "comments! ++++ +++ +++ . here", "", "\n"}, // 10 pluses -> 0x0A
	}

	for _, tc := range cases {
		for _, lvl := range allOptLevels {
			lvl := lvl
			tc := tc
			t.Run(tc.name+"/opt="+optName(lvl), func(t *testing.T) {
				got, err := runVM(t, tc.src, tc.input, lvl, EOFZero)
				if err != nil {
					t.Fatalf("vm error: %v", err)
				}
				if got != tc.want {
					t.Fatalf("vm output mismatch\n got: %q\nwant: %q", got, tc.want)
				}
			})
		}
		t.Run(tc.name+"/naive", func(t *testing.T) {
			got, err := runNaive(t, tc.src, tc.input, EOFZero)
			if err != nil {
				t.Fatalf("naive error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("naive output mismatch\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func optName(l OptLevel) string {
	switch l {
	case OptNone:
		return "none"
	case OptBasic:
		return "basic"
	default:
		return "full"
	}
}

// --- VM / naive / opt-level equivalence ------------------------------------

func TestOptimizerEquivalence(t *testing.T) {
	// A program touching arithmetic, nested loops, clears, scans and copy loops.
	progs := []struct {
		name  string
		src   string
		input string
	}{
		{"hello", mustReadProgram(t, "hello.bf"), ""},
		{"clearLoop", "+++++++[-]+++.", ""},
		{"copyLoop", "+++++[->++>+++<<]>.>.", ""},
		{"scanRight", "+>+>+>+>>+++[<]<-.", ""}, // build a barrier and scan
		{"nested", "++[>++[>+<-]<-]>>.", ""},
		{"cat", ",[.,]", "hello\x00world"},
	}
	for _, p := range progs {
		base, err := runNaive(t, p.src, p.input, EOFZero)
		if err != nil {
			t.Fatalf("%s naive: %v", p.name, err)
		}
		for _, lvl := range allOptLevels {
			got, err := runVM(t, p.src, p.input, lvl, EOFZero)
			if err != nil {
				t.Fatalf("%s opt=%s: %v", p.name, optName(lvl), err)
			}
			if got != base {
				t.Fatalf("%s opt=%s mismatch vs naive\n got: %q\nwant: %q",
					p.name, optName(lvl), got, base)
			}
		}
	}
}

// --- Bracket matching -------------------------------------------------------

func TestBracketErrors(t *testing.T) {
	cases := []struct {
		name       string
		src        string
		wantOffset int
		wantSubstr string
	}{
		{"unmatchedOpen", "+++[>+", 3, "unmatched '['"},
		{"unmatchedClose", "+]", 1, "unmatched ']'"},
		{"closeFirst", "]", 0, "unmatched ']'"},
		{"nestedUnmatched", "[[]", 0, "unmatched '['"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile([]byte(tc.src), OptFull)
			if err == nil {
				t.Fatalf("expected compile error, got nil")
			}
			ce, ok := err.(*CompileError)
			if !ok {
				t.Fatalf("expected *CompileError, got %T", err)
			}
			if ce.Offset != tc.wantOffset {
				t.Fatalf("offset = %d, want %d", ce.Offset, tc.wantOffset)
			}
			if !strings.Contains(ce.Error(), tc.wantSubstr) {
				t.Fatalf("error %q missing %q", ce.Error(), tc.wantSubstr)
			}
		})
	}
}

func TestBalancedAndDeeplyNested(t *testing.T) {
	// 1000 levels of nesting must compile without issues.
	depth := 1000
	src := strings.Repeat("[", depth) + strings.Repeat("]", depth)
	if _, err := Compile([]byte(src), OptFull); err != nil {
		t.Fatalf("deep nesting failed to compile: %v", err)
	}
	// Jump targets must be correct: a simple balanced loop runs the body.
	out, err := runVM(t, "++>+++<[->+<]>.", "", OptNone, EOFZero)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out != "\x05" { // 3 + 2 = 5
		t.Fatalf("jump-table loop wrong: got %q", out)
	}
}

// --- Cell wrap / arithmetic -------------------------------------------------

func TestCellWrap(t *testing.T) {
	// 256 increments returns a cell to 0.
	src := strings.Repeat("+", 256) + "."
	out, err := runVM(t, src, "", OptNone, EOFZero)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out != "\x00" {
		t.Fatalf("256x+ should wrap to 0, got %q", out)
	}

	// One decrement from 0 yields 255.
	out, err = runVM(t, "-.", "", OptNone, EOFZero)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out != "\xff" {
		t.Fatalf("0-1 should wrap to 255, got %q", out)
	}

	// Same under the optimizer (Add folding must also wrap).
	out, err = runVM(t, src, "", OptFull, EOFZero)
	if err != nil || out != "\x00" {
		t.Fatalf("opt full wrap: out=%q err=%v", out, err)
	}
}

// --- Pointer bounds ---------------------------------------------------------

func TestPointerOOB(t *testing.T) {
	if _, err := runVM(t, "<", "", OptNone, EOFZero); err == nil {
		t.Fatalf("expected runtime error for left OOB")
	} else if _, ok := err.(*RuntimeError); !ok {
		t.Fatalf("expected *RuntimeError, got %T", err)
	}

	// Moving beyond the right edge of a tiny tape.
	prog, _ := Compile([]byte(">>>"), OptNone)
	tape := NewTape(2, false)
	var out bytes.Buffer
	vm := NewVM(prog, tape, strings.NewReader(""), &out, EOFZero)
	if err := vm.Run(); err == nil {
		t.Fatalf("expected runtime error for right OOB")
	}
}

func TestWrapPtr(t *testing.T) {
	// With a circular tape of size 3, '<' from 0 lands on cell 2.
	prog, _ := Compile([]byte("<+."), OptNone)
	tape := NewTape(3, true)
	var out bytes.Buffer
	vm := NewVM(prog, tape, strings.NewReader(""), &out, EOFZero)
	if err := vm.Run(); err != nil {
		t.Fatalf("wrap-ptr run: %v", err)
	}
	if out.String() != "\x01" {
		t.Fatalf("wrap-ptr: got %q", out.String())
	}
	if tape.dp != 2 {
		t.Fatalf("wrap-ptr: dp = %d, want 2", tape.dp)
	}
}

// --- EOF behavior -----------------------------------------------------------

func TestEOFModes(t *testing.T) {
	// Program: read into cell, then output it. With empty input we hit EOF.
	src := ",."
	cases := []struct {
		mode EOFMode
		want string
	}{
		{EOFZero, "\x00"},
		{EOFNegOne, "\xff"},
	}
	for _, tc := range cases {
		got, err := runVM(t, src, "", OptFull, tc.mode)
		if err != nil {
			t.Fatalf("eof run: %v", err)
		}
		if got != tc.want {
			t.Fatalf("eof mode %d: got %q want %q", tc.mode, got, tc.want)
		}
	}

	// Unchanged: preload the cell with 7, then a ',' at EOF must leave it.
	got, err := runVM(t, "+++++++,.", "", OptFull, EOFUnchanged)
	if err != nil {
		t.Fatalf("eof unchanged run: %v", err)
	}
	if got != "\x07" {
		t.Fatalf("eof unchanged: got %q want \\x07", got)
	}
}

func TestCatTerminatesOnEmpty(t *testing.T) {
	got, err := runVM(t, ",[.,]", "", OptFull, EOFZero)
	if err != nil {
		t.Fatalf("cat empty: %v", err)
	}
	if got != "" {
		t.Fatalf("cat empty: got %q", got)
	}
}

// --- Optimizer shape assertions (via bytecode) ------------------------------

func TestOptimizerEmitsSpecializedOps(t *testing.T) {
	// [-] -> single Clear.
	prog, err := Compile([]byte("+++[-]"), OptFull)
	if err != nil {
		t.Fatal(err)
	}
	if !hasOp(prog, OpClear) || countOp(prog, OpJumpIfZero) != 0 {
		t.Fatalf("[-] should compile to Clear with no jumps: %v", dumpString(prog))
	}

	// +++++ -> single Add(5).
	prog, _ = Compile([]byte("+++++"), OptFull)
	if got := firstArg(prog, OpAdd); got != 5 {
		t.Fatalf("+++++ should fold to Add(5), got Add(%d)", got)
	}

	// [>] -> ScanRight.
	prog, _ = Compile([]byte("[>]"), OptFull)
	if !hasOp(prog, OpScanRight) {
		t.Fatalf("[>] should compile to ScanRight: %v", dumpString(prog))
	}

	// Copy loop -> AddTo + Clear.
	prog, _ = Compile([]byte("[->+<]"), OptFull)
	if !hasOp(prog, OpAddTo) || !hasOp(prog, OpClear) {
		t.Fatalf("[->+<] should compile to AddTo+Clear: %v", dumpString(prog))
	}

	// At opt=none, no specialized ops appear.
	prog, _ = Compile([]byte("[-]+++++"), OptNone)
	if hasOp(prog, OpClear) || hasOp(prog, OpAddTo) {
		t.Fatalf("opt=none must not emit specialized ops: %v", dumpString(prog))
	}
}

func hasOp(prog []Instruction, op OpCode) bool { return countOp(prog, op) > 0 }

func countOp(prog []Instruction, op OpCode) int {
	n := 0
	for _, ins := range prog {
		if ins.Op == op {
			n++
		}
	}
	return n
}

func firstArg(prog []Instruction, op OpCode) int {
	for _, ins := range prog {
		if ins.Op == op {
			return ins.Arg
		}
	}
	return -999999
}

func dumpString(prog []Instruction) string {
	var b strings.Builder
	DumpBytecode(&b, prog)
	return b.String()
}

// --- CLI surface ------------------------------------------------------------

func TestRunCLIExitCodes(t *testing.T) {
	cases := []struct {
		name string
		args []string
		in   string
		want int
	}{
		{"missingFile", []string{"does-not-exist.bf"}, "", exitIO},
		{"unbalanced", []string{"-"}, "+++[", exitCompile},
		{"badOpt", []string{"--opt", "wat", "-"}, "", exitCompile},
		{"badEOF", []string{"--eof", "wat", "-"}, "", exitCompile},
		{"runtimeOOB", []string{"-"}, "<", exitRuntime},
		{"helpFlag", []string{"-h"}, "", exitOK},
		{"ok", []string{"-"}, "+++.", exitOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			code := run(tc.args, strings.NewReader(tc.in), &out, &errb)
			if code != tc.want {
				t.Fatalf("exit = %d, want %d (stderr: %s)", code, tc.want, errb.String())
			}
		})
	}
}

func TestRunCLIHelloFromStdin(t *testing.T) {
	var out, errb bytes.Buffer
	src := mustReadProgram(t, "hello.bf")
	code := run([]string{"-"}, strings.NewReader(src), &out, &errb)
	if code != exitOK {
		t.Fatalf("exit = %d, stderr=%s", code, errb.String())
	}
	if out.String() != "Hello World!\n" {
		t.Fatalf("got %q", out.String())
	}
}

func TestRunCLIDump(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"--dump", "-"}, strings.NewReader("+++[-]"), &out, &errb)
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out.String(), "Clear") {
		t.Fatalf("dump missing Clear: %s", out.String())
	}
}

func TestREPL(t *testing.T) {
	// Two lines sharing tape state: first builds 'A' in cell0, second prints it.
	in := "++++++++[>++++++++<-]>+\n.\n"
	var out, errb bytes.Buffer
	code := run([]string{"repl"}, strings.NewReader(in), &out, &errb)
	if code != exitOK {
		t.Fatalf("repl exit = %d, stderr=%s", code, errb.String())
	}
	// cell0 after line 1 = 8*8+1 = 65 = 'A'; line 2 prints it.
	if out.String() != "A" {
		t.Fatalf("repl output = %q, want \"A\"", out.String())
	}
}
