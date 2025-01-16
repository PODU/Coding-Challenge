package main

import (
	"fmt"
	"maps"
	"slices"
)

type OpCode uint8

const (
	OpAdd           OpCode = iota // Arg = signed delta applied to current cell (mod 256)
	OpMove                        // Arg = signed pointer offset
	OpOutput                      // write current cell to output
	OpInput                       // read one byte into current cell
	OpJumpIfZero                  // Arg = target pc; jump if current cell == 0
	OpJumpIfNonZero               // Arg = target pc; jump if current cell != 0
	OpClear                       // set current cell to 0
	OpScanRight                   // move right until current cell == 0
	OpScanLeft                    // move left until current cell == 0
	OpAddTo                       // cells[dp+Arg] += Arg2 * cells[dp]
	OpHalt                        // stop execution
)

type Instruction struct {
	Op   OpCode
	Arg  int
	Arg2 int
}

type OptLevel int

const (
	OptNone  OptLevel = iota // one instruction per command, no folding
	OptBasic                 // run-length collapse of +/- and >/<
	OptFull                  // basic, plus [-], scan and copy loops
)

func ParseOptLevel(s string) (OptLevel, error) {
	switch s {
	case "none":
		return OptNone, nil
	case "basic":
		return OptBasic, nil
	case "full":
		return OptFull, nil
	default:
		return 0, &CompileError{Msg: fmt.Sprintf("invalid --opt level %q (want none|basic|full)", s), Offset: -1}
	}
}

// Compile turns source into bytecode with jump targets already resolved.
// Unbalanced brackets give a CompileError with the byte offset.
func Compile(src []byte, level OptLevel) ([]Instruction, error) {
	tokens := Tokenize(src)
	prog := make([]Instruction, 0, len(tokens)+1)
	var stack []int // JumpIfZero instructions still waiting for their ']'

	i := 0
	for i < len(tokens) {
		c := tokens[i].Cmd
		switch c {
		case '+', '-':
			if level >= OptBasic {
				delta, n := runLength(tokens, i, '+', '-')
				i += n
				prog = append(prog, Instruction{Op: OpAdd, Arg: delta})
			} else {
				d := 1
				if c == '-' {
					d = -1
				}
				prog = append(prog, Instruction{Op: OpAdd, Arg: d})
				i++
			}
		case '>', '<':
			if level >= OptBasic {
				delta, n := runLength(tokens, i, '>', '<')
				i += n
				prog = append(prog, Instruction{Op: OpMove, Arg: delta})
			} else {
				d := 1
				if c == '<' {
					d = -1
				}
				prog = append(prog, Instruction{Op: OpMove, Arg: d})
				i++
			}
		case '.':
			prog = append(prog, Instruction{Op: OpOutput})
			i++
		case ',':
			prog = append(prog, Instruction{Op: OpInput})
			i++
		case '[':
			if level >= OptFull {
				if op, n, ok := matchPattern(tokens, i); ok {
					prog = append(prog, op...)
					i += n
					continue
				}
			}
			stack = append(stack, len(prog))
			prog = append(prog, Instruction{Op: OpJumpIfZero})
			i++
		case ']':
			if len(stack) == 0 {
				return nil, &CompileError{Msg: "unmatched ']'", Offset: tokens[i].Offset}
			}
			open := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			closeIdx := len(prog)
			prog = append(prog, Instruction{Op: OpJumpIfNonZero, Arg: open + 1})
			prog[open].Arg = closeIdx + 1
			i++
		}
	}

	if len(stack) != 0 {
		return nil, &CompileError{Msg: "unmatched '['", Offset: firstUnmatchedOpen(tokens)}
	}

	prog = append(prog, Instruction{Op: OpHalt})
	return prog, nil
}

// runLength folds a run of +/- (or >/<) starting at i into one net delta and
// returns it with the number of tokens used.
func runLength(tokens []Token, i int, plusCmd, minusCmd byte) (delta, n int) {
	for i+n < len(tokens) {
		switch tokens[i+n].Cmd {
		case plusCmd:
			delta++
		case minusCmd:
			delta--
		default:
			return delta, n
		}
		n++
	}
	return delta, n
}

// matchPattern tries to replace the loop starting at tokens[i] with cheaper
// instructions. It returns them and the number of tokens they replace.
func matchPattern(tokens []Token, i int) ([]Instruction, int, bool) {
	// Only innermost loops without I/O can be reduced to arithmetic.
	j := i + 1
	for j < len(tokens) {
		c := tokens[j].Cmd
		if c == ']' {
			break
		}
		if c == '[' || c == '.' || c == ',' {
			return nil, 0, false
		}
		j++
	}
	if j >= len(tokens) {
		return nil, 0, false // unbalanced; Compile reports it
	}
	body := tokens[i+1 : j]
	consumed := j - i + 1

	if len(body) == 1 && (body[0].Cmd == '-' || body[0].Cmd == '+') {
		return []Instruction{{Op: OpClear}}, consumed, true
	}
	if len(body) == 1 && body[0].Cmd == '>' {
		return []Instruction{{Op: OpScanRight}}, consumed, true
	}
	if len(body) == 1 && body[0].Cmd == '<' {
		return []Instruction{{Op: OpScanLeft}}, consumed, true
	}

	// Copy/multiply loop: the pointer ends where it started and the current
	// cell drops by exactly 1 per pass. The loop then runs cells[dp] times, so
	// each other cell just gains delta*cells[dp], and cells[dp] ends at 0.
	deltas, ok := scanCopyLoop(body)
	if !ok {
		return nil, 0, false
	}
	if deltas[0] != -1 {
		return nil, 0, false
	}
	ops := make([]Instruction, 0, len(deltas))
	// Sorted so --dump output is stable.
	for _, off := range slices.Sorted(maps.Keys(deltas)) {
		if off == 0 {
			continue
		}
		ops = append(ops, Instruction{Op: OpAddTo, Arg: off, Arg2: deltas[off]})
	}
	ops = append(ops, Instruction{Op: OpClear})
	return ops, consumed, true
}

// scanCopyLoop returns the net change to each cell the loop body touches, by
// offset, or false if the pointer doesn't end where it started.
func scanCopyLoop(body []Token) (map[int]int, bool) {
	deltas := map[int]int{0: 0}
	off := 0
	for _, t := range body {
		switch t.Cmd {
		case '+':
			deltas[off]++
		case '-':
			deltas[off]--
		case '>':
			off++
			if _, ok := deltas[off]; !ok {
				deltas[off] = 0
			}
		case '<':
			off--
			if _, ok := deltas[off]; !ok {
				deltas[off] = 0
			}
		default:
			return nil, false
		}
	}
	if off != 0 {
		return nil, false
	}
	return deltas, true
}

// firstUnmatchedOpen returns the source offset of the first '[' that is never
// closed, or -1. Instruction indices no longer map to tokens once loops are
// optimised away, so this rescans the tokens; it only runs on the error path.
func firstUnmatchedOpen(tokens []Token) int {
	var stack []int
	for k := range tokens {
		switch tokens[k].Cmd {
		case '[':
			stack = append(stack, tokens[k].Offset)
		case ']':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if len(stack) > 0 {
		return stack[0]
	}
	return -1
}
