# Chess — Coding Challenge #56

A complete, rules-correct chess engine in Go with a text (CLI) interface for
human-vs-human, human-vs-computer, and computer-vs-computer play. The core is a
pure, headless, unit-testable rules engine; the UI is strictly separated from it.

This implements John Crickett's [Coding Challenge #56 — "Build your own
Chess"](https://codingchallenges.substack.com/p/coding-challenge-56-chess),
plus the engineering completeness (special moves, draws, FEN, perft) needed for
a production-grade engine.

## Features

- **Board & pieces** in a 0x88 representation with in-place make/unmake (no
  per-node copying), used by both search and perft.
- **Legal move generation** for every piece, including the special moves:
  castling (king- and queen-side, with full legality constraints), en passant,
  and promotion (Q/R/B/N).
- **Self-check legality** via a generic make/test/unmake filter — pins and
  moving into check are handled with no special-case code.
- **Game termination**: check, checkmate, stalemate, plus the fifty-move rule,
  threefold repetition (Zobrist hashing), and insufficient-material draws.
- **FEN** parse and emit, round-trippable, with strict validation.
- **AI**: negamax search with alpha-beta pruning and a material + piece-square
  evaluation, plus a random-legal-move mode (the challenge's initial step).
- **PGN/SAN** export (optional module), isolated from the engine.

## Layout

```
56-chess/
  go.mod                      module chess
  cmd/chess/main.go           CLI entry point; wires UI <-> engine
  internal/engine/            pure rules + search (no I/O)
    piece.go board.go move.go fen.go zobrist.go
    attacks.go movegen.go legal.go status.go
    perft.go eval.go search.go
    *_test.go                 perft, FEN, special moves, status, search
  internal/ui/                rendering + REPL (render.go, cli.go)
  internal/pgn/pgn.go         optional SAN/PGN export
```

The `engine` package has **no third-party dependencies** (standard library only)
and never imports the UI.

## Build & test

```sh
go build ./...
go test ./...            # full suite incl. deep perft (a couple of seconds)
go test ./... -short     # skips the deepest perft depths
go vet ./...
```

Perft node counts are verified against published reference values
(startpos to depth 5, Kiwipete, and positions 3–6), which is the primary
move-generation correctness gate. `-short` skips the slowest depths
(e.g. startpos perft(5) = 4,865,609).

## Running

```sh
go run ./cmd/chess              # human (White) vs AI (Black), Unicode board
go run ./cmd/chess -mode hvh    # human vs human
go run ./cmd/chess -mode ava    # AI vs AI demo
go run ./cmd/chess -depth 0     # AI plays random legal moves
go run ./cmd/chess -ascii       # ASCII pieces instead of Unicode glyphs
```

Flags: `-mode` (`hvh`, `hva`, `ava`), `-depth` (AI search depth, `0` = random),
`-ascii`, `-seed`.

### Commands at the prompt

| Command      | Action                                                  |
|--------------|---------------------------------------------------------|
| `e2e4`       | make a move in coordinate notation (`e7e8q` to promote) |
| `moves`      | list all legal moves                                    |
| `ai`         | let the computer move for the side to move              |
| `fen <fen>`  | load a position from FEN                                |
| `show`       | print the current position's FEN                        |
| `new`        | start a new game                                        |
| `undo`       | take back the last move                                 |
| `help`       | show help                                               |
| `quit`       | exit                                                    |

Illegal or malformed moves are rejected with a message and never mutate the
position.

## Design notes

- **0x88 board**: off-board detection is a single `sq & 0x88` test, which keeps
  sliding/stepping generation simple and avoids edge-wrap bugs.
- **Make/Unmake** with an undo stack (captured piece, castling rights, en
  passant square, halfmove clock, Zobrist hash) enables fast perft and search.
- **Legality** is uniform: generate pseudo-legal moves, then make/test
  `InCheck`/unmake. The same primitive (`IsSquareAttacked`) drives check
  detection, castling legality, and the legal-move filter.
- **Search** is negamax + alpha-beta over the engine's public move-gen and eval
  APIs; it contains no I/O and is unit-tested with mate-in-N puzzles and an
  alpha-beta == plain-minimax equivalence check.
