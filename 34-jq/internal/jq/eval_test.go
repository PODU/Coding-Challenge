package jq

import (
	"strings"
	"testing"
)

// evalString compiles src, runs it against the JSON value parsed from inputJSON
// and returns the outputs joined by newlines (each encoded with opts).
func evalString(src, inputJSON string, opts EncodeOptions) (string, error) {
	prog, err := Compile(src)
	if err != nil {
		return "", err
	}
	input, err := ParseValue([]byte(inputJSON))
	if err != nil {
		return "", err
	}
	var out []string
	err = Run(prog, input, func(v Value) error {
		out = append(out, Encode(v, opts))
		return nil
	})
	if err != nil {
		return "", err
	}
	return strings.Join(out, "\n"), nil
}

var compact = EncodeOptions{Compact: true}

// TestGolden is the core filter -> output matrix. Outputs were cross-checked
// against jq 1.8 (modulo number canonicalisation; see README caveats).
func TestGolden(t *testing.T) {
	cases := []struct {
		name   string
		filter string
		input  string
		opts   EncodeOptions
		want   string
	}{
		// identity / pretty
		{"identity_scalar", ".", "42", compact, "42"},
		{"identity_array", ".", "[1,2,3]", compact, "[1,2,3]"},
		{"identity_obj_order", ".", `{"b":2,"a":1}`, compact, `{"b":2,"a":1}`},

		// indexing
		{"index0", ".[0]", "[10,20,30]", compact, "10"},
		{"index_neg", ".[-1]", "[10,20,30]", compact, "30"},
		{"index_oor", ".[5]", "[10,20,30]", compact, "null"},
		{"index_neg_oor", ".[-9]", "[10,20,30]", compact, "null"},

		// field access
		{"field", ".name", `{"name":"ccjq"}`, compact, `"ccjq"`},
		{"field_nested", ".meta.v", `{"meta":{"v":1}}`, compact, "1"},
		{"field_missing", ".missing", `{"a":1}`, compact, "null"},
		{"field_on_null", ".foo", "null", compact, "null"},
		{"field_bracket_str", `.["name"]`, `{"name":"ccjq"}`, compact, `"ccjq"`},
		{"field_dot_str", `."name"`, `{"name":"ccjq"}`, compact, `"ccjq"`},

		// pipe / iterate / comma
		{"pipe", ".[0] | .commit.message", `[{"commit":{"message":"init"}}]`, compact, `"init"`},
		{"iterate_msgs", ".[].commit.message", `[{"commit":{"message":"init"}},{"commit":{"message":"fix"}}]`, compact, "\"init\"\n\"fix\""},
		{"iterate_obj", ".[]", `{"a":1,"b":2}`, compact, "1\n2"},
		{"comma", ".a, .b", `{"a":1,"b":2}`, compact, "1\n2"},

		// construction
		{"array_collect", "[.quotes[].quote]", `{"quotes":[{"quote":"a"},{"quote":"b"}]}`, compact, `["a","b"]`},
		{"array_map_double", "[.[] | . * 2]", "[10,20,30]", compact, "[20,40,60]"},
		{"obj_construct", "{n: .name}", `{"name":"ccjq"}`, compact, `{"n":"ccjq"}`},
		{"obj_shorthand", "{name}", `{"name":"ccjq","x":1}`, compact, `{"name":"ccjq"}`},
		{"obj_cartesian", "{a:(1,2),b:(3,4)}", "null", compact, `{"a":1,"b":3}` + "\n" + `{"a":1,"b":4}` + "\n" + `{"a":2,"b":3}` + "\n" + `{"a":2,"b":4}`},
		{"obj_computed_key", `{(.k): .v}`, `{"k":"id","v":7}`, compact, `{"id":7}`},
		{"empty_array", "[]", "null", compact, "[]"},
		{"empty_obj", "{}", "null", compact, "{}"},

		// slices
		{"slice", ".[1:3]", "[10,20,30,40]", compact, "[20,30]"},
		{"slice_open_lo", ".[:2]", "[10,20,30]", compact, "[10,20]"},
		{"slice_open_hi", ".[1:]", "[10,20,30]", compact, "[20,30]"},
		{"slice_neg", ".[-2:]", "[10,20,30]", compact, "[20,30]"},
		{"slice_string", ".[1:3]", `"hello"`, compact, `"el"`},

		// recursion
		{"recurse_numbers", "[.. | numbers]", `{"a":1,"b":[2,3],"c":"x"}`, compact, "[1,2,3]"},
		{"recurse_default", "[..]", "[1,[2]]", compact, "[[1,[2]],1,[2],2]"},

		// arithmetic / comparison
		{"add_nums", ". + 1", "41", compact, "42"},
		{"sub_nums", "10 - 3", "null", compact, "7"},
		{"mul", ". * 2", "21", compact, "42"},
		{"div", "10 / 4", "null", compact, "2.5"},
		{"mod", "7 % 3", "null", compact, "1"},
		{"str_concat", `. + "!"`, `"hi"`, compact, `"hi!"`},
		{"arr_concat", "[1,2] + [3]", "null", compact, "[1,2,3]"},
		{"arr_diff", "[1,2,3] - [2]", "null", compact, "[1,3]"},
		{"obj_merge", "{a:1} + {b:2,a:9}", "null", compact, `{"a":9,"b":2}`},
		{"str_split_div", `"a,b,c" / ","`, "null", compact, `["a","b","c"]`},
		{"cmp_lt", "1 < 2", "null", compact, "true"},
		{"cmp_eq", `. == "x"`, `"x"`, compact, "true"},
		{"null_lt_false", "null < false", "null", compact, "true"},
		{"and", "true and false", "null", compact, "false"},
		{"or", "false or true", "null", compact, "true"},
		{"not", "not", "false", compact, "true"},
		{"unary_neg", "-.", "5", compact, "-5"},

		// alternative / control flow
		{"alt_default", `.a // "default"`, `{"b":1}`, compact, `"default"`},
		{"alt_present", `.a // "default"`, `{"a":7}`, compact, "7"},
		{"alt_false", `false // 5`, "null", compact, "5"},
		{"if_then_else", `if .v==1 then "one" else "x" end`, `{"v":1}`, compact, `"one"`},
		{"if_elif", `if . < 0 then "neg" elif . == 0 then "zero" else "pos" end`, "5", compact, `"pos"`},
		{"if_no_else", `if . > 0 then "pos" end`, "-1", compact, "-1"},

		// optional / try
		{"optional_suppress", ".[]?", "5", compact, ""},
		{"try_catch", `try error("boom") catch .`, "1", compact, `"boom"`},
		{"try_no_catch", "try error", "1", compact, ""},
		{"try_success", "try (.+1)", "1", compact, "2"},

		// string interpolation
		{"interp", `"hi \(.n)!"`, `{"n":"x"}`, compact, `"hi x!"`},
		{"interp_num", `"v=\(.+1)"`, "1", compact, `"v=2"`},

		// builtins: structure / type
		{"length_arr", "length", "[1,2,3]", compact, "3"},
		{"length_str", "length", `"héllo"`, compact, "5"},
		{"length_obj", "length", `{"a":1,"b":2}`, compact, "2"},
		{"keys", "keys", `{"b":2,"a":1,"c":3}`, compact, `["a","b","c"]`},
		{"keys_unsorted", "keys_unsorted", `{"b":2,"a":1}`, compact, `["b","a"]`},
		{"keys_array", "keys", "[10,20]", compact, "[0,1]"},
		{"has", `has("a")`, `{"a":1}`, compact, "true"},
		{"in", `in({"x":1})`, `"x"`, compact, "true"},
		{"contains", `contains({a:1})`, `{"a":1,"b":2}`, compact, "true"},
		{"type", "type", `"s"`, compact, `"string"`},

		// builtins: arrays/objects
		{"add", "add", "[1,2,3]", compact, "6"},
		{"add_empty", "add", "[]", compact, "null"},
		{"any", "any", "[false,true]", compact, "true"},
		{"all", "all", "[true,false]", compact, "false"},
		{"flatten", "flatten", "[1,[2,[3]]]", compact, "[1,2,3]"},
		{"flatten_depth", "flatten(1)", "[1,[2,[3]]]", compact, "[1,2,[3]]"},
		{"range1", "[range(3)]", "null", compact, "[0,1,2]"},
		{"range2", "[range(2;5)]", "null", compact, "[2,3,4]"},
		{"range3", "[range(0;10;3)]", "null", compact, "[0,3,6,9]"},
		{"min", "min", "[3,1,2]", compact, "1"},
		{"max", "max", "[3,1,2]", compact, "3"},
		{"min_by", "min_by(.x)", `[{"x":3},{"x":1}]`, compact, `{"x":1}`},
		{"sort", "sort", "[3,1,2]", compact, "[1,2,3]"},
		{"sort_by", "sort_by(.n)", `[{"n":3},{"n":1},{"n":2}]`, compact, `[{"n":1},{"n":2},{"n":3}]`},
		{"unique", "unique", "[3,1,2,1,3]", compact, "[1,2,3]"},
		{"unique_by", "unique_by(.%2)", "[1,2,3,4]", compact, "[2,1]"},
		{"group_by", "group_by(.k)", `[{"k":1,"v":"a"},{"k":2},{"k":1,"v":"c"}]`, compact, `[[{"k":1,"v":"a"},{"k":1,"v":"c"}],[{"k":2}]]`},
		{"reverse", "reverse", "[1,2,3]", compact, "[3,2,1]"},
		{"to_entries", "to_entries", `{"a":1}`, compact, `[{"key":"a","value":1}]`},
		{"from_entries", "from_entries", `[{"key":"a","value":1}]`, compact, `{"a":1}`},
		{"with_entries", `with_entries({key:.key, value:(.value+1)})`, `{"a":1,"b":2}`, compact, `{"a":2,"b":3}`},

		// builtins: strings
		{"tostring", "tostring", "123", compact, `"123"`},
		{"tostring_obj", "tostring", `{"a":1}`, compact, `"{\"a\":1}"`},
		{"tonumber", "tonumber", `"1.5"`, compact, "1.5"},
		{"ascii_upcase", "ascii_upcase", `"aBc"`, compact, `"ABC"`},
		{"ascii_downcase", "ascii_downcase", `"aBc"`, compact, `"abc"`},
		{"ltrimstr", `ltrimstr("foo")`, `"foobar"`, compact, `"bar"`},
		{"rtrimstr", `rtrimstr("bar")`, `"foobar"`, compact, `"foo"`},
		{"startswith", `startswith("foo")`, `"foobar"`, compact, "true"},
		{"endswith", `endswith("baz")`, `"foobar"`, compact, "false"},
		{"split", `split(",")`, `"a,b,c"`, compact, `["a","b","c"]`},
		{"join", `join("-")`, `["a","b","c"]`, compact, `"a-b-c"`},
		{"join_mixed", `join(",")`, `[1,"x",null]`, compact, `"1,x,"`},
		{"test", `test("^h")`, `"hello"`, compact, "true"},

		// builtins: higher order
		{"map", "map(. + 1)", "[10,20,30]", compact, "[11,21,31]"},
		{"map_values", "map_values(. * 2)", `{"a":1,"b":2}`, compact, `{"a":2,"b":4}`},
		{"select_stream", "[.[] | select(. > 15)]", "[10,20,30]", compact, "[20,30]"},
		{"recurse_f", "[recurse(.children[]?)]", `{"v":1,"children":[{"v":2},{"v":3}]}`, compact, `[{"v":1,"children":[{"v":2},{"v":3}]},{"v":2},{"v":3}]`},
		{"paths", "[paths]", `{"a":[1]}`, compact, `[["a"],["a",0]]`},
		{"getpath", `getpath(["a","b"])`, `{"a":{"b":7}}`, compact, "7"},
		{"setpath", `setpath(["a","b"];9)`, `{"a":{"b":1}}`, compact, `{"a":{"b":9}}`},
		{"del_field", "del(.b)", `{"a":1,"b":2,"c":3}`, compact, `{"a":1,"c":3}`},
		{"del_index", "del(.[1])", "[10,20,30]", compact, "[10,30]"},
		{"del_iterate", "del(.[])", "[1,2,3]", compact, "[]"},
		{"walk", `walk(if type=="number" then .+1 else . end)`, `{"a":1,"b":[2,3]}`, compact, `{"a":2,"b":[3,4]}`},

		// builtins: io-ish & formats
		{"tojson", "tojson", `{"a":1}`, compact, `"{\"a\":1}"`},
		{"fromjson", "fromjson", `"[1,2,3]"`, compact, "[1,2,3]"},
		{"at_base64", "@base64", `"hello"`, compact, `"aGVsbG8="`},
		{"at_base64d", "@base64d", `"aGVsbG8="`, compact, `"hello"`},
		{"at_json", "@json", `{"a":1}`, compact, `"{\"a\":1}"`},
		{"at_csv", "@csv", `[1,"x",true]`, compact, `"1,\"x\",true"`},
		{"type_filter_strings", "[.[] | strings]", `[1,"a",true,"b"]`, compact, `["a","b"]`},
		{"type_filter_values", "[.[] | values]", "[1,null,2,null]", compact, "[1,2]"},

		// pretty output
		{"pretty_obj", ".", `{"a":1}`, EncodeOptions{}, "{\n  \"a\": 1\n}"},
		{"pretty_arr", ".", "[1,2]", EncodeOptions{}, "[\n  1,\n  2\n]"},
		{"raw_string", ".name", `{"name":"ccjq"}`, EncodeOptions{Raw: true}, "ccjq"},
		{"sort_keys", ".", `{"b":1,"a":2}`, EncodeOptions{Compact: true, SortKeys: true}, `{"a":2,"b":1}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := evalString(c.filter, c.input, c.opts)
			if err != nil {
				t.Fatalf("filter %q: unexpected error: %v", c.filter, err)
			}
			if got != c.want {
				t.Errorf("filter %q on %s\n got: %q\nwant: %q", c.filter, c.input, got, c.want)
			}
		})
	}
}

// TestEvalErrors checks that bad runtime operations raise (catchable) errors.
func TestEvalErrors(t *testing.T) {
	cases := []struct {
		name   string
		filter string
		input  string
		errSub string
	}{
		{"index_number", ".foo", "1", `Cannot index number with string "foo"`},
		{"iterate_scalar", ".[]", "5", "Cannot iterate over number"},
		{"div_zero", ". / 0", "1", "divisor is zero"},
		{"add_types", `1 + "x"`, "null", "cannot be added"},
		{"explicit_error", `error("custom")`, "null", "custom"},
		{"keys_on_number", "keys", "5", "has no keys"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := evalString(c.filter, c.input, compact)
			if err == nil {
				t.Fatalf("expected error for %q", c.filter)
			}
			if _, ok := err.(*EvalError); !ok {
				t.Fatalf("expected *EvalError, got %T: %v", err, err)
			}
			if !strings.Contains(err.Error(), c.errSub) {
				t.Errorf("error %q does not contain %q", err.Error(), c.errSub)
			}
		})
	}
}

// TestCompileErrors checks that malformed filters fail to compile.
func TestCompileErrors(t *testing.T) {
	bad := []string{
		".[",
		"{a:",
		"if . then",
		"(1",
		"1 +",
		"nonexistent_builtin",
		"map",       // wrong arity
		"length(1)", // wrong arity
		`"unterminated`,
	}
	for _, src := range bad {
		t.Run(src, func(t *testing.T) {
			if _, err := Compile(src); err == nil {
				t.Errorf("expected compile error for %q", src)
			}
		})
	}
}

func TestStreamDecode(t *testing.T) {
	dec := NewDecoder(strings.NewReader("1 2 3"))
	var got []string
	for {
		v, err := dec.Decode()
		if err != nil {
			break
		}
		got = append(got, Encode(v, compact))
	}
	if strings.Join(got, ",") != "1,2,3" {
		t.Errorf("stream decode got %v", got)
	}
}
