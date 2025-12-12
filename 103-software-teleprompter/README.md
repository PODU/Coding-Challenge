# Software Teleprompter

John Crickett's [Coding Challenge #103](https://codingchallenges.substack.com/p/coding-challenge-103-software-teleprompter).

A browser-based teleprompter: paste or import a script, format it (font face/size,
margins, line spacing), scroll it at an adjustable speed with play/pause via button
or **space bar**, and **mirror** the text for a beam-splitter rig.

The design separates a **pure, framework-free core** (scroll-position model,
playback state machine, settings, import URL transforms) from a thin **DOM
renderer + control layer**. The core is driven by an injectable clock, so the
scrolling behaviour is fully deterministic and unit-testable headlessly — no
browser, no `requestAnimationFrame`, no wall-clock timing.

## Features

- **Text input** — type or paste your script; it renders immediately.
- **Formatting** — font face, font size (slider), line spacing, optional Markdown
  for headings/emphasis.
- **Layout** — independent left/right margins (% of width) and line spacing.
- **Scrolling** — speed in **px/second** (frame-rate independent); start/stop with
  the **Play/Pause button or the space bar**; Reset to rewind.
- **Mirror mode** — horizontal flip (`scaleX(-1)`) that does not disturb scroll math.
- **Import** — from a plain URL, a **GitHub** markdown file (blob URL is rewritten to
  `raw.githubusercontent.com`), or a **Google Doc** (rewritten to `/export?format=txt`).

## Quick start

```bash
npm install
npm run dev      # Vite dev server — open the printed localhost URL
npm test         # Vitest headless suite
npm run build    # type-check (tsc --noEmit) + static bundle in dist/
```

`dist/` is a static site; deploy to any static host (Netlify / Vercel / GitHub
Pages) to satisfy the optional "deploy" step.

## Architecture

```
src/
  core/                 # PURE — no DOM, no I/O, fully unit-tested
    scrollEngine.ts     # position model: position += speed*dt; clamp; auto-pause at end
    playback.ts         # PlaybackState: Paused | Playing
    settings.ts         # Settings type + clampSettings (validation/clamping)
    script.ts           # ScriptModel: text + measured contentHeight
    loaders.ts          # pure transforms: plain text, GitHub/Google Doc URL rewrite, Markdown->HTML
    clock.ts            # Clock = () => ms; SystemClock + ManualClock (tests)
  ui/                   # IMPURE, thin
    renderer.ts         # reads core state -> writes transform/styles; measures heights
    controls.ts         # DOM events + space bar -> engine methods; import flow
    net.ts              # NetworkAdapter: the only real fetch (injectable/mockable)
  main.ts               # composition root: wires core + rAF + renderer + controls
test/                   # Vitest
  scrollEngine.test.ts  # advance-over-time, frame independence, play/pause, speed change,
                        #   clamp at end, reset, re-clamp on shrink, zero/negative speed
  playback.test.ts      # state-machine transitions
  settings.test.ts      # clampSettings ranges, NaN rejection, idempotence
  loaders.test.ts       # URL rewrites + Markdown rendering
  domSmoke.test.ts      # jsdom: renderer transforms, controls, space bar, mocked import
```

### The scroll model (heart of the system)

Scrolling is a single scalar `position` in CSS pixels, advanced by physics rather
than frame counting:

```ts
position += speed * dt;                 // dt = (now - lastTick) / 1000 seconds
position  = clamp(position, 0, maxPosition);   // maxPosition = contentHeight - viewportHeight
```

Because integration uses real elapsed time (px/s), a 60 Hz and a 144 Hz display
scroll at the same real speed, and a long stall is absorbed by one large `dt` and
clamped at the end. `tick(now)` is the only timing entry point, which is why tests
can feed a `ManualClock` and assert exact positions.

## Testing

`npm test` runs the full suite headlessly. Core engine tests are deterministic
(injected `ManualClock`, no real time, no rAF). The renderer/controls get a jsdom
smoke test, and the import path is exercised against a **mocked** `NetworkAdapter`
so no network access is required.

## Notes / caveats

- Import requires the target server to allow cross-origin requests (CORS).
  `raw.githubusercontent.com` and Google Doc `export?format=txt` typically work;
  arbitrary URLs may be blocked by the browser — this is a browser/CORS limitation,
  not a bug in the app.
- The Markdown renderer is intentionally minimal (ATX headings, bold/italic/code,
  paragraphs) and HTML-escapes input. It is not a full CommonMark implementation.
- Virtualized rendering for extreme (100k+ line) scripts is listed as future work;
  the engine itself is O(1) in script length.
