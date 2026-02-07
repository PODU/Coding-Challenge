package validator

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// mustParse decodes a JSON literal for use in tests, failing on error.
func mustParse(t *testing.T, s string) interface{} {
	t.Helper()
	v, err := ParseJSON([]byte(s))
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return v
}

// errKey renders an error as "<instancePath>|<keyword>" for compact assertions.
func errKey(e ValidationError) string {
	return e.InstancePath + "|" + e.Keyword
}

// validateLiterals is a convenience that parses schema+instance and validates.
func validateLiterals(t *testing.T, schema, instance string, opts Options) Result {
	t.Helper()
	s := mustParse(t, schema)
	i := mustParse(t, instance)
	return New(s, opts).Validate(i)
}

type caseSpec struct {
	name     string
	schema   string
	instance string
	// want is the set of expected "<instancePath>|<keyword>" error keys.
	// nil/empty means the instance must be valid.
	want []string
	opts Options
}

func TestKeywords(t *testing.T) {
	cases := []caseSpec{
		// ---- type ----
		{name: "type/integer ok", schema: `{"type":"integer"}`, instance: `42`},
		{name: "type/integer on 1.0 ok", schema: `{"type":"integer"}`, instance: `1.0`},
		{name: "type/integer fail", schema: `{"type":"integer"}`, instance: `4.5`,
			want: []string{"|type"}},
		{name: "type/array of types ok", schema: `{"type":["string","null"]}`, instance: `null`},
		{name: "type/array of types fail", schema: `{"type":["string","null"]}`, instance: `5`,
			want: []string{"|type"}},
		{name: "type/string vs number no coercion", schema: `{"type":"number"}`, instance: `"5"`,
			want: []string{"|type"}},

		// ---- enum / const ----
		{name: "enum ok", schema: `{"enum":["a","b",3]}`, instance: `3`},
		{name: "enum fail", schema: `{"enum":["a","b"]}`, instance: `"c"`,
			want: []string{"|enum"}},
		{name: "const ok numeric value-eq", schema: `{"const":1.0}`, instance: `1`},
		{name: "const fail", schema: `{"const":"x"}`, instance: `"y"`,
			want: []string{"|const"}},

		// ---- numeric ----
		{name: "minimum ok", schema: `{"minimum":10}`, instance: `10`},
		{name: "minimum fail", schema: `{"properties":{"age":{"minimum":18}}}`, instance: `{"age":17}`,
			want: []string{"/age|minimum"}},
		{name: "maximum fail", schema: `{"maximum":100}`, instance: `101`,
			want: []string{"|maximum"}},
		{name: "exclusiveMinimum fail", schema: `{"exclusiveMinimum":0}`, instance: `0`,
			want: []string{"|exclusiveMinimum"}},
		{name: "exclusiveMaximum ok", schema: `{"exclusiveMaximum":1}`, instance: `0.999`},
		{name: "multipleOf ok", schema: `{"multipleOf":3}`, instance: `9`},
		{name: "multipleOf float-safe ok", schema: `{"multipleOf":0.1}`, instance: `0.3`},
		{name: "multipleOf fail", schema: `{"multipleOf":2}`, instance: `7`,
			want: []string{"|multipleOf"}},
		{name: "multipleOf bigint ok", schema: `{"multipleOf":1}`, instance: `100000000000000000000000`},

		// ---- string ----
		{name: "minLength ok unicode", schema: `{"minLength":3}`, instance: `"héy"`},
		{name: "minLength fail", schema: `{"properties":{"name":{"minLength":3}}}`, instance: `{"name":"ab"}`,
			want: []string{"/name|minLength"}},
		{name: "maxLength fail unicode counts runes", schema: `{"maxLength":2}`, instance: `"abc"`,
			want: []string{"|maxLength"}},
		{name: "pattern ok", schema: `{"pattern":"^a.c$"}`, instance: `"abc"`},
		{name: "pattern fail", schema: `{"properties":{"sku":{"pattern":"^[0-9]+$"}}}`, instance: `{"sku":"x1"}`,
			want: []string{"/sku|pattern"}},

		// ---- format (assert) ----
		{name: "format email ok", schema: `{"format":"email"}`, instance: `"a@b.com"`,
			opts: Options{AssertFormat: true}},
		{name: "format email fail", schema: `{"format":"email"}`, instance: `"not-an-email"`,
			want: []string{"|format"}, opts: Options{AssertFormat: true}},
		{name: "format off by default (annotation)", schema: `{"format":"email"}`, instance: `"nope"`},
		{name: "format date ok", schema: `{"format":"date"}`, instance: `"2020-01-02"`,
			opts: Options{AssertFormat: true}},
		{name: "format date-time ok", schema: `{"format":"date-time"}`, instance: `"2020-01-02T03:04:05Z"`,
			opts: Options{AssertFormat: true}},
		{name: "format ipv4 fail", schema: `{"format":"ipv4"}`, instance: `"999.1.1.1"`,
			want: []string{"|format"}, opts: Options{AssertFormat: true}},
		{name: "format uuid ok", schema: `{"format":"uuid"}`, instance: `"12345678-1234-1234-1234-123456789abc"`,
			opts: Options{AssertFormat: true}},
		{name: "format uri ok", schema: `{"format":"uri"}`, instance: `"https://example.com/x"`,
			opts: Options{AssertFormat: true}},

		// ---- object: properties / required ----
		{name: "required ok", schema: `{"required":["email"]}`, instance: `{"email":"x"}`},
		{name: "required fail", schema: `{"required":["email","name"]}`, instance: `{"name":"x"}`,
			want: []string{"|required"}},
		{name: "properties nested type fail", schema: `{"properties":{"address":{"properties":{"zip":{"type":"string"}}}}}`,
			instance: `{"address":{"zip":5}}`, want: []string{"/address/zip|type"}},

		// ---- object: additionalProperties / patternProperties ----
		{name: "additionalProperties false fail", schema: `{"properties":{"a":{}},"additionalProperties":false}`,
			instance: `{"a":1,"extra":2}`, want: []string{"/extra|additionalProperties"}},
		{name: "additionalProperties schema fail", schema: `{"additionalProperties":{"type":"string"}}`,
			instance: `{"x":1}`, want: []string{"/x|type"}},
		{name: "patternProperties ok", schema: `{"patternProperties":{"^s_":{"type":"string"}}}`,
			instance: `{"s_name":"ok"}`},
		{name: "patternProperties fail", schema: `{"patternProperties":{"^s_":{"type":"string"}}}`,
			instance: `{"s_age":5}`, want: []string{"/s_age|type"}},
		{name: "additionalProperties covered by pattern", schema: `{"patternProperties":{"^x":{}},"additionalProperties":false}`,
			instance: `{"xyz":1}`},

		// ---- object: counts / propertyNames ----
		{name: "minProperties fail", schema: `{"minProperties":2}`, instance: `{"a":1}`,
			want: []string{"|minProperties"}},
		{name: "maxProperties fail", schema: `{"maxProperties":1}`, instance: `{"a":1,"b":2}`,
			want: []string{"|maxProperties"}},
		{name: "propertyNames fail", schema: `{"propertyNames":{"maxLength":3}}`, instance: `{"toolong":1}`,
			want: []string{"/toolong|propertyNames"}},

		// ---- arrays ----
		{name: "items list ok", schema: `{"items":{"type":"integer"}}`, instance: `[1,2,3]`},
		{name: "items list fail path", schema: `{"items":{"type":"integer"}}`, instance: `[1,2,"x"]`,
			want: []string{"/2|type"}},
		{name: "items tuple ok", schema: `{"items":[{"type":"integer"},{"type":"string"}]}`, instance: `[1,"a"]`},
		{name: "items tuple fail", schema: `{"items":[{"type":"integer"},{"type":"string"}]}`, instance: `[1,2]`,
			want: []string{"/1|type"}},
		{name: "additionalItems false fail", schema: `{"items":[{"type":"integer"}],"additionalItems":false}`,
			instance: `[1,2]`, want: []string{"/1|additionalItems"}},
		{name: "additionalItems schema ok", schema: `{"items":[{"type":"integer"}],"additionalItems":{"type":"string"}}`,
			instance: `[1,"a","b"]`},
		{name: "additionalItems schema fail", schema: `{"items":[{"type":"integer"}],"additionalItems":{"type":"string"}}`,
			instance: `[1,"a",3]`, want: []string{"/2|type"}},
		{name: "minItems fail", schema: `{"minItems":2}`, instance: `[1]`,
			want: []string{"|minItems"}},
		{name: "maxItems fail", schema: `{"maxItems":1}`, instance: `[1,2]`,
			want: []string{"|maxItems"}},
		{name: "uniqueItems ok", schema: `{"uniqueItems":true}`, instance: `[1,2,3]`},
		{name: "uniqueItems fail", schema: `{"properties":{"values":{"uniqueItems":true}}}`,
			instance: `{"values":[1,2,1]}`, want: []string{"/values|uniqueItems"}},
		{name: "uniqueItems deep objects fail", schema: `{"uniqueItems":true}`,
			instance: `[{"a":1},{"a":1}]`, want: []string{"|uniqueItems"}},
		{name: "contains ok", schema: `{"contains":{"type":"integer"}}`, instance: `["a",1]`},
		{name: "contains fail", schema: `{"contains":{"type":"integer"}}`, instance: `["a","b"]`,
			want: []string{"|contains"}},

		// ---- combinators ----
		{name: "allOf ok", schema: `{"allOf":[{"type":"integer"},{"minimum":0}]}`, instance: `5`},
		{name: "allOf fail surfaces inner", schema: `{"allOf":[{"type":"integer"},{"minimum":10}]}`, instance: `5`,
			want: []string{"|minimum"}},
		{name: "anyOf ok", schema: `{"anyOf":[{"type":"string"},{"type":"integer"}]}`, instance: `5`},
		{name: "anyOf fail at root", schema: `{"anyOf":[{"type":"string"},{"type":"boolean"}]}`, instance: `5`,
			want: []string{"|anyOf"}},
		{name: "oneOf ok exactly one", schema: `{"oneOf":[{"type":"integer"},{"type":"string"}]}`, instance: `5`},
		{name: "oneOf fail zero", schema: `{"oneOf":[{"type":"string"},{"type":"boolean"}]}`, instance: `5`,
			want: []string{"|oneOf"}},
		{name: "oneOf fail more than one", schema: `{"oneOf":[{"minimum":0},{"type":"integer"}]}`, instance: `5`,
			want: []string{"|oneOf"}},
		{name: "not ok", schema: `{"not":{"type":"string"}}`, instance: `5`},
		{name: "not fail", schema: `{"not":{"type":"integer"}}`, instance: `5`,
			want: []string{"|not"}},

		// ---- if / then / else ----
		{name: "if-then ok", schema: `{"if":{"properties":{"t":{"const":"a"}}},"then":{"required":["x"]}}`,
			instance: `{"t":"a","x":1}`},
		{name: "if-then fail", schema: `{"if":{"properties":{"t":{"const":"a"}},"required":["t"]},"then":{"required":["x"]}}`,
			instance: `{"t":"a"}`, want: []string{"|required"}},
		{name: "if-else fail", schema: `{"if":{"properties":{"t":{"const":"a"}},"required":["t"]},"then":{"required":["x"]},"else":{"required":["y"]}}`,
			instance: `{"t":"b"}`, want: []string{"|required"}},

		// ---- boolean schemas ----
		{name: "true schema accepts", schema: `true`, instance: `{"anything":1}`},
		{name: "false schema rejects", schema: `false`, instance: `1`, want: []string{"|false"}},
		{name: "false subschema property", schema: `{"properties":{"a":false}}`, instance: `{"a":1}`,
			want: []string{"/a|false"}},
		{name: "empty schema accepts", schema: `{}`, instance: `null`},

		// ---- $ref ----
		{name: "ref ok", schema: `{"definitions":{"pos":{"minimum":0}},"$ref":"#/definitions/pos"}`, instance: `5`},
		{name: "ref fail", schema: `{"definitions":{"pos":{"minimum":0}},"properties":{"n":{"$ref":"#/definitions/pos"}}}`,
			instance: `{"n":-1}`, want: []string{"/n|minimum"}},
		{name: "ref recursive deep path", schema: `{"definitions":{"node":{"properties":{"value":{"type":"integer"},"next":{"$ref":"#/definitions/node"}}}},"$ref":"#/definitions/node"}`,
			instance: `{"value":1,"next":{"value":"bad"}}`, want: []string{"/next/value|type"}},
		{name: "ref root recursion", schema: `{"properties":{"child":{"$ref":"#"}}}`,
			instance: `{"child":{"child":{}}}`},

		// ---- multiple errors collected, nested ----
		{name: "collect all errors", schema: `{"properties":{"a":{"type":"integer"},"b":{"type":"string"}},"required":["c"]}`,
			instance: `{"a":"x","b":5}`, want: []string{"/a|type", "/b|type", "|required"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := validateLiterals(t, tc.schema, tc.instance, tc.opts)

			got := make([]string, 0, len(res.Errors))
			for _, e := range res.Errors {
				got = append(got, errKey(e))
			}
			sort.Strings(got)

			want := append([]string(nil), tc.want...)
			sort.Strings(want)

			if len(tc.want) == 0 {
				if !res.Valid {
					t.Fatalf("expected valid, got errors: %s", strings.Join(got, ", "))
				}
				return
			}
			if res.Valid {
				t.Fatalf("expected invalid, but valid")
			}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("error keys mismatch\n got: %s\nwant: %s\nfull: %+v",
					strings.Join(got, ", "), strings.Join(want, ", "), res.Errors)
			}
		})
	}
}

