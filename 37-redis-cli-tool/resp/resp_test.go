package resp

import (
	"bufio"
	"reflect"
	"strings"
	"testing"
)

func TestEncode(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"ping", []string{"PING"}, "*1\r\n$4\r\nPING\r\n"},
		{"set", []string{"SET", "name", "John"}, "*3\r\n$3\r\nSET\r\n$4\r\nname\r\n$4\r\nJohn\r\n"},
		{"empty arg", []string{"SET", "k", ""}, "*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$0\r\n\r\n"},
		{"arg with spaces", []string{"SET", "k", "two words"}, "*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$9\r\ntwo words\r\n"},
		{"no args", []string{}, "*0\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(Encode(tt.args)); got != tt.want {
				t.Errorf("Encode(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Value
	}{
		{"simple string", "+OK\r\n", Value{Type: SimpleString, Str: "OK"}},
		{"error", "-ERR unknown command\r\n", Value{Type: Error, Str: "ERR unknown command"}},
		{"integer", ":1000\r\n", Value{Type: Integer, Int: 1000}},
		{"negative integer", ":-5\r\n", Value{Type: Integer, Int: -5}},
		{"bulk string", "$5\r\nhello\r\n", Value{Type: BulkString, Str: "hello"}},
		{"empty bulk", "$0\r\n\r\n", Value{Type: BulkString, Str: ""}},
		{"null bulk", "$-1\r\n", Value{Type: BulkString, IsNull: true}},
		{"null array", "*-1\r\n", Value{Type: Array, IsNull: true}},
		{"empty array", "*0\r\n", Value{Type: Array, Array: []Value{}}},
		{"binary-safe bulk", "$3\r\na\r\x00\r\n", Value{Type: BulkString, Str: "a\r\x00"}},
		{
			"flat array",
			"*2\r\n$1\r\na\r\n:7\r\n",
			Value{Type: Array, Array: []Value{
				{Type: BulkString, Str: "a"},
				{Type: Integer, Int: 7},
			}},
		},
		{
			"nested array",
			"*2\r\n$1\r\na\r\n*2\r\n$6\r\nnested\r\n:7\r\n",
			Value{Type: Array, Array: []Value{
				{Type: BulkString, Str: "a"},
				{Type: Array, Array: []Value{
					{Type: BulkString, Str: "nested"},
					{Type: Integer, Int: 7},
				}},
			}},
		},
		{
			"array with nil element",
			"*2\r\n$-1\r\n$2\r\nhi\r\n",
			Value{Type: Array, Array: []Value{
				{Type: BulkString, IsNull: true},
				{Type: BulkString, Str: "hi"},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bufio.NewReader(strings.NewReader(tt.input))
			got, err := Decode(r)
			if err != nil {
				t.Fatalf("Decode(%q) error: %v", tt.input, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Decode(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDecodeErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unknown type byte", "?oops\r\n"},
		{"bad integer", ":notanumber\r\n"},
		{"bad bulk length", "$abc\r\nhello\r\n"},
		{"missing CRLF on line", "+OK\n"},
		{"truncated bulk", "$5\r\nhel"},
		{"truncated array", "*2\r\n$1\r\na\r\n"},
		{"empty stream", ""},
		{"oversized bulk", "$999999999999\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bufio.NewReader(strings.NewReader(tt.input))
			if _, err := Decode(r); err == nil {
				t.Errorf("Decode(%q) expected error, got nil", tt.input)
			}
		})
	}
}

// TestRoundTrip confirms Marshal and Decode are inverses for every type,
// including nested arrays and nils.
func TestRoundTrip(t *testing.T) {
	values := []Value{
		{Type: SimpleString, Str: "PONG"},
		{Type: Error, Str: "ERR boom"},
		{Type: Integer, Int: 42},
		{Type: BulkString, Str: "hello"},
		{Type: BulkString, IsNull: true},
		{Type: Array, IsNull: true},
		{Type: Array, Array: []Value{}},
		{Type: Array, Array: []Value{
			{Type: BulkString, Str: "a"},
			{Type: Array, Array: []Value{
				{Type: Integer, Int: 1},
				{Type: BulkString, IsNull: true},
			}},
		}},
	}
	for _, v := range values {
		b := Marshal(v)
		got, err := Decode(bufio.NewReader(strings.NewReader(string(b))))
		if err != nil {
			t.Fatalf("Decode(Marshal(%+v)) error: %v", v, err)
		}
		if !reflect.DeepEqual(got, v) {
			t.Errorf("round-trip mismatch: got %+v, want %+v (wire %q)", got, v, b)
		}
	}
}
