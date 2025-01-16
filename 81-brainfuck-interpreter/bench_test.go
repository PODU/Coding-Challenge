package main

import (
	"bytes"
	"strings"
	"testing"
)

// busyProgram is a compute-heavy, deterministic program: a triple-nested loop
// that performs a large number of cell operations and clears, exercising the
// run-length, Clear and copy-loop optimizations. It produces no output.
const busyProgram = "++++++++++[->++++++++++[->++++++++++[-]<]<]"

func benchOpt(b *testing.B, level OptLevel) {
	prog, err := Compile([]byte(busyProgram), level)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tape := NewTape(30000, false)
		var out bytes.Buffer
		vm := NewVM(prog, tape, strings.NewReader(""), &out, EOFZero)
		if err := vm.Run(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBusyOptNone(b *testing.B)  { benchOpt(b, OptNone) }
func BenchmarkBusyOptBasic(b *testing.B) { benchOpt(b, OptBasic) }
func BenchmarkBusyOptFull(b *testing.B)  { benchOpt(b, OptFull) }
