package cut

import "testing"

func TestParseList_Valid(t *testing.T) {
	tests := []struct {
		name string
		spec string
		// probe maps a 1-indexed position to whether it should be selected.
		probe map[int]bool
	}{
		{"single", "1", map[int]bool{1: true, 2: false}},
		{"comma list", "1,2", map[int]bool{1: true, 2: true, 3: false}},
		{"space list", "1 2", map[int]bool{1: true, 2: true, 3: false}},
		{"mixed sep", "1, 3", map[int]bool{1: true, 2: false, 3: true}},
		{"closed range", "2-5", map[int]bool{1: false, 2: true, 5: true, 6: false}},
		{"open range", "3-", map[int]bool{2: false, 3: true, 100: true}},
		{"to-M range", "-4", map[int]bool{1: true, 4: true, 5: false}},
		{"overlapping", "1-3,2-4", map[int]bool{1: true, 4: true, 5: false}},
		{"adjacent merge", "1-2,3-4", map[int]bool{1: true, 4: true, 5: false}},
		{"out of order", "5,1", map[int]bool{1: true, 5: true, 3: false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sel, err := ParseList(tt.spec, false)
			if err != nil {
				t.Fatalf("ParseList(%q) unexpected error: %v", tt.spec, err)
			}
			for pos, want := range tt.probe {
				if got := sel.Selected(pos); got != want {
					t.Errorf("Selected(%d) = %v, want %v", pos, got, want)
				}
			}
		})
	}
}

func TestParseList_Errors(t *testing.T) {
	tests := []struct {
		name string
		spec string
	}{
		{"empty", ""},
		{"zero", "0"},
		{"decreasing", "5-2"},
		{"non-numeric", "a"},
		{"negative-ish", "-"},
		{"zero in range", "0-3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseList(tt.spec, false); err == nil {
				t.Errorf("ParseList(%q) expected error, got nil", tt.spec)
			}
		})
	}
}

func TestSelector_Complement(t *testing.T) {
	sel, err := ParseList("2-4", true)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int]bool{1: true, 2: false, 3: false, 4: false, 5: true, 10: true}
	for pos, w := range want {
		if got := sel.Selected(pos); got != w {
			t.Errorf("complement Selected(%d) = %v, want %v", pos, got, w)
		}
	}
}

func TestSelector_ComplementOpenRange(t *testing.T) {
	// Complement of "1-" selects nothing.
	sel, err := ParseList("1-", true)
	if err != nil {
		t.Fatal(err)
	}
	for pos := 1; pos <= 5; pos++ {
		if sel.Selected(pos) {
			t.Errorf("complement of 1- should select nothing, but Selected(%d)=true", pos)
		}
	}
}