func TestPointerRender(t *testing.T) {
	tests := []struct {
		tokens []string
		want   string
	}{
		{nil, ""},
		{[]string{"a"}, "/a"},
		{[]string{"a", "b"}, "/a/b"},
		{[]string{"a/b"}, "/a~1b"},
		{[]string{"m~n"}, "/m~0n"},
		{[]string{"~/"}, "/~0~1"},
	}
	for _, tt := range tests {
		var p Pointer = tt.tokens
		if got := p.String(); got != tt.want {
			t.Errorf("Pointer(%v).String() = %q, want %q", tt.tokens, got, tt.want)
		}
	}
	if Pointer(nil).Human() != "(root)" {
		t.Errorf("root Human() should be (root)")
	}
}

func TestParsePointerRoundTrip(t *testing.T) {
	toks, ok := parsePointer("/a~1b/m~0n")
	if !ok {
		t.Fatal("parse failed")
	}
	if len(toks) != 2 || toks[0] != "a/b" || toks[1] != "m~n" {
		t.Fatalf("unexpected tokens: %v", toks)
	}
	if _, ok := parsePointer("noslash"); ok {
		t.Fatal("expected failure for pointer without leading slash")
	}
	if toks, ok := parsePointer(""); !ok || len(toks) != 0 {
		t.Fatal("empty pointer should be root")
	}
}

