package value

import (
	"math"
	"testing"
)

func TestResolvePlain(t *testing.T) {
	cases := []struct {
		in   string
		kind Kind
	}{
		// Null
		{"", Null}, {"~", Null}, {"null", Null}, {"Null", Null}, {"NULL", Null},
		// Bool (YAML 1.1 family, case-insensitive)
		{"true", Bool}, {"True", Bool}, {"FALSE", Bool},
		{"yes", Bool}, {"no", Bool}, {"on", Bool}, {"off", Bool}, {"Off", Bool},
		// Int
		{"0", Int}, {"42", Int}, {"-7", Int}, {"+3", Int},
		{"0x12d4", Int}, {"023332", Int}, {"0o23332", Int}, {"0b1010", Int},
		// Int pitfalls -> string (leading-zero non-octal, underscores)
		{"08", Str}, {"1_2_3", Str},
		// Float
		{"3.14", Float}, {"-0.5", Float}, {"1e10", Float}, {"6.8e-3", Float}, {".5", Float},
		// Float specials
		{".inf", Float}, {"-.inf", Float}, {"+.inf", Float}, {".nan", Float}, {".NaN", Float},
		// Strings
		{"hello", Str}, {"hello world", Str}, {"inf", Str}, {"nan", Str},
		{"3.14.15", Str}, {"0xZZ", Str},
	}
	for _, c := range cases {
		got := ResolvePlain(c.in)
		if got.Kind != c.kind {
			t.Errorf("ResolvePlain(%q).Kind = %v, want %v", c.in, got.Kind, c.kind)
		}
	}
}

func TestResolveValues(t *testing.T) {
	if v := ResolvePlain("0x12d4"); v.Int != 4820 {
		t.Errorf("0x12d4 = %d, want 4820", v.Int)
	}
	if v := ResolvePlain("023332"); v.Int != 9946 {
		t.Errorf("023332 = %d, want 9946", v.Int)
	}
	if v := ResolvePlain("off"); v.Bool != false {
		t.Errorf("off should be false")
	}
	if v := ResolvePlain("on"); v.Bool != true {
		t.Errorf("on should be true")
	}
	if v := ResolvePlain(".inf"); !math.IsInf(v.Float, 1) {
		t.Errorf(".inf should be +Inf")
	}
	if v := ResolvePlain("-.inf"); !math.IsInf(v.Float, -1) {
		t.Errorf("-.inf should be -Inf")
	}
	if v := ResolvePlain(".nan"); !math.IsNaN(v.Float) {
		t.Errorf(".nan should be NaN")
	}
}
