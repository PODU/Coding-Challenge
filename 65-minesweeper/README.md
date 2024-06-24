# Minesweeper — Coding Challenge #65

A playable Minesweeper built as a **pure, headlessly-testable board engine**
plus a thin **terminal renderer + input layer**, in TypeScript on Node.

## Features

- Grid of randomly-distributed hidden mines.
- Reveal a cell to see a mine or a number (count of mines in the 8 neighbours).
- Flood-fill: revealing a zero-adjacency cell opens the whole connected empty
  region and its numbered border.
- Flag / unflag suspected mines; mines-left counter and an elapsed-seconds timer.
- **First click is always safe** — mines are placed lazily after the first
  reveal, excluding that cell (and, when there is room, its neighbours).
- Difficulty levels: Beginner (9x9 / 10), Intermediate (16x16 / 40),
  Expert (16x30 / 99), and validated Custom.
- Lose on mine (all mines revealed); win when every safe cell is revealed.
- Restart / replay at any time.
- **Deterministic** mine placement via an injected, seeded RNG (mulberry32) — so
  the entire ruleset is unit-testable without a UI.

## Requirements

- Node.js 18+ and npm on your PATH.

## Install

```sh
npm install
```

## Build (type-checked, strict)

```sh
npm run build      # tsc -> dist/
```

## Test (headless core + UI tests)

```sh
npm test           # vitest run
```

Covered: deterministic seeded layout, adjacency counts (corner/edge/surrounded),
flood-fill region & termination, win/lose & reveal-all-mines, flag toggling and
no-ops, first-click safety, plus input-parser / renderer / timer smoke tests.

## Play (terminal)

No build needed — run the TypeScript directly with `tsx`:

```sh
npm run play
```

Or run the compiled build:

```sh
npm run build && npm start
```

### Commands

```
r <row> <col>      reveal a cell        (alias: reveal)
f <row> <col>      flag / unflag a cell (alias: flag)
new <difficulty>   beginner | intermediate | expert | custom <rows> <cols> <mines>
help               show help            (alias: h, ?)
quit               exit                 (alias: q, exit)
```

Rows and columns are **0-indexed**. Glyphs: `.` hidden, `F` flag, `*` mine,
`1`–`8` adjacent mine count, blank = zero. Example:

```
> r 4 4
> f 0 0
> new expert
```

## Project layout

```
src/
  core/        pure engine (no I/O): types, rng, difficulty, board
  ui/          renderer, input parser, timer (effect-free pieces)
  cli.ts       the single stateful loop (input -> core -> renderer)
  index.ts     public barrel for the core API
test/          vitest specs against the pure core + UI helpers
```

## Core API (a taste)

```ts
import { createBoard, reveal, toggleFlag, minesRemaining, isOver } from './src';
import { mulberry32 } from './src/core/rng';

let board = createBoard({ rows: 9, cols: 9, mines: 10 });
board = reveal(board, 4, 4, mulberry32(123)); // lazy, first-click-safe placement
board = toggleFlag(board, 0, 0);
```

Every core function is total, performs no I/O, and takes its randomness from an
injected `RNG`, so behaviour is fully reproducible for a given seed.