func TestDeepEqual(t *testing.T) {
	eq := func(a, b string) bool {
		return deepEqual(mustParse(t, a), mustParse(t, b))
	}
	if !eq(`1`, `1.0`) {
		t.Error("1 should equal 1.0")
	}
	if !eq(`10e-1`, `1`) {
		t.Error("10e-1 should equal 1")
	}
	if !eq(`{"a":1,"b":2}`, `{"b":2,"a":1}`) {
		t.Error("objects equal regardless of key order")
	}
	if !eq(`[1,[2,3]]`, `[1,[2,3]]`) {
		t.Error("nested arrays equal")
	}
	if eq(`null`, `0`) {
		t.Error("null != 0")
	}
	if eq(`"1"`, `1`) {
		t.Error("string != number")
	}
	if eq(`{"a":1}`, `{"a":1,"b":2}`) {
		t.Error("different key counts not equal")
	}
}

func TestNumericEdgeCases(t *testing.T) {
	if !isInteger(mustParse(t, `1.0`)) {
		t.Error("1.0 should be integer")
	}
	if isInteger(mustParse(t, `1.5`)) {
		t.Error("1.5 should not be integer")
	}
	big := mustParse(t, `100000000000000000000000000000001`)
	if !isInteger(big) {
		t.Error("huge integer should be integer")
	}
}

