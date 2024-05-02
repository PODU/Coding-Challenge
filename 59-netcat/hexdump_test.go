package main

import (
	"bytes"
	"testing"
)

func TestHexDumpCanonical(t *testing.T) {
	var b bytes.Buffer
	hexDump(&b, 0, []byte("hi\n"))
	want := "00000000  68 69 0a                                          |hi.|\n"
	if b.String() != want {
		t.Fatalf("hexdump:\n got %q\nwant %q", b.String(), want)
	}
}

func TestHexDumpFullRows(t *testing.T) {
	data := []byte("0123456789ABCDEF0123456789abcdef!")
	var b bytes.Buffer
	hexDump(&b, 0, data)
	want := "" +
		"00000000  30 31 32 33 34 35 36 37  38 39 41 42 43 44 45 46  |0123456789ABCDEF|\n" +
		"00000010  30 31 32 33 34 35 36 37  38 39 61 62 63 64 65 66  |0123456789abcdef|\n" +
		"00000020  21                                                |!|\n"
	if b.String() != want {
		t.Fatalf("hexdump full rows:\n got %q\nwant %q", b.String(), want)
	}
}

func TestHexSinkPassthroughOffset(t *testing.T) {
	var dump bytes.Buffer
	s := newHexSink(&dump)
	n, err := s.Write([]byte("ab"))
	if err != nil || n != 2 {
		t.Fatalf("write1: n=%d err=%v", n, err)
	}
	s.Write([]byte("c"))
	// Second write must continue the offset at 0x02.
	want := "" +
		"00000000  61 62                                             |ab|\n" +
		"00000002  63                                                |c|\n"
	if dump.String() != want {
		t.Fatalf("hexsink offset:\n got %q\nwant %q", dump.String(), want)
	}
}
