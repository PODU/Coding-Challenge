# Wheel of Names — Coding Challenge #71

A browser **Wheel of Names** (inspired by *Wheel of Fortune* / wheelofnames.com).
Enter names, spin a decelerating canvas wheel, and a fixed pointer picks the
winner. Built in **TypeScript** around a pure, deterministic, DOM-free core that
is fully unit-tested in Node — the canvas renderer and animation driver are thin
shells on top.

## Features

- **Enter names** — one per line (blank/whitespace lines stripped).
- **Shuffle & Sort** — Fisher–Yates shuffle (seeded RNG) and locale-aware,
  case-insensitive, numeric-aware sort.
- **Draw the wheel** — equal segments, each a distinct colour from both
  neighbours (incl. wrap-around), labels auto-fit (shrink + ellipsis).
- **Spin** — accelerate/decelerate ease-out animation, then announce the winner.
- **Advanced mode** — per-name colour and relative size (weight).
- **Fair & deterministic** — win probability equals a segment's angular share;
  with a fixed seed, selection and final angle are reproducible.
- **Geometry ↔ winner consistency** — the segment under the pointer is exactly
  the reported winner (half-open intervals, single normalisation — no off-by-one).
- **Remove winner** — optional; rebuilds the wheel from the remainder.
- **Persistence** — names/weights/colours saved to `localStorage`.

## Architecture

```
src/core/     pure, deterministic, DOM-free (unit-tested)
  rng.ts        RNG interface + mulberry32 seeded PRNG
  names.ts      parseNames, shuffle, sortNames, addName, removeAt, dedupe
  colors.ts     assignColors (no equal neighbours)
  geometry.ts   buildSegments, angleToIndex, normalize
  select.ts     pickWinner (weighted by arc)
  physics.ts    planSpin, resolveWinner, easeOutCubic
  persist.ts    encodeWheel / decodeWheel
  errors.ts     EmptyWheelError, WheelDecodeError, IndexRangeError
src/render/   canvas renderer + rAF animation driver (browser only)
  WheelRenderer.ts, SpinAnimator.ts
src/main.ts   DOM/UI wiring
test/         Vitest specs against the core (headless)
```

The core never imports the DOM. `pickWinner` chooses the index; `planSpin`
computes a `targetAngle` that lands the pointer on that exact index; and
`resolveWinner(segs, targetAngle)` re-derives the index purely from geometry —
asserted equal in tests, so the visual outcome cannot disagree with the logic.

### Angle convention

Radians, clockwise from the pointer at **3 o'clock (0 rad)**. Segment *i* spans
`[start, end)` (half-open); boundaries belong to the segment that *starts* there.
The pointer reads local angle `normalize(-wheelAngle)`.

## Prerequisites

Node 20+ and npm 10+.

## Install / Run / Test

```bash
npm install      # install dependencies
npm run dev      # Vite dev server (open the printed localhost URL)
npm run build    # tsc --noEmit type-check + Vite production bundle -> dist/
npm run preview  # serve the built bundle
npm test         # Vitest run (headless core tests)
npm run test:watch
npm run typecheck
```

## How to play

1. Type names into the textarea, one per line.
2. Optionally **Shuffle** or **Sort**, or open **Advanced…** to set per-name
   colour and weight.
3. Click **SPIN**. The wheel decelerates and the pointer lands on the winner,
   which is announced below.
4. Tick **Remove winner after spin** to draw without replacement.

Your list (and advanced colours/weights) persist across reloads via
`localStorage`.

## Tests

`npm test` runs the headless Vitest suite covering:

- **RNG/determinism** — `mulberry32` stable stream; same seed → same outputs.
- **Selection fairness (R6)** — equal segments within tolerance of `1/N`;
  weights `[1,2,3]` → ≈ `1/6, 2/6, 3/6`; fixed seed → identical winner sequence.
- **Geometry ↔ winner mapping (R9)** — start/mid/just-before-end for
  `n = 1,2,3,7,50`; boundary → starting segment; `resolveWinner(planSpin(...))`
  round-trip (equal **and** weighted); `±TAU·k`/negative-angle normalisation.
- **Names ops** — `parseNames` (CRLF/LF, trim, blanks), `shuffle` (permutation,
  pure, low identity rate, deterministic), `sortNames`, `removeAt` (range errors).
- **Colours (R3)** — length `n`, no equal neighbour, wrap differs.
- **Physics (R4)** — `easeOutCubic` endpoints/monotonic/decelerating;
  `planSpin` minimum turns, positive duration, landing strictly inside the arc.
- **Edge cases** — empty wheel throws `EmptyWheelError`; 1 name → index 0;
  duplicates independent; weights ≤ 0 / NaN clamped.
- **Persistence (R11)** — `decode(encode(s))` deep-equals `s`; malformed → typed error.

## Caveats

- Renderer pixels are not unit-tested (per the design); only the pure core is.
- Colours use evenly spaced HSL hues to guarantee the no-equal-neighbour rule;
  with very many names hues become visually close though always distinct strings.
- Labels are hidden below a minimum arc threshold (and for very large lists) to
  stay legible; the wheel remains spinnable.
