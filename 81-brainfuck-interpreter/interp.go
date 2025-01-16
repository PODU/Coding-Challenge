package main

import (
	"bufio"
	"io"
)

// Interpreter runs the commands directly, with no compiler or optimisations.
// It exists so the tests can check the VM against something independent.
type Interpreter struct {
	tape *Tape
	in   *bufio.Reader
	out  *bufio.Writer
	eof  EOFMode
}

func NewInterpreter(tape *Tape, in io.Reader, out io.Writer, eof EOFMode) *Interpreter {
	return &Interpreter{
		tape: tape,
		in:   bufio.NewReader(in),
		out:  bufio.NewWriter(out),
		eof:  eof,
	}
}

func (it *Interpreter) Run(src []byte) error {
	tokens := Tokenize(src)
	jump, err := buildJumpTable(tokens)
	if err != nil {
		return err
	}
	if err := it.exec(tokens, jump); err != nil {
		it.out.Flush()
		return err
	}
	if err := it.out.Flush(); err != nil {
		return &IOError{Msg: "flushing output", Err: err}
	}
	return nil
}

// buildJumpTable maps each bracket's token index to its partner's.
func buildJumpTable(tokens []Token) ([]int, error) {
	jump := make([]int, len(tokens))
	var stack []int
	for i, t := range tokens {
		switch t.Cmd {
		case '[':
			stack = append(stack, i)
		case ']':
			if len(stack) == 0 {
				return nil, &CompileError{Msg: "unmatched ']'", Offset: t.Offset}
			}
			open := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			jump[open] = i
			jump[i] = open
		}
	}
	if len(stack) != 0 {
		return nil, &CompileError{Msg: "unmatched '['", Offset: tokens[stack[0]].Offset}
	}
	return jump, nil
}

func (it *Interpreter) exec(tokens []Token, jump []int) error {
	tape := it.tape
	for pc := 0; pc < len(tokens); pc++ {
		switch tokens[pc].Cmd {
		case '>':
			if err := tape.move(1); err != nil {
				return err
			}
		case '<':
			if err := tape.move(-1); err != nil {
				return err
			}
		case '+':
			tape.add(1)
		case '-':
			tape.add(-1)
		case '.':
			if err := it.out.WriteByte(tape.get()); err != nil {
				return &IOError{Msg: "writing output", Err: err}
			}
		case ',':
			if err := it.readInto(); err != nil {
				return err
			}
		case '[':
			if tape.get() == 0 {
				pc = jump[pc]
			}
		case ']':
			if tape.get() != 0 {
				pc = jump[pc]
			}
		}
	}
	return nil
}

func (it *Interpreter) readInto() error {
	b, err := it.in.ReadByte()
	if err == nil {
		it.tape.set(b)
		return nil
	}
	if err == io.EOF {
		switch it.eof {
		case EOFZero:
			it.tape.set(0)
		case EOFNegOne:
			it.tape.set(255)
		case EOFUnchanged:
		}
		return nil
	}
	return &IOError{Msg: "reading input", Err: err}
}
