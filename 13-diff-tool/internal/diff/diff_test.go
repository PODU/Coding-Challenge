package diff

import "testing"

func TestDiff(t *testing.T) {
	cases := []struct {
		name string
		a, b []string
		want []string
	}{
		{
			name: "identical",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "b", "c"},
			want: nil,
		},
		{
			name: "both empty",
			a:    nil,
			b:    nil,
			want: nil,
		},
		{
			name: "pure additions",
			a:    []string{"a", "b"},
			b:    []string{"a", "x", "b", "y"},
			want: []string{"> x", "> y"},
		},
		{
			name: "pure deletions",
			a:    []string{"a", "x", "b", "y"},
			b:    []string{"a", "b"},
			want: []string{"< x", "< y"},
		},
		{
			name: "changed line: delete before add",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "B", "c"},
			want: []string{"< b", "> B"},
		},
		{
			name: "empty vs non-empty (all added)",
			a:    nil,
			b:    []string{"a", "b"},
			want: []string{"> a", "> b"},
		},
		{
			name: "non-empty vs empty (all deleted)",
			a:    []string{"a", "b"},
			b:    nil,
			want: []string{"< a", "< b"},
		},
		{
			name: "blank lines are significant",
			a:    []string{"a", "", "b"},
			b:    []string{"a", "b"},
			want: []string{"< "},
		},
		{
			name: "mixed interleaving",
			a:    []string{"one", "two", "three", "four"},
			b:    []string{"zero", "one", "three", "FOUR"},
			want: []string{"> zero", "< two", "< four", "> FOUR"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Diff(tc.a, tc.b)
			if !equalStrings(got, tc.want) {
				t.Errorf("Diff(%v,%v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
			// The Myers path must produce identical output.
			if gotM := DiffMyers(tc.a, tc.b); !equalStrings(gotM, tc.want) {
				t.Errorf("DiffMyers(%v,%v) = %v, want %v", tc.a, tc.b, gotM, tc.want)
			}
		})
	}
}

func TestDiffAutoEquivalence(t *testing.T) {
	a := []string{"a", "b", "c", "d"}
	b := []string{"a", "x", "c", "d", "e"}
	want := Diff(a, b)
	if got := DiffAuto(a, b); !equalStrings(got, want) {
		t.Errorf("DiffAuto = %v, want %v", got, want)
	}
}