func TestRefResolverAndCycle(t *testing.T) {
	schema := mustParse(t, `{"definitions":{"x":{"type":"integer"}},"$ref":"#/definitions/x"}`)
	v := New(schema, Options{})
	target, ok := v.resolveRef("#/definitions/x")
	if !ok {
		t.Fatal("could not resolve ref")
	}
	if m, _ := asMap(target); m["type"] != "integer" {
		t.Fatalf("resolved wrong target: %v", target)
	}
	if _, ok := v.resolveRef("#/definitions/missing"); ok {
		t.Fatal("expected missing ref to fail")
	}
	// Whole-document ref.
	if _, ok := v.resolveRef("#"); !ok {
		t.Fatal("root ref should resolve")
	}

	// Cyclic schema must not loop forever and must validate correctly.
	cyc := mustParse(t, `{"properties":{"self":{"$ref":"#"}}}`)
	res := New(cyc, Options{}).Validate(mustParse(t, `{"self":{"self":{"self":{}}}}`))
	if !res.Valid {
		t.Fatalf("cyclic schema validation failed: %+v", res.Errors)
	}
}

func TestMaxDepthGuard(t *testing.T) {
	// Deeply nested schema following $ref against deeply nested instance.
	schema := mustParse(t, `{"properties":{"a":{"$ref":"#"}}}`)
	// Build a deep instance.
	inst := "{}"
	for i := 0; i < 50; i++ {
		inst = fmt.Sprintf(`{"a":%s}`, inst)
	}
	res := New(schema, Options{MaxDepth: 10}).Validate(mustParse(t, inst))
	// Should not panic; depth guard may or may not trigger but must terminate.
	_ = res
}

func TestMalformedJSON(t *testing.T) {
	if _, err := ParseJSON([]byte(`{`)); err == nil {
		t.Error("expected error for malformed JSON")
	}
	if _, err := ParseJSON([]byte(`{} trailing`)); err == nil {
		t.Error("expected error for trailing content")
	}
	if _, err := ParseJSON([]byte(`{"a":1}`)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
