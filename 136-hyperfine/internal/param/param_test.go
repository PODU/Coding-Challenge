package param

import (
	"reflect"
	"strings"
	"testing"
)

func values(t *testing.T, bs []Binding, name string) []string {
	t.Helper()
	out := make([]string, 0, len(bs))
	for _, b := range bs {
		out = append(out, b.Values[name])
	}
	return out
}

func TestScanInteger(t *testing.T) {
	bs, err := Expand([]Scan{{Var: "n", Min: "1", Max: "4"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"1", "2", "3", "4"}
	if got := values(t, bs, "n"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestScanWithStep(t *testing.T) {
	bs, err := Expand([]Scan{{Var: "n", Min: "1", Max: "8", Step: "3"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"1", "4", "7"}
	if got := values(t, bs, "n"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestScanDecimalStepAvoidsFloatDrift(t *testing.T) {
	bs, err := Expand([]Scan{{Var: "d", Min: "0.1", Max: "0.5", Step: "0.1"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"0.1", "0.2", "0.3", "0.4", "0.5"}
	if got := values(t, bs, "d"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestScanDescending(t *testing.T) {
	bs, err := Expand([]Scan{{Var: "n", Min: "5", Max: "1", Step: "-2"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"5", "3", "1"}
	if got := values(t, bs, "n"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestScanErrors(t *testing.T) {
	cases := []struct {
		name string
		scan Scan
		want string
	}{
		{"zero step", Scan{Var: "n", Min: "1", Max: "5", Step: "0"}, "must not be zero"},
		{"wrong direction", Scan{Var: "n", Min: "1", Max: "5", Step: "-1"}, "never reaches"},
		{"not a number", Scan{Var: "n", Min: "a", Max: "5"}, "is not a number"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Expand([]Scan{c.scan}, nil)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %v, want one containing %q", err, c.want)
			}
		})
	}
}

func TestListCartesianProduct(t *testing.T) {
	bs, err := Expand(nil, []List{
		{Var: "cc", Values: []string{"gcc", "clang"}},
		{Var: "opt", Values: []string{"O0", "O2"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bs) != 4 {
		t.Fatalf("got %d bindings, want 4", len(bs))
	}
	var got []string
	for _, b := range bs {
		got = append(got, b.Substitute("{cc} -{opt}"))
	}
	want := []string{"gcc -O0", "gcc -O2", "clang -O0", "clang -O2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestScanAndListAreExclusive(t *testing.T) {
	_, err := Expand([]Scan{{Var: "n", Min: "1", Max: "2"}}, []List{{Var: "x", Values: []string{"a"}}})
	if err == nil {
		t.Fatal("expected an error when combining --parameter-scan and --parameter-list")
	}
}

func TestExpandWithNoParametersYieldsOneBinding(t *testing.T) {
	bs, err := Expand(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(bs) != 1 {
		t.Fatalf("got %d bindings, want 1", len(bs))
	}
	if got := bs[0].Substitute("echo {n}"); got != "echo {n}" {
		t.Errorf("unbound placeholder was rewritten: %q", got)
	}
}
