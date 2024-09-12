package sudoku

import "testing"

// solvedEasy is the unique solution to easyPuzzle (a classic fixture).
const solvedEasy = "534678912672195348198342567859761423426853791713924856961537284287419635345286179"

func mustParse(t *testing.T, s string) *Board {
	t.Helper()
	b, err := ParseBoard(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return b
}

func TestIsLegalDuplicates(t *testing.T) {
	dots := func(n int) string { return repeat(".", n) }
	cases := []struct {
		name  string
		board string
		legal bool
	}{
		{"row dup", "11" + dots(79), false},
		{"col dup", "1" + dots(8) + "1" + dots(71), false},
		{"box dup", "1" + dots(9) + "1" + dots(70), false},
		{"legal partial", easyPuzzle, true},
		{"legal solved", solvedEasy, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsLegal(mustParse(t, tc.board)); got != tc.legal {
				t.Errorf("IsLegal = %v, want %v", got, tc.legal)
			}
		})
	}
}

func TestIsSolved(t *testing.T) {
	if !IsSolved(mustParse(t, solvedEasy)) {
		t.Error("known solution not reported solved")
	}
	if IsSolved(mustParse(t, easyPuzzle)) {
		t.Error("partial board reported solved")
	}
	// One cell changed to create a duplicate.
	broken := []byte(solvedEasy)
	broken[1] = broken[0] // make (0,1) duplicate (0,0)
	if IsSolved(mustParse(t, string(broken))) {
		t.Error("board with a duplicate reported solved")
	}
}

func TestPermutationBreaksBox(t *testing.T) {
	// Each row and column is a permutation of 1..9 (cyclic shift), but boxes
	// contain duplicates, so the board is filled yet not legal.
	var sb []byte
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			sb = append(sb, byte('1'+((r+c)%9)))
		}
	}
	b := mustParse(t, string(sb))
	if IsLegal(b) {
		t.Error("cyclic board should break boxes and be illegal")
	}
	if IsSolved(b) {
		t.Error("cyclic board should not be a valid solution")
	}
}

func TestLegalPlacement(t *testing.T) {
	b := mustParse(t, easyPuzzle)
	// (0,2) empty in easy puzzle. Row0 has 5,3,7; col2 has 8; box has 5,3,6,9,8.
	if LegalPlacement(b, 0, 2, 5) {
		t.Error("5 should conflict with row")
	}
	if !LegalPlacement(b, 0, 2, 4) {
		t.Error("4 should be a legal placement at (0,2)")
	}
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
