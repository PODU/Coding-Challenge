# Coding Challenge #130 — Sort Visualiser

An interactive, dependency-free sorting algorithm visualiser. Single HTML file, plain JavaScript, canvas rendering.

Challenge: https://codingchallenges.substack.com/p/coding-challenge-130-sort-visualiser

## Run

Open `index.html` in any browser. No build step, no server needed.

## Features

- **8 algorithms**: bubble, insertion, selection, merge, quick (Lomuto), heap, shell (Ciura gaps), radix (LSD base 10)
- **Live stats**: comparisons, writes, array accesses, elapsed time, current config
- **Colour coding**: idle (blue), comparing (yellow), swap/write (red), pivot/key (purple), sorted (green)
- **Playback**: start / pause / resume / restart / single-step; speed presets (slow / medium / fast) plus a 1–1000 ops/sec custom slider
- **Display modes**: proportional bars or a numbers grid (pluggable renderer interface)
- **Configuration**: sample size (10–200), initial order (random / reversed / sorted / nearly sorted)
- **Correctness validation** on completion, followed by a green sweep animation
- **Keyboard**: `Space` start/pause, `s` step, `r` restart

## Architecture

The challenge's core design constraint is separation of algorithms from rendering:

- **`Tracked`** wraps the array. Algorithms mutate only through `get / set / swap / compare`, which record statistics and highlight marks uniformly.
- **Algorithms are generators.** Each `yield` marks one visible operation; the algorithm knows nothing about pacing, colours, or the DOM.
- **The driver** owns the animation loop. It converts the speed setting into an operations-per-frame budget, advances the generator, and hands the current state to a renderer.
- **Renderers are pluggable** objects with a single `draw(ctx, w, h, tracked, maxVal, done)` method. `bars` and `numbers` are provided; adding another is one object literal.

Adding a new algorithm is one generator function plus one entry in the `ALGORITHMS` map.

## Testing

```
npm test
```

`test/test_sorts.js` loads the DOM-free core out of `index.html` into a Node `vm` sandbox and runs all 8 algorithms to completion against empty, single-element, sorted, reversed, duplicate-heavy, all-equal and random inputs, checking sortedness and permutation equality. It also checks stats/marks bookkeeping and `buildArray` orders. No dependencies.
