# Coding Challenge #91 — Monkeytype

A browser typing-speed test in the style of [monkeytype.com](https://monkeytype.com).
Shows a block of random common English words, captures every keystroke, highlights each
character correct/incorrect, tracks a blinking cursor, runs a timed test, and reports
**WPM**, **raw WPM**, and **accuracy** on a results screen.

The interesting part is *not* the DOM — it is the **typing state machine** and the
**WPM/accuracy math**. Both live in a pure, DOM-free, clock-injected core (`src/core/`)
so they are provably correct under headless unit tests with no browser and no real clock.
The browser shell (`src/ui/`) is a thin adapter that owns the only real clock and the
`requestAnimationFrame` loop.

## Quick start

```bash
cd 91-monkeytype
npm install
npm run dev      # local dev server
npm test         # headless deterministic core + jsdom UI tests
npm run build    # type-check (tsc --noEmit) + static bundle in dist/
```

Open the dev server URL, then just start typing. **Tab** or **Esc** restarts; the duration
buttons (15 / 30 / 60 / 120 s) start a fresh test at that length.

## Metrics (monkeytype convention)

A "word" is standardised to **5 characters**, so `words = chars / 5`:

- **WPM** = `(correct chars / 5) / minutes`
- **Raw WPM** = `(total typed chars / 5) / minutes`
- **Accuracy** = `correct chars / total typed chars × 100` (current-state definition)

Divide-by-zero is guarded (0 WPM before any time passes); accuracy is 100 with zero
keystrokes. Extra characters typed past a word/target end count as errors (raising raw WPM
and lowering accuracy) but never as correct.

## Architecture

```
Browser shell (src/ui)         Pure core (src/core, no DOM, no globals)
  KeyboardController  --intents-->  TypingSession  (state machine)
  Renderer            <--snapshot-- Metrics        (wpm / raw / accuracy)
  App (RAF + clock)   --now-------->  WordProvider   (seedable PRNG)
                                     Clock          (injected time source)
```

- **`core/session.ts`** — `TypingSession`: `idle | running | finished`; per-index char
  status; `typeChar` / `backspace` / `tick` / `reset` / `append`, all taking an injected
  `nowMs`. Emits an immutable `SessionSnapshot`.
- **`core/metrics.ts`** — pure `wpm` / `rawWpm` / `accuracy` / `elapsedMinutes` over a snapshot.
- **`core/words.ts`** — `mulberry32` seedable PRNG + `WordProvider` over a bundled list of
  the ~200 most common English words (clean, family-friendly).
- **`core/clock.ts`** — `Clock` interface, `RealClock` (`performance.now()`), `FakeClock` (tests).
- **`ui/`** — `KeyboardController` (keydown → intents), `Renderer` (snapshot → DOM),
  `App` (wiring + timer/RAF loop, word refill, restart).

## Tests

Headless under Vitest (Node), time supplied by `FakeClock`, words by a fixed PRNG seed:

- `test/metrics.test.ts` — WPM/raw/accuracy math, zero-time and zero-keystroke guards, frozen-after-finish.
- `test/session.test.ts` — char correctness, idle→running, extras, mixed scripted sequence, reset/append, invariants.
- `test/backspace.test.ts` — counter reversal, no-op at start, extras, strict vs relaxed word boundary, correct-after-fix.
- `test/timer.test.ts` — timed end on the exact boundary, frozen counters/metrics after finish.
- `test/words.test.ts` — deterministic generation, word count, spacing, list membership.
- `test/ui.test.ts` (jsdom) — keyboard intent mapping and renderer span/cursor projection.

## Scope

Implements challenge Steps 1–3 and Step 5 (typing test, live feedback, results, random
words). Step 4 (auth) and Step 6 (leaderboard) need a backend and are out of scope.
