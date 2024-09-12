# Sudoku — Coding Challenge #72

A self-contained Sudoku engine in Go: board model, legality/solution validator,
a backtracking solver with no-solution and multiple-solution detection, a
unique-solution puzzle generator with seedable difficulty levels, and an
interactive terminal game. Zero third-party runtime dependencies.

## Layout

```
72-sudoku/
├── go.mod                        module github.com/cc/sudoku
├── cmd/sudoku/main.go            CLI entrypoint (play/generate/solve/check)
└── internal/sudoku/
    ├── board.go                  Board type, parse/format, cell access
    ├── validate.go               legality + solved checks
    ├── solver.go                 backtracking solver (MRV) + solution counter
    ├── generator.go              full-grid build + unique-preserving clue removal
    ├── difficulty.go             difficulty → clue-count band
    └── play.go                   interactive terminal game loop
```

Each source file has a matching `*_test.go`.

## Build & test

```sh
go build ./...
go test ./...
go test ./internal/sudoku -bench=BenchmarkSolveHard
```

A canonical 17-clue puzzle solves in well under a millisecond on a typical
machine.

## CLI usage

```
sudoku play     [--difficulty hard] [--seed 42] [--symmetric]
sudoku generate [--difficulty hard] [--seed 42] [--symmetric] [--show-solution]
sudoku solve    [--file puzzle.txt | --board <81chars>]
sudoku check    [--file solved.txt | --board <81chars>]
```

Difficulties: `easy`, `medium`, `hard`, `expert`, `extreme`.

Exit codes: `0` success/valid; `1` invalid board or no solution; `2` usage/parse
error.

### Examples

```sh
# Generate a reproducible hard puzzle
go run ./cmd/sudoku generate --difficulty hard --seed 42 --show-solution

# Solve a puzzle given as an 81-char string ('.' or '0' = empty)
go run ./cmd/sudoku solve --board "53..7....6..195....98....6.8...6...34..8.3..17...2...6.6....28....419..5....8..79"

# Check whether a board is a complete, correct solution
go run ./cmd/sudoku check --board "534678912672195348198342567859761423426853791713924856961537284287419635345286179"

# Play interactively
go run ./cmd/sudoku play --difficulty medium --seed 7
```

### Play commands

| Command       | Effect                                                  |
|---------------|---------------------------------------------------------|
| `r c v`       | place value `v` (1–9) at row `r`, col `c` (1-indexed)   |
| `r c 0`       | clear the cell at (`r`,`c`)                             |
| `clear r c`   | clear the cell at (`r`,`c`)                             |
| `check`       | report whether the board is legal / solved             |
| `solve`       | reveal the solution                                    |
| `new [diff]`  | generate a fresh puzzle                                 |
| `print`       | redraw the board                                        |
| `help`        | show command help                                       |
| `quit`        | exit                                                    |

Given clues are locked: attempts to edit or clear them are rejected. When the
board is filled, `check` reports a win (legal + complete) or a failure.

## Board text format

- **Canonical (round-trip):** a single line of 81 characters, `1`–`9` for
  digits and `.` (or `0`) for empty.
- **Lenient parse:** newlines, spaces, and grid-drawing characters (`-`, `+`,
  `|`, `_`) are ignored, so ASCII-art grids load too.
- Parse errors (wrong cell count, illegal character) produce a descriptive
  error.

## Public API (`internal/sudoku`)

```go
func ParseBoard(s string) (*Board, error)
func (b *Board) String() string        // 81-char round-trip form
func (b *Board) Grid() string          // pretty display
func (b *Board) Clone() *Board
func (b *Board) Count() int

func IsLegal(b *Board) bool
func IsSolved(b *Board) bool
func LegalPlacement(b *Board, r, c int, v uint8) bool

func Solve(b *Board) (*Board, bool)
func CountSolutions(b *Board, limit int) int
func HasUniqueSolution(b *Board) bool

func Generate(opts GenOptions) (puzzle, solution *Board)
```

## How it works

- **Solver.** Backtracking with the MRV heuristic (always branch on the empty
  cell with the fewest legal candidates), backed by incremental row/column/box
  bitmasks for O(1) legality checks. `CountSolutions` keeps searching past the
  first hit up to a caller-supplied limit, which is the basis of the uniqueness
  guarantee (`HasUniqueSolution` == `CountSolutions(b, 2) == 1`).
- **Generator.** Builds a full solved grid by running the solver with seeded
  candidate shuffling, then removes clues one at a time (optionally in
  180°-symmetric pairs), keeping a removal only if the puzzle still has a unique
  solution and never dropping below 17 clues. Difficulty selects the target clue
  band. Generation is fully deterministic for a fixed seed.

## Caveats

- Clue count is a strong but imperfect proxy for human-perceived difficulty; it
  is the standard, deterministic, testable heuristic and is sufficient here.
- The carving loop targets a clue band but stops early if no further removal
  preserves uniqueness, so very low bands (expert/extreme) may occasionally
  yield a few more clues than the band's lower bound. The result is always a
  valid, uniquely-solvable puzzle.
- Determinism is guaranteed for a given seed with this Go `math/rand` source;
  across major Go releases the standard library RNG stream is stable, but the
  guarantee is "same binary, same seed, same output".
