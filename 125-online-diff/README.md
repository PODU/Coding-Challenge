# Online Diff — browser diff viewer (Coding Challenge #125)

A GitHub-style diff viewer that runs entirely in the browser: paste or drop two texts
and see line-level differences computed with a hand-written **Myers diff algorithm**
(the algorithm Git uses), with side-by-side and unified views, intra-line word/character
highlights, ignore options, collapsible unchanged regions, change navigation, and export
to a Git-compatible unified patch.

## Build & run

```sh
npm install
npm run dev       # dev server (prints a local URL)
npm test          # 62 headless tests (Vitest; core tests need no DOM)
npm run build     # strict typecheck + production bundle in dist/
npm run preview   # serve dist/
```

No runtime dependencies — the diff engine and UI are hand-written TypeScript; the only
packages are the toolchain (Vite, Vitest, TypeScript, jsdom).

## Features

- **Two inputs** — type/paste with 150 ms debounced recompute, or load files via the
  picker / drag-and-drop (name, size and line count shown per pane).
- **Side-by-side view** — original left, modified right, per-side line numbers, hatched
  gaps where a line exists on only one side, paired "modified" lines aligned.
- **Unified view** — Git `+`/`-`/space format; one-click toggle between views.
- **Intra-line precision** — word (default), character, or plain line mode; changed
  parts inside a modified line pair are `<mark>`-highlighted.
- **Ignore options** — whitespace (collapse+trim, covers indentation-only changes),
  case, blank lines (shown muted, not counted), line endings (CRLF ≡ LF, on by default).
- **Statistics bar** — lines added / removed / modified / unchanged / total changes.
- **Collapse unchanged** — runs longer than 10 lines fold behind a
  "⋯ N unchanged lines" bar (3 context lines kept at each edge); click to expand.
- **Navigation** — next/prev buttons, `J`/`K` keys, `Shift+Home`/`Shift+End` for
  first/last, a `n / m` position indicator and an outline on the current change.
- **Export** — copy a Git-compatible unified patch to the clipboard, or download it as
  a `.patch` file (correct `@@` headers, hunk merging, `\ No newline at end of file`).
- **Themes** — dark/light toggle, initial value from `prefers-color-scheme`.
- **Line wrap toggle** and sticky toolbar.

## How the diff is computed

1. Both texts are split into lines; each line gets a **comparison key** (the ignore
   options normalize only the key — the displayed text is always the original). A NUL
   sentinel on the last key makes a missing trailing newline diff-visible, like Git.
2. **Myers O(ND)** over the key arrays (after common prefix/suffix trimming) yields a
   minimal edit script; each maximal change run is coalesced into one delete block +
   one insert block. A depth cap (D ≤ 1000) degrades pathological inputs to a
   replace-all block instead of freezing the tab — a 10 000-line near-identical pair
   diffs in milliseconds.
3. Delete/insert blocks are **paired line-by-line** into "modified" rows (the GitHub
   convention); the same Myers engine then runs word- or character-wise inside each
   pair to produce the intra-line highlight ranges.
4. The unified-patch writer works from the raw segments, so the patch is byte-exact
   Git format regardless of view options.

Everything above lives in `src/core/` — pure, DOM-free, deterministic, and unit-tested
under plain Node. `src/ui/` is a thin shell that projects the row model into the DOM
(all text via `textContent`; pasted content can't inject markup).

## Tests

```sh
npm test
```

62 tests across 6 files: Myers minimality is verified against an LCS
dynamic-programming reference (fixed cases + a seeded pseudo-random stress), patch
output is compared byte-for-byte against expected Git-format strings, and a jsdom smoke
suite exercises the shell (view toggle, collapse/expand, navigation, ignore-option
round-trip).
