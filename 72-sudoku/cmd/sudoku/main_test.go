package main

import "testing"

const (
	easyPuzzle = "53..7....6..195....98....6.8...6...34..8.3..17...2...6.6....28....419..5....8..79"
	solvedEasy = "534678912672195348198342567859761423426853791713924856961537284287419635345286179"
)

func TestRunSolveExitCode(t *testing.T) {
	if code := run([]string{"solve", "--board", easyPuzzle}); code != 0 {
		t.Errorf("solve exit = %d, want 0", code)
	}
	// No solution => exit 1.
	noSol := "12345678." + "........9"
	for len(noSol) < 81 {
		noSol += "."
	}
	if code := run([]string{"solve", "--board", noSol}); code != 1 {
		t.Errorf("solve no-solution exit = %d, want 1", code)
	}
}

func TestRunCheckExitCodes(t *testing.T) {
	if code := run([]string{"check", "--board", solvedEasy}); code != 0 {
		t.Errorf("check valid exit = %d, want 0", code)
	}
	bad := "55" + repeatDot(79)
	if code := run([]string{"check", "--board", bad}); code != 1 {
		t.Errorf("check invalid exit = %d, want 1", code)
	}
}

func TestRunUsageErrors(t *testing.T) {
	if code := run(nil); code != 2 {
		t.Errorf("no args exit = %d, want 2", code)
	}
	if code := run([]string{"solve", "--board", "tooshort"}); code != 2 {
		t.Errorf("bad board exit = %d, want 2", code)
	}
	if code := run([]string{"bogus"}); code != 2 {
		t.Errorf("unknown subcommand exit = %d, want 2", code)
	}
}

func repeatDot(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = '.'
	}
	return string(b)
}
