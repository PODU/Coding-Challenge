package main

import (
	"bufio"
	"io"
)

// EOFMode is what ',' stores once input has run out.
type EOFMode int

const (
	EOFZero      EOFMode = iota // store 0 (default)
	EOFNegOne                   // store 255 (-1 as a byte)
	EOFUnchanged                // leave the cell as-is
)

func ParseEOFMode(s string) (EOFMode, error) {
	switch s {
	case "0":
		return EOFZero, nil
	case "neg1":
		return EOFNegOne, nil
	case "unchanged":
		return EOFUnchanged, nil
	default:
		return 0, &CompileError{Msg: "invalid --eof mode " + s + " (want 0|neg1|unchanged)", Offset: -1}
	}
}

// VM runs compiled bytecode. Output is buffered; Run flushes it.
type VM struct {
	prog []Instruction
	tape *Tape
	in   *bufio.Reader
	out  *bufio.Writer
	eof  EOFMode
}

// NewVM wraps in and out in buffers unless they already are.
func NewVM(prog []Instruction, tape *Tape, in io.Reader, out io.Writer, eof EOFMode) *VM {
	return &VM{
		prog: prog,
		tape: tape,
		in:   bufio.NewReader(in),
		out:  bufio.NewWriter(out),
		eof:  eof,
	}
}

// Run executes the program. Errors are *RuntimeError or *IOError.
func (vm *VM) Run() error {
	if err := vm.exec(); err != nil {
		vm.out.Flush() // emit whatever was produced before the failure
		return err
	}
	if err := vm.out.Flush(); err != nil {
		return &IOError{Msg: "flushing output", Err: err}
	}
	return nil
}

func (vm *VM) exec() error {
	prog := vm.prog
	tape := vm.tape
	pc := 0
	for pc < len(prog) {
		ins := prog[pc]
		switch ins.Op {
		case OpAdd:
			tape.add(ins.Arg)
			pc++
		case OpMove:
			if err := tape.move(ins.Arg); err != nil {
				return err
			}
			pc++
		case OpOutput:
			if err := vm.out.WriteByte(tape.get()); err != nil {
				return &IOError{Msg: "writing output", Err: err}
			}
			pc++
		case OpInput:
			if err := vm.readInto(); err != nil {
				return err
			}
			pc++
		case OpJumpIfZero:
			if tape.get() == 0 {
				pc = ins.Arg
			} else {
				pc++
			}
		case OpJumpIfNonZero:
			if tape.get() != 0 {
				pc = ins.Arg
			} else {
				pc++
			}
		case OpClear:
			tape.set(0)
			pc++
		case OpScanRight:
			for tape.get() != 0 {
				if err := tape.move(1); err != nil {
					return err
				}
			}
			pc++
		case OpScanLeft:
			for tape.get() != 0 {
				if err := tape.move(-1); err != nil {
					return err
				}
			}
			pc++
		case OpAddTo:
			if tape.get() != 0 {
				cell, err := tape.at(ins.Arg)
				if err != nil {
					return err
				}
				*cell += uint8(ins.Arg2 * int(tape.get()))
			}
			pc++
		case OpHalt:
			return nil
		default:
			return &RuntimeError{Msg: "unknown opcode"}
		}
	}
	return nil
}

func (vm *VM) readInto() error {
	b, err := vm.in.ReadByte()
	if err == nil {
		vm.tape.set(b)
		return nil
	}
	if err == io.EOF {
		switch vm.eof {
		case EOFZero:
			vm.tape.set(0)
		case EOFNegOne:
			vm.tape.set(255)
		case EOFUnchanged:
			// leave cell as-is
		}
		return nil
	}
	return &IOError{Msg: "reading input", Err: err}
}
