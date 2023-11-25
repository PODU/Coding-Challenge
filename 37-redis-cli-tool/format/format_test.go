package format

import (
	"testing"

	"rediscli/resp"
)

func TestRender(t *testing.T) {
	tests := []struct {
		name string
		in   resp.Value
		want string
	}{
		{"simple string", resp.Value{Type: resp.SimpleString, Str: "PONG"}, "PONG"},
		{"integer", resp.Value{Type: resp.Integer, Int: 42}, "(integer) 42"},
		{"error", resp.Value{Type: resp.Error, Str: "ERR unknown command 'FOO'"}, "(error) ERR unknown command 'FOO'"},
		{"bulk string", resp.Value{Type: resp.BulkString, Str: "hello"}, `"hello"`},
		{"empty bulk", resp.Value{Type: resp.BulkString, Str: ""}, `""`},
		{"null bulk", resp.Value{Type: resp.BulkString, IsNull: true}, "(nil)"},
		{"null array", resp.Value{Type: resp.Array, IsNull: true}, "(nil)"},
		{"empty array", resp.Value{Type: resp.Array, Array: []resp.Value{}}, "(empty array)"},
		{"bulk with quotes/newline", resp.Value{Type: resp.BulkString, Str: "a\"b\nc"}, `"a\"b\nc"`},
		{
			"flat array",
			resp.Value{Type: resp.Array, Array: []resp.Value{
				{Type: resp.BulkString, Str: "a"},
				{Type: resp.Integer, Int: 1},
			}},
			"1) \"a\"\n2) (integer) 1",
		},
		{
			"nested array",
			resp.Value{Type: resp.Array, Array: []resp.Value{
				{Type: resp.BulkString, Str: "a"},
				{Type: resp.Array, Array: []resp.Value{
					{Type: resp.BulkString, Str: "nested"},
					{Type: resp.Integer, Int: 7},
				}},
			}},
			"1) \"a\"\n2) 1) \"nested\"\n   2) (integer) 7",
		},
		{
			"array index alignment >= 10",
			resp.Value{Type: resp.Array, Array: mkInts(10)},
			" 1) (integer) 1\n 2) (integer) 2\n 3) (integer) 3\n 4) (integer) 4\n" +
				" 5) (integer) 5\n 6) (integer) 6\n 7) (integer) 7\n 8) (integer) 8\n" +
				" 9) (integer) 9\n10) (integer) 10",
		},
		{
			"array with nil element",
			resp.Value{Type: resp.Array, Array: []resp.Value{
				{Type: resp.BulkString, IsNull: true},
				{Type: resp.BulkString, Str: "x"},
			}},
			"1) (nil)\n2) \"x\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Render(tt.in); got != tt.want {
				t.Errorf("Render() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func mkInts(n int) []resp.Value {
	out := make([]resp.Value, n)
	for i := range out {
		out[i] = resp.Value{Type: resp.Integer, Int: int64(i + 1)}
	}
	return out
}
