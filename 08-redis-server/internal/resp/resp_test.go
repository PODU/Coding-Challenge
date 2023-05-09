package resp

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"
)

// encode is a test helper that serializes v to bytes.
func encode(t *testing.T, v Value) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := NewWriter(bufio.NewWriter(&buf))
	if err := w.Write(v); err != nil {
		t.Fatalf("Write(%+v) error: %v", v, err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush error: %v", err)
	}
	return buf.Bytes()
}

// decode is a test helper that reads one value from wire bytes.
func decode(t *testing.T, wire string) (Value, error) {
	t.Helper()
	r := NewReader(bufio.NewReader(strings.NewReader(wire)))
	return r.Read()
}

func TestEncode(t *testing.T) {
	cases := []struct {
		name string
		val  Value
		want string
	}{
		{"simple string", SimpleString("OK"), "+OK\r\n"},
		{"error", ErrorString("Error message"), "-Error message\r\n"},
		{"integer", Int(1000), ":1000\r\n"},
		{"negative integer", Int(-42), ":-42\r\n"},
		{"bulk string", BulkString("hello"), "$5\r\nhello\r\n"},
		{"empty bulk", BulkString(""), "$0\r\n\r\n"},
		{"null bulk", NullBulk(), "$-1\r\n"},
		{"array", ArrayOf(BulkString("PING")), "*1\r\n$4\r\nPING\r\n"},
		{"empty array", ArrayOf(), "*0\r\n"},
		{"null array", NullArray(), "*-1\r\n"},
		{"nested array", ArrayOf(Int(1), BulkString("two")), "*2\r\n:1\r\n$3\r\ntwo\r\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := string(encode(t, c.val))
			if got != c.want {
				t.Errorf("encode = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	cases := []struct {
		name string
		wire string
		want Value
	}{
		{"simple string", "+OK\r\n", SimpleString("OK")},
		{"error", "-Error message\r\n", ErrorString("Error message")},
		{"integer", ":1000\r\n", Int(1000)},
		{"bulk string", "$5\r\nhello\r\n", BulkString("hello")},
		{"empty bulk", "$0\r\n\r\n", BulkString("")},
		{"null bulk", "$-1\r\n", NullBulk()},
		{"array", "*1\r\n$4\r\nPING\r\n", ArrayOf(BulkString("PING"))},
		{"empty array", "*0\r\n", ArrayOf()},
		{"null array", "*-1\r\n", NullArray()},
		{"command array", "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n",
			ArrayOf(BulkString("SET"), BulkString("foo"), BulkString("bar"))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := decode(t, c.wire)
			if err != nil {
				t.Fatalf("decode(%q) error: %v", c.wire, err)
			}
			if !got.Equal(c.want) {
				t.Errorf("decode = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	corpus := []Value{
		SimpleString("OK"),
		ErrorString("WRONGTYPE bad"),
		Int(0),
		Int(-9223372036854775808),
		Int(9223372036854775807),
		BulkString("hello world"),
		BulkString(""),
		NullBulk(),
		ArrayOf(),
		NullArray(),
		ArrayOf(BulkString("SET"), BulkString("k"), BulkString("v")),
		ArrayOf(Int(1), ArrayOf(BulkString("nested"), NullBulk())),
		// Binary-safe payload with embedded CRLF and NUL bytes.
		BulkString("line1\r\nline2\x00end"),
	}
	for i, v := range corpus {
		wire := encode(t, v)
		got, err := decode(t, string(wire))
		if err != nil {
			t.Fatalf("case %d: decode error: %v", i, err)
		}
		if !got.Equal(v) {
			t.Errorf("case %d: round-trip mismatch: got %+v want %+v", i, got, v)
		}
	}
}

func TestBinarySafeBulk(t *testing.T) {
	// A bulk string whose payload contains CRLF must be read by length, not by
	// scanning for the terminator.
	payload := "ab\r\ncd"
	wire := "$6\r\n" + payload + "\r\n"
	got, err := decode(t, wire)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got.Str != payload {
		t.Errorf("got %q, want %q", got.Str, payload)
	}
}

func TestInlineCommand(t *testing.T) {
	got, err := decode(t, "PING\r\n")
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	want := ArrayOf(BulkString("PING"))
	if !got.Equal(want) {
		t.Errorf("inline = %+v, want %+v", got, want)
	}

	got, err = decode(t, "SET foo bar\r\n")
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	want = ArrayOf(BulkString("SET"), BulkString("foo"), BulkString("bar"))
	if !got.Equal(want) {
		t.Errorf("inline = %+v, want %+v", got, want)
	}
}

func TestMalformedInput(t *testing.T) {
	cases := []struct {
		name string
		wire string
	}{
		{"non-numeric bulk length", "$abc\r\n"},
		{"negative bulk length", "$-5\r\n"},
		{"non-numeric array length", "*xx\r\n"},
		{"bad integer", ":notanint\r\n"},
		{"bulk missing CRLF", "$3\r\nabcd\r\n"}, // payload longer than declared
		{"truncated bulk", "$10\r\nabc"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Must return an error, never panic.
			_, err := decode(t, c.wire)
			if err == nil {
				t.Errorf("expected error for %q, got nil", c.wire)
			}
		})
	}
}

func TestEOF(t *testing.T) {
	_, err := decode(t, "")
	if err != io.EOF {
		t.Errorf("expected io.EOF on empty input, got %v", err)
	}
}
