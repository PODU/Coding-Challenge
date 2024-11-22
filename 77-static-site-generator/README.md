# ccsg — a small static site generator

Coding Challenge [#77](https://codingchallenges.substack.com/p/coding-challenge-77-build-your-own).

`ccsg` scaffolds a site, lets you author content in Markdown, and builds a folder
of static HTML by merging that content into theme templates. It also ships a dev
server with live reload. It is written in Go with **zero third-party
dependencies** and produces a single static binary.

## Install / build

```sh
cd 77-static-site-generator
go build -o ccsg .      # produces ./ccsg (ccsg.exe on Windows)
```

Or run directly with `go run . <command>`.

## Quick start

```sh
ccsg new site mysite      # scaffold a site with a placeholder homepage
cd mysite
ccsg new theme cc         # create themes/cc/index.html
ccsg new page about       # create content/about.md
ccsg build                # render content/**.md -> public/**.html
ccsg server               # serve public/ at http://localhost:8080 with live reload
```

## Commands

| Command | Action |
|---|---|
| `ccsg new site <name>` | Create `<name>/` with `content/index.md`, `themes/default/index.html`, and a `config` selecting the active theme. |
| `ccsg new theme <name>` | Create `themes/<name>/index.html` (run inside a site). |
| `ccsg new page <name>` | Create `content/<name>.md` — `<name>` may include sub-paths (`blog/first`). |
| `ccsg build` | Render Markdown to `public/`, recreating it from scratch each run. |
| `ccsg server [--port N]` | Build, serve `public/` (default port `8080`), watch `content/`+`themes/`, live-reload browsers on change. |
| `ccsg help` | Usage. |

`ccsg <name>` is a compatibility alias for `ccsg new site <name>`.

## How it works

- **Content** lives in `content/` as Markdown. **Themes** live in `themes/<name>/index.html`.
- A theme is an HTML file containing exactly two template tags:
  - `{{ Title }}` — the first top-level Markdown heading (`# ...`); falls back to the
    title-cased file name when absent.
  - `{{ Content }}` — the page's Markdown body rendered to HTML.
  - Whitespace inside braces is tolerated (`{{Title}}`, `{{  Title  }}`). Unknown
    `{{ ... }}` tags are left untouched.
- **Build** walks `content/**.md`, renders each file, substitutes it into the active
  theme, and writes `public/<path>.html` (mirroring sub-directories). `public/` is
  removed and recreated each build so no stale files remain.
- **Markdown** is rendered by a small, self-contained subset renderer: ATX headings,
  paragraphs, ordered/unordered lists, fenced code blocks, blockquotes, horizontal
  rules, and inline `**bold**`, `*italic*`, `` `code` ``, and `[text](url)` links.
  Raw text outside code is HTML-escaped.
- **Live reload**: the dev server injects a tiny `EventSource` snippet before
  `</body>`; a watcher polls `content/` and `themes/`, rebuilds on change, and pushes
  a reload event over Server-Sent Events. Standard library only — no `fsnotify`.

## Project layout

```
77-static-site-generator/
├── main.go                 CLI arg parsing + command dispatch
└── internal/
    ├── scaffold/           new site / theme / page generators
    ├── markdown/           Markdown -> HTML renderer + title extraction
    ├── template/           {{ Title }} / {{ Content }} substitution
    ├── builder/            walk content, render, write public/
    └── server/             static file server + watcher + live reload
```

## Tests

All tests are hermetic (Go `testing` + `t.TempDir()`, no network, no clocks):

```sh
go test ./...
```

Coverage includes Markdown rendering and title extraction, template substitution,
scaffolding (including duplicate-target and outside-a-site errors), a golden build
test (content + theme → expected `public/` HTML, including nested pages and stale-file
cleanup), an end-to-end CLI flow, and the dev server (serves built files with the
injected snippet, SSE content-type, watcher-triggered rebuild, path-traversal guard).

## Exit codes

`0` success; `1` usage error (bad/missing args, existing target); `2` runtime error
(I/O, run outside a site, port in use). Errors print to stderr prefixed `ccsg: `.
