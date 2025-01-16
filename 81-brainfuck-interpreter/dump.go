package main

import (
	"fmt"
	"io"
)

func opName(op OpCode) string {
	switch op {
	case OpAdd:
		return "Add"
	case OpMove:
		return "Move"
	case OpOutput:
		return "Output"
	case OpInput:
		return "Input"
	case OpJumpIfZero:
		return "JumpIfZero"
	case OpJumpIfNonZero:
		return "JumpIfNonZero"
	case OpClear:
		return "Clear"
	case OpScanRight:
		return "ScanRight"
	case OpScanLeft:
		return "ScanLeft"
	case OpAddTo:
		return "AddTo"
	case OpHalt:
		return "Halt"
	default:
		return "Unknown"
	}
}

// DumpBytecode prints the program, one instruction per line.
func DumpBytecode(w io.Writer, prog []Instruction) {
	for i, ins := range prog {
		switch ins.Op {
		case OpAdd, OpMove:
			fmt.Fprintf(w, "%4d  %-14s %d\n", i, opName(ins.Op), ins.Arg)
		case OpJumpIfZero, OpJumpIfNonZero:
			fmt.Fprintf(w, "%4d  %-14s -> %d\n", i, opName(ins.Op), ins.Arg)
		case OpAddTo:
			fmt.Fprintf(w, "%4d  %-14s off=%d factor=%d\n", i, opName(ins.Op), ins.Arg, ins.Arg2)
		default:
			fmt.Fprintf(w, "%4d  %s\n", i, opName(ins.Op))
		}
	}
}
