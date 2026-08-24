# Coding Challenge #133 — Particle Playground

An interactive 2D particle simulation combining a **falling-sand cellular automaton**
with a **particle engine**, per the
[challenge spec](https://codingchallenges.substack.com/p/coding-challenge-133-particle-playground).

Paint materials on a canvas and watch them behave: sand piles up, water finds its
level, oil floats, fire spreads and burns out, fireworks light flammable terrain.

## Run

No build step, no dependencies.

```
cd 133-particle-playground
python -m http.server 8133      # or any static server
# open http://localhost:8133
```

(Opening `index.html` directly from disk also works in most browsers.)

## Controls

- **Palette** — pick a material (Sand, Water, Oil, Stone, Wood, Fire), Erase,
  Sparks (plain physics particles), or Firework.
- **Paint** — click / drag on the canvas. Brush size via slider or mouse wheel.
- **Wind** — horizontal force applied to all particles.
- **Firework tool** — click to launch a rocket from that point; burst height comes
  from the slider, or drag upward and the drag distance overrides it. Pattern
  selector: Ring/Sphere, Willow, Crossette, Strobe. Embers ignite wood and oil
  on contact.
- **Pause / Resume** and **Clear**. Painting still works while paused.
- HUD shows FPS, live particle count, and active cell count.

## Architecture

Everything lives in `playground.js`:

- **Grid CA** — 240×160 cells (4 px each) stored in flat typed arrays
  (`mat`, `life`, `shade`, `stamp`). One bottom-up scan per step with alternating
  horizontal direction; a per-cell frame stamp prevents double-moves. Sand sinks
  through liquids and falls diagonally; liquids fall then disperse sideways
  (water displaces oil, which is why oil floats); fire ignites neighbors with
  per-material probabilities, boils adjacent water into steam, and vents smoke;
  gases drift upward and dissipate (steam can condense back to water).
- **Particle engine** — pooled array (cap 4000) of sparks, rockets, embers and
  trail particles under gravity, wind and velocity-proportional drag, with damped
  bouncing at edges and against solid grid cells. Rockets tunnel out of terrain
  they were launched inside, then detonate on any later solid contact. Embers
  ignite only flammable cells (wood, oil) and fizzle in water.
- **Performance** — fixed 60 Hz timestep decoupled from `requestAnimationFrame`
  (accumulator with backlog cap); 16×16 chunk activity tracking so settled
  regions sleep; the grid renders through a single `ImageData` blit scaled up
  with pixelated smoothing.

## Tests

Headless test harness (stubs the DOM, drives the sim directly):

```
npm test          # same as: node test/test_playground.js
```

`Math.random` is seeded in the harness, so runs are deterministic.

Covers: sand conservation + settling + pile shape, water leveling, oil-on-water
buoyancy, fire consuming wood / smoke / self-extinguish, steam generation, stone
impermeability, rocket burst + ember ignition, and the particle pool cap.
