# Coding Challenge #36 — Pong

A playable clone of Atari's 1972 **Pong**, built in TypeScript. The human plays
the left paddle; a (beatable) AI plays the right. Ball bounces off walls and
paddles, scoring when it exits an edge; first to a configurable winning score
(default **11**) wins, then you can replay.

The architecture is split deliberately:

- **`src/core/` — a pure, deterministic game-logic core** (physics, collision,
  scoring, AI). No DOM, no canvas. All randomness flows through an injected
  `rng: () => number`, so every behaviour is reproducible and unit-tested
  headlessly under Node.
- **`src/render/`, `src/input/`, `src/app/` — a thin browser shell** that draws
  the core's `GameState` to a `<canvas>` and feeds it keyboard input.

## Requirements

- Node 20+ and npm (developed on Node v20.17, npm 10.8).

## Install

```bash
npm install
```

## Test (headless logic core)

```bash
npm test
```

Runs the deterministic unit suite with `node --test` (via `tsx`) — 28 tests
covering wall bounces, paddle collision angles, swept anti-tunnelling, AI
tracking, serving, scoring, win detection, restart, and determinism. No browser
or display server required; exits non-zero on failure.

## Build

```bash
npm run build
```

- `tsc` type-checks (strict) and emits ES modules to `dist/`.
- `esbuild` bundles the browser entry point to `dist/bundle.js`.

(`npm run typecheck` runs the strict type-check alone.)

## Play in the browser

After `npm run build`, open `index.html` in any modern browser. Because it loads
an ES module, use a static server rather than the `file://` protocol, e.g.:

```bash
npx serve .          # then open the printed URL
# or:  python -m http.server 8000   ->  http://localhost:8000
```

### Controls

| Action | Keys |
|--------|------|
| Move left paddle up   | `↑` or `W` |
| Move left paddle down | `↓` or `S` |
| Restart after game over | `Space` or `Enter` |

## Configuration

All tunables live in `src/core/config.ts` (`defaultConfig`): table size, paddle
size/speed, ball radius/speed/max-bounce-angle/max-speed, AI speed/dead-zone/
error, winning score, and serve delay. Use `makeConfig({ ... })` to override a
subset. Raising `ai.error` makes the AI sloppier (and non-deterministic unless
the rng is stubbed).

## Project layout

```
36-pong/
  index.html
  src/
    core/    types.ts config.ts physics.ts paddle.ts ai.ts serve.ts game.ts index.ts
    render/  canvasRenderer.ts
    input/   keyboard.ts
    app/     main.ts
  test/      physics.test.ts paddle.test.ts ai.test.ts serve.test.ts game.test.ts helpers.ts
  package.json  tsconfig.json
```

## Notes & caveats

- **Fixed timestep** (1/120 s accumulator in `app/main.ts`) keeps physics
  frame-rate independent and reproducible; the renderer draws the latest state.
- **Anti-tunnelling**: paddle collision uses a swept test against the contact
  plane, and the ball speed is clamped to `ball.maxSpeed`, so a fast ball can't
  pass straight through a paddle in one step.
- **AI determinism**: with the default `ai.error = 0` the AI consumes no
  randomness, so a fixed rng makes an entire match reproducible.
- The renderer/input layers are browser-only and validated by hand, not by the automated suite.
- Networked multiplayer (the challenge's optional stretch goal) is out of scope.
```
