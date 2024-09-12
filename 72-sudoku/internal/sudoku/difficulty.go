package sudoku

import (
	"fmt"
	"math/rand"
	"strings"
)

// Difficulty picks how many clues a generated puzzle has; fewer is generally
// harder. A Sudoku with a unique solution needs at least 17.
type Difficulty int

const (
	Easy Difficulty = iota
	Medium
	Hard
	Expert
	Extreme
)

// clueBands gives each difficulty's clue count range, inclusive.
var clueBands = map[Difficulty][2]int{
	Easy:    {40, 45},
	Medium:  {32, 39},
	Hard:    {27, 31},
	Expert:  {22, 26},
	Extreme: {17, 21},
}

// String returns the difficulty's name, as accepted by ParseDifficulty.
func (d Difficulty) String() string {
	switch d {
	case Easy:
		return "easy"
	case Medium:
		return "medium"
	case Hard:
		return "hard"
	case Expert:
		return "expert"
	case Extreme:
		return "extreme"
	default:
		return fmt.Sprintf("Difficulty(%d)", int(d))
	}
}

// ClueTarget picks a clue count in the difficulty's range.
func (d Difficulty) ClueTarget(rng *rand.Rand) int {
	band, ok := clueBands[d]
	if !ok {
		band = clueBands[Medium]
	}
	lo, hi := band[0], band[1]
	if hi <= lo {
		return lo
	}
	return lo + rng.Intn(hi-lo+1)
}

// ParseDifficulty parses a difficulty name, ignoring case.
func ParseDifficulty(s string) (Difficulty, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "easy":
		return Easy, nil
	case "medium":
		return Medium, nil
	case "hard":
		return Hard, nil
	case "expert":
		return Expert, nil
	case "extreme":
		return Extreme, nil
	default:
		return Medium, fmt.Errorf("sudoku: unknown difficulty %q", s)
	}
}
