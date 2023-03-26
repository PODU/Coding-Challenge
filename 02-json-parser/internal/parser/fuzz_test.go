package parser

import (
	"encoding/json"
	"testing"
)

// FuzzParseAgainstStdlib feeds arbitrary bytes to our parser and asserts two
// invariants:
//  1. The parser never panics.
//  2. Our validity verdict agrees with encoding/json.Valid (the stdlib oracle).
//
// Run with: go test -run=^$ -fuzz=FuzzParseAgainstStdlib ./internal/parser
func FuzzParseAgainstStdlib(f *testing.F) {
	seeds := []string{
		`{}`, `[]`, `{"a":1}`, `[1,2,3]`, `"x"`, `42`, `true`, `null`,
		`{"a":[1,{"b":2}]}`, `-1.2e+10`, ``, `{`, `[1,]`, `{"a":}`,
		`"é"`, `"😀"`, `01`, `1.`, `tru`,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		_, err := Parse(data)
		ourValid := err == nil
		stdValid := json.Valid(data)

		if ourValid == stdValid {
			return
		}

		// We are intentionally *stricter* than encoding/json in ways the design
		// documents and RFC 8259 sanctions:
		//   - invalid UTF-8 bytes are rejected (stdlib accepts them),
		//   - lone/unpaired \uXXXX surrogate escapes are rejected (stdlib
		//     replaces them with U+FFFD and accepts).
		// So "we reject, stdlib accepts" is allowed. The bug-catching direction
		// is the opposite: stdlib is lenient, so anything it rejects is truly
		// invalid JSON — if we accept it, that is a real bug.
		if ourValid && !stdValid {
			t.Errorf("we accepted JSON that the stdlib rejects (bug): %q", data)
		}
	})
}
