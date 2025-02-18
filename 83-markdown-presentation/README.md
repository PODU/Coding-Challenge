# mdpresent — Markdown Presentation Tool

Coding Challenge #83. Turn a Markdown file into a styled slide deck you can view in the
browser with **live reload**, customize with **themes/templates**, and **export to PDF**.

Built in TypeScript/Node with a hard line between a **pure core** (parse → split → render —
no I/O, fully unit-tested) and an **impure shell** (HTTP server, file watching, headless
browser).

## Install & build

```sh
npm install
npm run build      # tsc -> ./lib, then copies built-in templates
npm test           # vitest: core unit + golden + integration suites
```

Run the CLI without building via `npm run dev -- <command>` (uses `tsx`), or after building
via `node lib/cli.js <command>` (exposed as the `mdpresent` bin).

## Commands

```
mdpresent init <dir>           Scaffold a project: slides/, templates/, images/, README,
                               mdpresent.config.json, .gitignore, and `git init`.
mdpresent serve [file|dir]     Dev server + live reload (default ./slides).
    --port <n>                 Port (default 3000)
    --template-dir <path>      Custom templates; missing files fall back to defaults
    --no-open                  Do not auto-open the browser
mdpresent build [file|dir]     Render a self-contained static deck.
    --out <dir>                Output dir (default ./dist)
    --template-dir <path>
mdpresent export [file|dir]    Render and print to PDF.
    --out <file.pdf>           Output PDF (default ./out.pdf)
    --template-dir <path>
```

Exit codes: `0` success, `1` user error, `2` internal error.

## Quick start

```sh
node lib/cli.js init my-deck
cd my-deck
node ../lib/cli.js serve        # open http://localhost:3000
```

Edit `slides/slides.md`; the browser reloads automatically.

## Slide model

- Slides are separated by a full line of `---` (three or more hyphens).
- An optional **YAML front-matter** block (`---` … `---`) at the very top sets deck options
  (`title`, `theme`, `templateDir`). It is consumed before splitting, so it never produces a
  slide.
- `---` **inside a fenced code block** (```` ``` ```` or `~~~`) is literal — it does not split.
- Empty leading/trailing slides are dropped; a whitespace-only file still yields one slide,
  while a truly empty file yields zero (a friendly placeholder is shown).
- Per-slide `<!-- layout: name -->` and `<!-- notes: ... -->` directives are parsed.

> **Setext heading trade-off:** a line of `---` always splits, even directly under text
> (which CommonMark would read as an H2). Put a blank line before separators — matching
> reveal.js / Marp.

## Templates & themes (Step 4)

A template directory contains `layout.html`, `theme.css`, and `client.js`. Point at your own
with `--template-dir`; **any file you omit falls back to the built-in default**, so you can
override just the CSS and keep everything else. The layout is a string with `{{TITLE}}`,
`{{STYLE}}`, `{{HEAD_EXTRA}}`, `{{SLIDES}}`, `{{COUNT}}`, and `{{SCRIPT}}` placeholders.
Built-in templates live in `src/templates/default`.

## PDF export (gated)

`mdpresent export` first produces the same self-contained HTML as `build`, then prints it to
PDF with **Puppeteer** (headless Chromium, one slide per page via CSS `@page` / `break-after`).

Puppeteer is an **optional dependency** and Chromium may be unavailable (CI, sandboxes). In
that case `export` does **not** crash: it still writes the HTML deck and exits `1` with a
clear message telling you to open the HTML and print to PDF manually. To enable real PDF
output:

```sh
npm install puppeteer        # downloads a Chromium build
node lib/cli.js export slides --out deck.pdf
```

Because of this gating, the automated test suite **always** tests the HTML-export path
(`buildStatic`), and the Puppeteer PDF test is gated behind `MDP_E2E=1`:

```sh
npm run test:e2e             # runs the Chromium PDF smoke test (if available)
```

## Architecture

```
PURE CORE  src/core      frontmatter · splitter (fence-aware) · slide-model · renderer
           src/client    navigation (clamp logic, headless-testable)
IMPURE     src/shell      template-loader · pipeline · server · live-reload · build ·
                          pdf-export · scaffold
           src/cli.ts     init | serve | build | export
```

## Tests

| Suite | What it covers |
|---|---|
| `test/splitter.spec.ts` | `""`→`[]`, n separators → n+1, fence-aware (``` / `~~~`), CRLF, front-matter offsets, `startLine` |
| `test/slide-model.spec.ts` | empty-slide drop policy, ids/indexes, layout directive, full pipeline |
| `test/renderer.spec.ts` | per-element goldens, image rewriting, deck injection + snapshot, empty placeholder |
| `test/navigation.spec.ts` | `clampIndex`/`next`/`prev` bounds + property test (no wrap) |
| `test/template-loader.spec.ts` | custom dir with per-file built-in fallback (Step 4) |
| `test/e2e/serve.spec.ts` | HTTP `GET /`, `/raw`, `/assets/*`, 404, and the live-reload WS `reload` push |
| `test/e2e/init.spec.ts` | scaffold layout + idempotency |
| `test/e2e/export.spec.ts` | static HTML export (always) + gated Chromium PDF smoke |
