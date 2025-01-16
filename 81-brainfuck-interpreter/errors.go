package main

import "fmt"

// CompileError is an unbalanced bracket or bad option. Exit code 2.
type CompileError struct {
	Msg    string
	Offset int // byte offset in the source, or -1
}

func (e *CompileError) Error() string {
	if e.Offset >= 0 {
		return fmt.Sprintf("compile error: %s at byte offset %d", e.Msg, e.Offset)
	}
	return "compile error: " + e.Msg
}

// RuntimeError is a failure while running, such as the pointer leaving the
// tape. Exit code 1.
type RuntimeError struct {
	Msg string
}

func (e *RuntimeError) Error() string { return "runtime error: " + e.Msg }

// IOError is a read or write failure, such as a missing source file. Exit
// code 3.
type IOError struct {
	Msg string
	Err error
}

func (e *IOError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("io error: %s: %v", e.Msg, e.Err)
	}
	return "io error: " + e.Msg
}

func (e *IOError) Unwrap() error { return e.Err }
