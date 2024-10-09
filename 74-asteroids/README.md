# Asteroids — Coding Challenge #74

A playable clone of Atari's 1979 *Asteroids*, built in TypeScript and rendered to
an HTML5 Canvas. The design ( The core has no
DOM, no `Math.random`, and no clock — it is advanced by a fixed `dt` and an
injected seeded RNG, so the whole simulation is reproducible and headlessly
unit-testable.

## Requirements

- Node 18+ and npm on PATH.

## Install

```sh
npm install
```

## Play

```sh
npm run dev      # Vite dev server — open the printed http://localhost URL
```

Or build a static bundle and preview it:

```sh
npm run build    # tsc --noEmit (typecheck) + vite build -> dist/
npm run preview  # serve the production build
```

### Controls

| Action | Keys |
|---|---|
| Rotate left / right | `←` / `→` or `A` / `D` |
| Thrust | `↑` or `W` |
| Fire | `Space` |
| Start / Restart | `Enter` |

Press **Enter** on the title screen to start. Shoot asteroids: **Large → Medium →
Small**, with smaller fragments worth more (20 / 50 / 100). Colliding with an
asteroid costs a life; clearing the field advances to a harder wave. Game over at
zero lives — press **Enter** to play again. The screen wraps on all edges.

## Test

```sh
npm test         # Vitest, headless (Node) — runs the core test suite once
npm run test:watch
npm run test:cov # with coverage over src/core/**
npm run typecheck
```

The core tests cover physics integration (thrust / rotation / inertia / clamp),
screen wrap and wrap-aware distance, bullet↔asteroid collision and split sizes,
ship death and lives, scoring, wave clear, and frame-for-frame determinism from a
seeded RNG (`test/`).

## Layout

```
src/
  core/        # pure, deterministic, 100%-tested
    vec2.ts        vector math
    rng.ts         seeded mulberry32 RNG + helpers
    constants.ts   all gameplay tuning
    entities.ts    types + factory functions
    physics.ts     integrate, wrap, ship controls
    collisions.ts  wrap-aware circle overlap
    split.ts       asteroid tier-down + children
    scoring.ts     points by tier
    wave.ts        spawn a wave, detect clear
    step.ts        master reducer: step(state, input, dt, rng)
  shell/       # impure, browser-only
    input.ts       keyboard -> InputState
    renderer.ts    GameState -> canvas
    main.ts        bootstrap + fixed-dt loop
  types.ts     # shared InputState / GameState / GameEvent
test/          # headless Vitest specs
```

## Core entry point

```ts
step(state: GameState, input: InputState, dt: number, rng: Rng)
  : { state: GameState; events: GameEvent[] }
```

Pure and deterministic: it never mutates the caller's state (works on a deep
clone) and never draws or plays sound — it emits `GameEvent`s the shell turns
into effects. Identical `(seed, input-sequence)` always reproduces identical
state, frame for frame.
