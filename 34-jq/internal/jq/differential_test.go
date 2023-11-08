package jq

import (
	"os/exec"
	"strings"
	"testing"
)

// TestDifferentialAgainstJq runs the same (filter, input) through real jq and
// asserts byte-equality (modulo Windows CRLF) with our in-process evaluator.
// It is skipped when jq is not on PATH so CI without jq still passes.
func TestDifferentialAgainstJq(t *testing.T) {
	jqPath, err := exec.LookPath("jq")
	if err != nil {
		t.Skip("jq not installed; skipping differential tests")
	}

	cases := []struct{ filter, input string }{
		{".", `{"b":2,"a":1}`},
		{".", `[1,2,3]`},
		{".a.b.c", `{"a":{"b":{"c":7}}}`},
		{".[0]", `[10,20,30]`},
		{".[-1]", `[10,20,30]`},
		{".[5]", `[10,20,30]`},
		{".[]", `{"a":1,"b":2}`},
		{".[1:3]", `[1,2,3,4,5]`},
		{"map(.+1)", `[1,2,3]`},
		{"keys", `{"c":3,"a":1,"b":2}`},
		{"keys_unsorted", `{"c":3,"a":1,"b":2}`},
		{"to_entries", `{"a":1,"b":2}`},
		{"from_entries", `[{"key":"a","value":1}]`},
		{"[paths]", `{"a":[1,2],"b":3}`},
		{"group_by(.k)", `[{"k":1},{"k":2},{"k":1}]`},
		{"sort_by(.n)", `[{"n":3},{"n":1},{"n":2}]`},
		{"unique", `[3,1,2,1,3]`},
		{"unique_by(.%2)", `[1,2,3,4,5]`},
		{"add", `[1,2,3]`},
		{"flatten", `[1,[2,[3]]]`},
		{"flatten(1)", `[1,[2,[3]]]`},
		{"[range(2;8;2)]", `null`},
		{"sort", `[3,1,2]`},
		{"reverse", `[1,2,3]`},
		{". * 2", `21`},
		{"7 % 3", `null`},
		{`"a,b,c" / ","`, `null`},
		{"{a:1} + {b:2}", `null`},
		{"{a:1,b:2} * {b:{c:3}}", `null`},
		{"[1,2,3] - [2]", `null`},
		{`.a // "d"`, `{"b":1}`},
		{"@csv", `[1,"x",true,null]`},
		{"@tsv", `[1,"x"]`},
		{"@base64", `"hello"`},
		{"ascii_upcase", `"abc"`},
		{"contains({a:1})", `{"a":1,"b":2}`},
		{`has("a")`, `{"a":1}`},
		{"min_by(.x)", `[{"x":3},{"x":1}]`},
		{"max_by(.x)", `[{"x":3},{"x":1}]`},
		{"tostring", `123`},
		{"tonumber", `"1.5"`},
		{`split(",")`, `"a,b,c"`},
		{`join("-")`, `["a","b","c"]`},
		{`walk(if type=="number" then .+1 else . end)`, `{"a":1,"b":[2,3]}`},
		{"del(.b)", `{"a":1,"b":2,"c":3}`},
		{"del(.[1])", `[10,20,30]`},
		{`setpath(["a","b"];9)`, `{"a":{"b":1}}`},
		{`getpath(["a","b"])`, `{"a":{"b":7}}`},
		{`{(.k):.v}`, `{"k":"id","v":7}`},
		{`if .v==1 then "one" else "x" end`, `{"v":1}`},
		{"[.. | numbers]", `{"a":1,"b":[2,3],"c":"x"}`},
		{`"hi \(.n)!"`, `{"n":"x"}`},
		{"map_values(.*2)", `{"a":1,"b":2}`},
		{"[.[]|select(.>15)]", `[10,20,30]`},
		{"length", `"héllo"`},
		{"-5 | length", `null`},
	}

	for _, c := range cases {
		t.Run(c.filter, func(t *testing.T) {
			cmd := exec.Command(jqPath, "-c", c.filter)
			cmd.Stdin = strings.NewReader(c.input)
			jqOut, err := cmd.Output()
			if err != nil {
				t.Fatalf("jq failed for %q: %v", c.filter, err)
			}
			want := strings.ReplaceAll(string(jqOut), "\r\n", "\n")
			want = strings.TrimRight(want, "\n")

			got, err := evalString(c.filter, c.input, compact)
			if err != nil {
				t.Fatalf("ccjq failed for %q: %v", c.filter, err)
			}
			if got != want {
				t.Errorf("filter %q on %s:\n ccjq: %q\n jq:   %q", c.filter, c.input, got, want)
			}
		})
	}
}
