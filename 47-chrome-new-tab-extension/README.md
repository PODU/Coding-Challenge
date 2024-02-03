# Coding Challenge #47 — Chrome New Tab Extension

A Chrome **Manifest V3** extension that replaces the browser's New Tab page with
a Coding-Challenges-branded dashboard:

- Coding Challenges blue (`#04295B`) background with a centered `Coding Challenges>_` wordmark.
- A large, live clock (ticks every second, drift-corrected) and a human-readable date.
- A dynamic panel listing the **open pull requests** of the
  [`CodingChallengesFYI/SharedSolutions`](https://github.com/CodingChallengesFYI/SharedSolutions)
  repository via the public GitHub REST API.
- Graceful offline degradation: the clock and branding always render; the PR
  panel shows a clear loading / empty / error message instead of breaking.

Built with **TypeScript** (strict), bundled with **esbuild**, tested with **Vitest**.

## Project layout

```
47-chrome-new-tab-extension/
├── public/            # static shell copied verbatim into dist/
│   ├── manifest.json  # MV3, chrome_url_overrides.newtab -> newtab.html
│   ├── newtab.html / newtab.css
│   └── icons/         # 16/48/128 png icons
├── src/
│   ├── clock.ts       # pure time/date formatting + injectable tick scheduler
│   ├── github.ts      # PR fetch behind a FetchLike interface + pure mapPulls()
│   ├── rss.ts         # pure RSS XML -> items parser (optional Version 1 source)
│   ├── render.ts      # DOM rendering (PR list / loading / error / empty)
│   └── newtab.ts      # thin entry point wiring the above to DOM + timers
├── scripts/           # build helpers (copy assets, generate icons)
├── test/              # Vitest suites (clock, github, render, rss)
└── dist/              # build output — load this folder as an unpacked extension
```

The pure core (`clock`, `github.mapPulls`, `rss`, `render`) takes data in and
returns view models / DOM out, so it is unit-testable with no network and no
real timers. `src/newtab.ts` is the only module that touches `fetch` and the
live DOM.

## Prerequisites

- Node.js >= 18 and npm (developed against Node v20.17, npm 10.8).

## Build

```bash
npm install
npm run build
```

This type-checks (`tsc --noEmit`, strict), bundles `src/newtab.ts` →
`dist/newtab.js` with esbuild, and copies `public/` into `dist/`. The resulting
`dist/` folder is a complete, loadable unpacked extension.

> Icons are pre-generated and committed under `public/icons/`. To regenerate
> them run `node scripts/gen-icons.mjs`.

## Load in Chrome

1. Run `npm run build` to produce `dist/`.
2. Open `chrome://extensions`.
3. Enable **Developer mode** (top-right toggle).
4. Click **Load unpacked** and select the `dist/` folder.
5. Open a new tab — the Coding Challenges dashboard appears.

To remove it, toggle the extension off or remove it from `chrome://extensions`.

## Test

```bash
npm test
```

Runs the Vitest suite, fully offline and deterministic:

- **clock.test.ts** — `formatTime` (midnight, single-digit padding, 24h),
  `formatDate` (exact human-readable string), and `startClock` with a fake
  scheduler + incrementing fake `now()` (tick values, drift correction, `Stop`).
- **github.test.ts** — `mapPulls` maps a canned fixture and drops malformed
  entries; `fetchOpenPulls` with an injected fake `fetch` (200 → PRs, 403 →
  typed rate-limit error, 500 → status carried, network reject → propagates).
- **render.test.ts** — PR-list rendering from a **mocked fetch** into a real DOM
  (happy-dom), empty-state message, XSS-safe titles, and the **offline fallback**
  error rendering.
- **rss.test.ts** — `parseRss` extracts items from a canned feed, decodes
  entities, and returns `[]` for an empty feed.

## Design notes & caveats

- **No service worker / proxy needed.** The GitHub REST API sends permissive
  CORS headers, so the New Tab page can `fetch` it directly. `host_permissions`
  is scoped to `https://api.github.com/*`.
- **RSS (Version 1) is parser-only.** The Substack RSS feed lacks CORS headers,
  so it cannot be fetched directly from the page. `rss.ts` parses already-fetched
  XML (so it stays testable); wiring it to a proxy/background fetch is left as a
  documented stretch and is not part of the primary build.
- **Rate limiting.** Unauthenticated GitHub requests are limited (~60/hour per
  IP). On 403/429 the panel shows a rate-limit message rather than crashing.
- **Determinism.** Dates are formatted from explicit weekday/month tables
  (locale-independent) so tests assert exact strings.
