# Coding Challenge #63 — Space Invaders

A faithful, production-quality clone of the 1978 arcade game *Space Invaders*,
built in **TypeScript (Node 20)** with an **HTML5 Canvas** renderer.

The codebase is split into a **pure, deterministic game core** (no DOM, no timers,
no `Math.random`, no `Date`) and thin **adapters** (canvas renderer, keyboard input,
fixed-timestep clock, optional audio). The same core drives both the headless test
suite and the live browser game, so there is no logic drift between them.

## Requirements covered

Player ship with 3 lives and clamped horizontal movement (R1, R2); 5×11 alien fleet
that marches as a block, bounces off the *outermost-alive* edge, descends and speeds
up (R3–R6); attrition speed-ramp (R6); alien bombs (R7); single player shot in flight
(R8); AABB collisions for shots/aliens/bombs/ship/bunkers (R9); row-tiered scoring
(R10); destructible bunkers eroded bottom-up by shots and top-down by bombs (R11);
lose on no lives **or** fleet reaching the player row (R12); win a wave → harder next
wave (R13); game-over + restart (R14). Optional mystery UFO (O1) and WebAudio SFX (O2)
are implemented behind config flags.

## Project layout

```
src/
  core/        # pure deterministic game logic (no I/O)
    types.ts config.ts rng.ts ship.ts fleet.ts
    projectiles.ts bunkers.ts ufo.ts collisions.ts game.ts
  adapters/    # impure edges
    canvasRenderer.ts keyboardInput.ts rafClock.ts audio.ts
  main.ts      # the only impure entry point; wires adapters to the core
index.html
test/          # headless node:test suites on the core only
```

## Prerequisites

- Node.js 20+ and npm on your PATH (developed against Node 20.17 / npm 10.8).

## Install

```sh
npm install
```

## Build (type-check + browser bundle)

```sh
npm run build
```

Runs `tsc --noEmit` in `strict` mode, then bundles `src/main.ts` to
`dist/bundle.js` with esbuild.

## Play

```sh
npm run dev
```

This bundles, watches, and serves the project root. Open the printed
`http://localhost:8000` (esbuild default) and load `index.html`.

To play without the dev server, run `npm run build` once and open `index.html`
directly in a browser (it loads `dist/bundle.js`).

### Controls

- **Move:** `←` / `→` or `A` / `D`
- **Fire:** `Space` or `↑` (one shot in flight at a time)
- **Start / Restart:** `Enter`

## Test

```sh
npm test
```

Runs the headless suite via `node --import tsx --test`. All tests exercise the
**core only** — no canvas, no timers — and are fully deterministic (seeded
Mulberry32 RNG + fixed `dt`). The suite covers:

- **fleet** — horizontal march, edge-bounce/descend/speed-up, outermost-alive
  edge detection, attrition speed ramp (capped).
- **ship** — movement, edge clamping, fire-cooldown gating.
- **projectiles** — single-shot rule, bomb advance/cull, RNG-seeded bomb-drop
  determinism.
- **collisions** — shot↔alien (scoring), shot↔bunker, bomb↔ship (life loss),
  bomb↔bunker, resolution ordering.
- **bunkers** — bottom-up vs top-down erosion, pass-through of fully eroded
  columns.
- **rules** — lose by descent, lose by no lives (idempotent terminal state),
  win/wave progression, `reset()`.
- **integration** — scripted mini-game to a win and a loss, plus a 2000-tick
  smoke run of the default game.

## Determinism

The core depends only on an injected `RNG` port (`Mulberry32` in both production
and tests) and an injected `dt`. Given the same `(config, seed, input, dt)`
sequence it produces identical state and event streams — making replays and the
test suite fully reproducible. `main.ts` uses a fixed-timestep accumulator
(`1/120 s`) with a catch-up cap so physics is frame-rate independent.
