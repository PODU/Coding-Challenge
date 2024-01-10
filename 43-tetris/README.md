# Coding Challenge #43 — Tetris

A production-ready Tetris clone with a **pure, deterministic game-state core**
(no DOM, no timers, no randomness) and a thin **browser shell** (Canvas renderer
+ keyboard input + animation loop). The entire rule set is unit-tested headlessly
with Vitest; the browser layer is a small adapter.

- 10 × 20 well, 7 tetrominoes in standard colors
- Move / rotate with **SRS wall kicks** (plus a simple-rotation variant)
- Soft drop, hard drop, gravity + lock, ghost piece
- Line clears (single / double / triple / **tetris**) with NES-style scoring `base × (level + 1)`
- Level + gravity-speed progression every 10 lines
- **7-bag** randomizer with next-piece preview
- Game over on spawn collision, with restart
- Fully deterministic from a seed (reproducible tests)

## Requirements

- Node.js ≥ 18 and npm on your PATH.

## Install

```bash
npm install
```

## Test (headless logic suite — the CI gate)

```bash
npm test
```

Runs the full Vitest suite in Node (no browser): tetromino tables, collision,
movement, rotation + wall kicks, line clears (incl. tetris), scoring, level
progression, 7-bag spawn, game over, lock cycle, determinism/invariants, and a
renderer smoke test.

## Build (type-check + browser bundle)

```bash
npm run build
```

This type-checks with `tsc --noEmit` (strict mode) and bundles the shell to
`dist/bundle.js` with esbuild.

## Play

Build first, then open `index.html` in a browser (it loads `./dist/bundle.js`),
or run the dev server which bundles and serves on port 8000:

```bash
npm run dev
# open http://localhost:8000
```

### Controls

| Key | Action |
|---|---|
| ← / → | Move left / right (with auto-repeat) |
| ↓ | Soft drop (+1 pt per cell) |
| ↑ or X | Rotate clockwise |
| Z | Rotate counter-clockwise |
| Space | Hard drop (+2 pts per cell) |
| P | Pause / resume |
| Enter | Start / restart |

## Architecture

```
src/
  core/                       pure, deterministic, no I/O
    types.ts                  shared types + constants (COLS, ROWS)
    tetrominoes.ts            shape tables, spawn data, colors
    rng.ts                    seedable mulberry32 PRNG
    board.ts                  collision, lock, full-row detect/clear
    rotation.ts               SRS rotation + wall-kick tables
    scoring.ts                line scoring, level, gravity speed
    engine.ts                 reduce(state, action) reducer + 7-bag
    index.ts                  public core API
  shell/                      impure, browser-only
    CanvasRenderer.ts         draws GameState -> <canvas>
    InputController.ts        keydown/keyup -> Action (DAS/ARR repeat)
    GameLoop.ts               requestAnimationFrame + dt -> TICK
    screens.ts                overlays + HUD
    main.ts                   wires it all together (only DOM-touching file)
test/                         headless Vitest suites
```

The core never imports from the shell. State transitions go through a single
pure reducer:

```ts
const game = createGame({ seed: 1234 });
const playing = reduce(game, { type: 'START' });
const moved = reduce(playing, { type: 'MOVE', dir: -1 });
```

Time is injected via `{ type: 'TICK', dtMs }` and randomness via the seeded RNG,
so the same seed + same action sequence always yields an identical state.

### Scoring

| Lines cleared | Base points |
|---|---|
| 1 (single) | 40 |
| 2 (double) | 100 |
| 3 (triple) | 300 |
| 4 (tetris) | 1200 |

`points = base × (level + 1)`. Soft drop awards +1/cell, hard drop +2/cell.
Level = `floor(totalLines / 10)`; gravity starts at 800 ms/cell and decreases per
level down to a 50 ms floor.

## Notes / caveats

- **Soft drop never locks.** A soft drop on a grounded piece is a clean no-op;
  locking is handled exclusively by gravity (`TICK`). This honors the spec's
  edge case ("soft drop at floor must not corrupt the accumulator or
  double-lock") and avoids any double-lock path.
- **`RESTART` continues the RNG stream** from the current state rather than
  re-seeding, so a restart is still deterministic and not a repeat of the same
  bag sequence.
- The next-piece preview shows one piece (the bag is computed lazily as needed).
- Rotation uses full SRS wall kicks by default; pass
  `createGame({ simpleRotation: true })` for the minimal "rotate only if clear"
  variant.
```
