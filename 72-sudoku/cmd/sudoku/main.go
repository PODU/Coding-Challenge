// Command sudoku plays, generates, solves and checks Sudoku puzzles.
//
// Usage:
//
//	sudoku play     [--difficulty hard] [--seed 42] [--symmetric]
//	sudoku generate [--difficulty hard] [--seed 42] [--symmetric] [--show-solution]
//	sudoku solve    [--file puzzle.txt | --board <81chars>]
//	sudoku check    [--file solved.txt | --board <81chars>]
//
// Exit codes: 0 ok, 1 invalid board or no solution, 2 bad usage or input.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cc/sudoku/internal/sudoku"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	switch args[0] {
	case "play":
		return cmdPlay(args[1:])
	case "generate", "gen":
		return cmdGenerate(args[1:])
	case "solve":
		return cmdSolve(args[1:])
	case "check":
		return cmdCheck(args[1:])
	case "-h", "--help", "help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", args[0])
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `sudoku — build-your-own Sudoku engine

Subcommands:
  play     [--difficulty hard] [--seed N] [--symmetric]
  generate [--difficulty hard] [--seed N] [--symmetric] [--show-solution]
  solve    [--file puzzle.txt | --board <81chars>]
  check    [--file solved.txt | --board <81chars>]

Difficulties: easy, medium, hard, expert, extreme
`)
}

// loadBoard reads the --board or --file board. The int is an exit code.
func loadBoard(board, file string) (*sudoku.Board, int) {
	switch {
	case board != "":
		b, err := sudoku.ParseBoard(board)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return nil, 2
		}
		return b, 0
	case file != "":
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return nil, 2
		}
		b, err := sudoku.ParseBoard(string(data))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return nil, 2
		}
		return b, 0
	default:
		fmt.Fprintln(os.Stderr, "provide --board <81chars> or --file <path>")
		return nil, 2
	}
}

func cmdPlay(args []string) int {
	fs := flag.NewFlagSet("play", flag.ContinueOnError)
	diff := fs.String("difficulty", "medium", "easy|medium|hard|expert|extreme")
	seed := fs.Int64("seed", time.Now().UnixNano(), "RNG seed")
	sym := fs.Bool("symmetric", false, "180-degree symmetric clues")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	d, err := sudoku.ParseDifficulty(*diff)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	puzzle, solution := sudoku.Generate(sudoku.GenOptions{Difficulty: d, Seed: *seed, Symmetric: *sym})
	g := sudoku.NewGame(puzzle, solution, os.Stdin, os.Stdout)
	g.Difficulty = d
	g.Seed = *seed
	if err := g.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func cmdGenerate(args []string) int {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	diff := fs.String("difficulty", "medium", "easy|medium|hard|expert|extreme")
	seed := fs.Int64("seed", time.Now().UnixNano(), "RNG seed")
	sym := fs.Bool("symmetric", false, "180-degree symmetric clues")
	showSol := fs.Bool("show-solution", false, "also print the solution")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	d, err := sudoku.ParseDifficulty(*diff)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	puzzle, solution := sudoku.Generate(sudoku.GenOptions{Difficulty: d, Seed: *seed, Symmetric: *sym})
	fmt.Printf("# %s puzzle, %d clues, seed %d\n", d, puzzle.Count(), *seed)
	fmt.Println(puzzle.String())
	fmt.Print(puzzle.Grid())
	if *showSol {
		fmt.Println("# solution")
		fmt.Println(solution.String())
	}
	return 0
}

func cmdSolve(args []string) int {
	fs := flag.NewFlagSet("solve", flag.ContinueOnError)
	board := fs.String("board", "", "81-character board")
	file := fs.String("file", "", "path to board file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	b, code := loadBoard(*board, *file)
	if b == nil {
		return code
	}
	sol, ok := sudoku.Solve(b)
	if !ok {
		fmt.Fprintln(os.Stderr, "no solution")
		return 1
	}
	fmt.Println(sol.String())
	fmt.Print(sol.Grid())
	if !sudoku.HasUniqueSolution(b) {
		fmt.Fprintln(os.Stderr, "warning: puzzle has multiple solutions")
	}
	return 0
}

func cmdCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	board := fs.String("board", "", "81-character board")
	file := fs.String("file", "", "path to board file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	b, code := loadBoard(*board, *file)
	if b == nil {
		return code
	}
	switch {
	case sudoku.IsSolved(b):
		fmt.Println("valid: complete and correct solution")
		return 0
	case !sudoku.IsLegal(b):
		fmt.Println("invalid: board has a conflict")
		return 1
	default:
		fmt.Println("incomplete: legal so far but not finished")
		return 1
	}
}
