package sudoku

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Game is an interactive play session.
type Game struct {
	Board      *Board
	Solution   *Board
	Difficulty Difficulty
	Seed       int64
	in         *bufio.Scanner
	out        io.Writer
}

// NewGame returns a Game that reads commands from in and writes to out.
func NewGame(b, solution *Board, in io.Reader, out io.Writer) *Game {
	return &Game{
		Board:    b,
		Solution: solution,
		in:       bufio.NewScanner(in),
		out:      out,
	}
}

func (g *Game) printf(format string, a ...any) { fmt.Fprintf(g.out, format, a...) }

// Run reads and runs commands until EOF or "quit".
func (g *Game) Run() error {
	g.printGrid()
	g.printf("Type 'help' for commands.\n")
	for {
		g.printf("> ")
		if !g.in.Scan() {
			break
		}
		line := strings.TrimSpace(g.in.Text())
		if line == "" {
			continue
		}
		if quit := g.handle(line); quit {
			break
		}
	}
	return g.in.Err()
}

// handle runs one command and returns true to quit.
func (g *Game) handle(line string) bool {
	fields := strings.Fields(line)
	cmd := strings.ToLower(fields[0])

	switch cmd {
	case "help", "h", "?":
		g.printHelp()
	case "quit", "q", "exit":
		g.printf("Goodbye.\n")
		return true
	case "print", "p", "show":
		g.printGrid()
	case "check", "c":
		g.check()
	case "solve":
		g.reveal()
	case "clear":
		if len(fields) != 3 {
			g.printf("usage: clear <row> <col>\n")
			return false
		}
		g.placeFromArgs(fields[1], fields[2], "0")
	case "new", "n":
		diff := g.Difficulty
		if len(fields) >= 2 {
			d, err := ParseDifficulty(fields[1])
			if err != nil {
				g.printf("%v\n", err)
				return false
			}
			diff = d
		}
		g.newPuzzle(diff)
	default:
		// "row col value"
		if len(fields) == 3 {
			g.placeFromArgs(fields[0], fields[1], fields[2])
			return false
		}
		g.printf("Unknown command %q. Type 'help'.\n", line)
	}
	return false
}

// placeFromArgs takes a 1-indexed row and column and a value.
func (g *Game) placeFromArgs(rs, cs, vs string) {
	r, err1 := strconv.Atoi(rs)
	c, err2 := strconv.Atoi(cs)
	v, err3 := strconv.Atoi(vs)
	if err1 != nil || err2 != nil || err3 != nil {
		g.printf("row, col and value must be numbers.\n")
		return
	}
	if r < 1 || r > 9 || c < 1 || c > 9 {
		g.printf("row and col must be 1..9.\n")
		return
	}
	if v < 0 || v > 9 {
		g.printf("value must be 0..9 (0 clears).\n")
		return
	}
	g.place(r-1, c-1, uint8(v))
}

// place sets (r, c), 0-indexed, refusing to change clues or make illegal moves.
func (g *Game) place(r, c int, v uint8) {
	if g.Board.IsGiven(r, c) {
		g.printf("Cell (%d,%d) is a given clue and cannot be changed.\n", r+1, c+1)
		return
	}
	if v == 0 {
		g.Board.set(r, c, 0)
		g.printf("Cleared (%d,%d).\n", r+1, c+1)
		g.printGrid()
		return
	}
	if !LegalPlacement(g.Board, r, c, v) {
		g.printf("%d cannot go at (%d,%d): conflicts with row, column, or box.\n", v, r+1, c+1)
		return
	}
	g.Board.set(r, c, v)
	g.printGrid()
	if g.Board.Count() == 81 {
		g.check()
	}
}

func (g *Game) check() {
	if !IsLegal(g.Board) {
		g.printf("Board has a conflict — not legal.\n")
		return
	}
	if g.Board.Count() < 81 {
		g.printf("Board is legal so far; %d cells remain.\n", 81-g.Board.Count())
		return
	}
	if IsSolved(g.Board) {
		g.printf("Solved! The board is complete and correct. You win!\n")
	} else {
		g.printf("Board is full but not a correct solution.\n")
	}
}

// reveal shows the solution, solving the board if there isn't a stored one.
func (g *Game) reveal() {
	sol := g.Solution
	if sol == nil {
		s, ok := Solve(g.Board)
		if !ok {
			g.printf("This board has no solution.\n")
			return
		}
		sol = s
	}
	g.Board = sol.Clone()
	g.Board.markGivens()
	g.printf("Solution revealed:\n")
	g.printGrid()
}

func (g *Game) newPuzzle(d Difficulty) {
	g.Seed++
	p, sol := Generate(GenOptions{Difficulty: d, Seed: g.Seed})
	g.Board, g.Solution, g.Difficulty = p, sol, d
	g.printf("New %s puzzle (%d clues):\n", d, p.Count())
	g.printGrid()
}

func (g *Game) printGrid() { g.printf("%s", g.Board.Grid()) }

func (g *Game) printHelp() {
	g.printf(`Commands:
  r c v        place value v (1-9) at row r, col c (1-9)
  r c 0        clear the cell at row r, col c
  clear r c    clear the cell at row r, col c
  check        report whether the board is legal / solved
  solve        reveal the solution
  new [diff]   generate a new puzzle (easy|medium|hard|expert|extreme)
  print        redraw the board
  help         show this help
  quit         exit
Given clues are locked and cannot be changed.
`)
}
