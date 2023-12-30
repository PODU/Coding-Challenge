package tr

import (
	"bytes"
	"testing"

	"github.com/podu2997/tr/internal/set"
)

func build(t *testing.T, cfg Config) *Engine {
	t.Helper()
	e, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	return e
}

func mustParse(t *testing.T, s string) *set.Set {
	t.Helper()
	st, err := set.Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return st
}

func apply(t *testing.T, cfg Config, in string) string {
	t.Helper()
	e := build(t, cfg)
	var out bytes.Buffer
	if err := e.Run(bytes.NewReader([]byte(in)), &out); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	return out.String()
}

func TestTranslateMap(t *testing.T) {
	cfg := Config{Set1: mustParse(t, "abc"), Set2: mustParse(t, "xyz"), HasSet2: true}
	if got := apply(t, cfg, "cab"); got != "zxy" {
		t.Errorf("got %q", got)
	}
}

func TestPadLastChar(t *testing.T) {
	cfg := Config{Set1: mustParse(t, "abc"), Set2: mustParse(t, "x"), HasSet2: true}
	if got := apply(t, cfg, "abc"); got != "xxx" {
		t.Errorf("got %q", got)
	}
}

func TestTruncateSet1(t *testing.T) {
	cfg := Config{Set1: mustParse(t, "abc"), Set2: mustParse(t, "xy"), HasSet2: true, Truncate: true}
	if got := apply(t, cfg, "abc"); got != "xyc" {
		t.Errorf("got %q", got)
	}
}

func TestDeleteMembership(t *testing.T) {
	cfg := Config{Delete: true, Set1: mustParse(t, "a-c")}
	if got := apply(t, cfg, "abcd"); got != "d" {
		t.Errorf("got %q", got)
	}
}

func TestDeleteComplement(t *testing.T) {
	cfg := Config{Delete: true, Complement: true, Set1: mustParse(t, "a-c")}
	if got := apply(t, cfg, "abcd"); got != "abc" {
		t.Errorf("got %q", got)
	}
}

func TestSqueezeOnly(t *testing.T) {
	cfg := Config{Squeeze: true, Set1: mustParse(t, "a")}
	if got := apply(t, cfg, "aaabbbaa"); got != "abbba" {
		t.Errorf("got %q", got)
	}
}

func TestTranslateThenSqueeze(t *testing.T) {
	cfg := Config{Squeeze: true, Set1: mustParse(t, "a-z"), Set2: mustParse(t, "A-Z"), HasSet2: true}
	if got := apply(t, cfg, "aabb"); got != "AB" {
		t.Errorf("got %q", got)
	}
}

func TestDeleteThenSqueeze(t *testing.T) {
	cfg := Config{Delete: true, Squeeze: true, Set1: mustParse(t, "a"), Set2: mustParse(t, "b"), HasSet2: true}
	if got := apply(t, cfg, "aabbb"); got != "b" {
		t.Errorf("got %q", got)
	}
}

func TestEmptySet2Error(t *testing.T) {
	_, err := Build(Config{Set1: mustParse(t, "a"), Set2: mustParse(t, ""), HasSet2: true})
	if err == nil {
		t.Error("expected empty SET2 error")
	}
}

func TestIllegalClassSet2Error(t *testing.T) {
	_, err := Build(Config{Set1: mustParse(t, "a-z"), Set2: mustParse(t, "[:digit:]"), HasSet2: true})
	if err == nil {
		t.Error("expected illegal class error")
	}
}

func TestComplementTranslateMapsToLast(t *testing.T) {
	cfg := Config{Complement: true, Set1: mustParse(t, "[:alpha:]"), Set2: mustParse(t, "#"), HasSet2: true}
	if got := apply(t, cfg, "a1b2"); got != "a#b#" {
		t.Errorf("got %q", got)
	}
}
