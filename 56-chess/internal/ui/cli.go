package ui

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"strings"

	"chess/internal/engine"
)

// Mode says which sides the computer plays.
type Mode int

const (
	HumanVsHuman Mode = iota
	HumanVsAI         // the computer plays Black
	AIVsAI
)

// CLI is the interactive text game.
type CLI struct {
	board   *engine.Board
	mode    Mode
	depth   int // 0 means the computer plays random moves
	unicode bool
	rng     *rand.Rand
	in      *bufio.Reader
	out     io.Writer
}

// Config holds the options for NewCLI.
type Config struct {
	Mode    Mode
	Depth   int
	Unicode bool
	Seed    int64
}

// NewCLI returns a CLI that reads commands from in and writes to out.
func NewCLI(in io.Reader, out io.Writer, cfg Config) *CLI {
	return &CLI{
		board:   engine.NewBoard(),
		mode:    cfg.Mode,
		depth:   cfg.Depth,
		unicode: cfg.Unicode,
		rng:     rand.New(rand.NewSource(cfg.Seed)),
		in:      bufio.NewReader(in),
		out:     out,
	}
}

func (c *CLI) aiControls() bool {
	switch c.mode {
	case AIVsAI:
		return true
	case HumanVsAI:
		return c.board.SideToMove() == engine.Black
	default:
		return false
	}
}

func (c *CLI) printf(format string, a ...any) { fmt.Fprintf(c.out, format, a...) }

func (c *CLI) showBoard() {
	c.printf("\n%s", Render(c.board, c.unicode))
	st := c.board.Status()
	c.printf("%s to move.", title(c.board.SideToMove().String()))
	if c.board.InCheck(c.board.SideToMove()) && st == engine.Ongoing {
		c.printf(" Check!")
	}
	c.printf("\n")
	if st.IsOver() {
		c.announce(st)
	}
}

func (c *CLI) announce(st engine.Status) {
	switch st {
	case engine.Checkmate:
		winner := c.board.SideToMove().Opposite()
		c.printf("Checkmate. %s wins.\n", title(winner.String()))
	default:
		c.printf("Game over: %s.\n", st)
	}
}

func (c *CLI) aiMove() {
	var m engine.Move
	if c.depth <= 0 {
		m = engine.RandomMove(c.board, c.rng)
	} else {
		m, _ = engine.BestMove(c.board, c.depth)
	}
	if m.IsNull() {
		return
	}
	c.printf("Computer plays %s\n", m)
	c.board.MakeMove(m)
}

// Run plays until the game ends, the user quits or input runs out.
func (c *CLI) Run() {
	c.printHelp()
	for {
		c.showBoard()
		if c.board.Status().IsOver() {
			return
		}
		if c.aiControls() {
			c.aiMove()
			continue
		}
		c.printf("%s> ", c.board.SideToMove().String())
		line, err := c.in.ReadString('\n')
		if err != nil && line == "" {
			return
		}
		if c.handle(strings.TrimSpace(line)) {
			return
		}
	}
}

// handle runs one command and reports whether to quit.
func (c *CLI) handle(line string) bool {
	if line == "" {
		return false
	}
	fields := strings.Fields(line)
	cmd := fields[0]
	switch cmd {
	case "quit", "exit":
		return true
	case "help":
		c.printHelp()
	case "new":
		c.board = engine.NewBoard()
		c.printf("New game.\n")
	case "fen":
		if len(fields) < 2 {
			c.printf("usage: fen <fen-string>\n")
			break
		}
		nb, err := engine.ParseFEN(strings.Join(fields[1:], " "))
		if err != nil {
			c.printf("Invalid FEN: %v\n", err)
			break
		}
		c.board = nb
		c.printf("Position loaded. FEN: %s\n", c.board.FEN())
	case "undo":
		if !c.tryUndo() {
			c.printf("Nothing to undo.\n")
		}
	case "moves":
		c.listMoves()
	case "ai":
		c.aiMove()
	case "show":
		c.printf("FEN: %s\n", c.board.FEN())
	default:
		c.tryMove(cmd)
	}
	return false
}

// ProcessLine runs one command and reports whether to quit. Tests use it to
// drive the CLI without an input stream.
func (c *CLI) ProcessLine(line string) bool { return c.handle(strings.TrimSpace(line)) }

// Board returns the current board.
func (c *CLI) Board() *engine.Board { return c.board }

func (c *CLI) tryUndo() bool {
	if !c.board.HasHistory() {
		return false
	}
	c.board.UnmakeMove()
	c.printf("Undone.\n")
	return true
}

func (c *CLI) tryMove(s string) {
	m, ok := engine.ParseMove(c.board, s)
	if !ok {
		c.printf("Illegal or malformed move %q. Type 'moves' to list legal moves.\n", s)
		return
	}
	c.board.MakeMove(m)
}

func (c *CLI) listMoves() {
	moves := c.board.LegalMoves()
	strs := make([]string, len(moves))
	for i, m := range moves {
		strs[i] = m.String()
	}
	c.printf("%d legal moves: %s\n", len(strs), strings.Join(strs, " "))
}

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func (c *CLI) printHelp() {
	c.printf(`Commands:
  e2e4        make a move in coordinate notation (e7e8q to promote)
  moves       list all legal moves
  ai          let the computer make a move for the side to move
  fen <fen>   load a position from FEN
  show        print the current position's FEN
  new         start a new game
  undo        take back the last move
  help        show this help
  quit        exit
`)
}
