# Coding Challenge #108 — Online Coding Platform

A browser code editor with a **sandboxed, server-side execution backend** for running
untrusted Python 3.12 and JavaScript (Node) code, plus a mini-LeetCode **problem mode**
with hidden test cases. Built in TypeScript / Node 20.

---

## Quick start

```bash
npm install
npm run build      # tsc strict build -> dist/
npm test           # vitest + supertest (spawns real interpreters; safe programs only)
npm start          # serves API + editor on http://localhost:3000
```

Requirements: `node` (>=20), `npm`, and `python` (3.12) on `PATH`.
Override the Python binary with `PYTHON_BIN=python3` if needed.

Open `http://localhost:3000` to use the editor: write code, **Run**, see output;
save/load/delete snippets (localStorage); pick a problem and **Submit** against its test cases.

---

## What's implemented

| Area | Detail |
|------|--------|
| Editor UI (`web/`) | CodeMirror 5 (CDN) with syntax highlighting + textarea fallback (R5/R11), Run/Submit, output panel with error line/traceback (R3/R4), loading spinner (R9), snippet sidebar |
| Snippets | `localStorage` CRUD by name, survives reloads, graceful quota/disabled handling (R6/R7/R11) |
| Run API | `POST /api/run` — free-form execution with structured `RunResult` (R2/R3/R4/R8) |
| Problem mode | `GET /api/problems`, `GET /api/problems/:id` (hidden cases stripped), `POST /api/problems/:id/submit` (R12/R13) |
| Languages | Python + JavaScript via a pluggable **runner registry** (R14); `GET /api/languages` |
| Sandbox | In-process Layer 1: wall-clock deadline + `tree-kill`, output ceiling, scrubbed env, detached process group; POSIX Layer 2 `ulimit` wrapper |
| Load control | Bounded worker pool + per-IP rate limit + payload size caps (413/429) |

## API

| Method & path | Purpose |
|---------------|---------|
| `GET /api/health` | Liveness |
| `GET /api/languages` | Installed languages + versions |
| `POST /api/run` | `{ language, source, stdin? }` → `RunResult` |
| `GET /api/problems` | List problems (`id`, `title`, `languages`) |
| `GET /api/problems/:id` | Public problem (hidden cases + expected output stripped) |
| `POST /api/problems/:id/submit` | `{ language, source }` → per-case pass/fail `SubmitResult` |

`RunResult.status` ∈ `ok | error | timeout | output_limit | memory_limit | internal_error`.
On error, `error: { type, message, line, traceback }` (R4). Schemas live in `src/types.ts`.

## Configuration (env vars)

| Var | Default | Meaning |
|-----|---------|---------|
| `PORT` | `3000` | HTTP port |
| `RUN_TIMEOUT_MS` | `5000` | Default per-run wall-clock deadline |
| `RUN_MAX_TIMEOUT_MS` | `15000` | Ceiling on any requested timeout |
| `RUN_OUTPUT_LIMIT_BYTES` | `262144` | stdout+stderr capture ceiling |
| `RUN_MAX_SOURCE_BYTES` | `65536` | Max source size (→ 413) |
| `RUN_MAX_CONCURRENCY` | `4` | Max simultaneous untrusted processes |
| `RUN_MAX_QUEUE` | `32` | Max queued runs before 429 |
| `RATE_LIMIT_MAX` / `RATE_LIMIT_WINDOW_MS` | `60` / `60000` | Per-IP rate limit |
| `PYTHON_BIN` | `python` (win) / `python3` | Python interpreter |
| `CORS_ORIGIN` | unset (same-origin) | Allowed editor origin |

---

## SECURITY — read this before deploying

**Executing untrusted user code is the entire risk surface of this product.** Every
submission is treated as actively hostile.

### What this codebase enforces (always on — Layer 1, in-process)

- **Hard wall-clock timeout** — `setTimeout` deadline; on fire the whole **process group**
  is `tree-kill`ed with `SIGKILL` (`taskkill /T /F` on Windows). Infinite loops die (R8).
- **Output ceiling** — stdout+stderr captured into a bounded buffer; on overflow the process
  is killed and the result flagged `output_limit` / `truncated`. Stops print-loop blowups.
- **Detached process group** — child runs in its own group so one kill reaps grandchildren
  (mitigates fork-bomb orphans).
- **Scrubbed environment** — host env is dropped; Python runs `-I -B -E` (isolated mode:
  ignores `PYTHON*`, user site-packages, and cwd on `sys.path`); Node runs with empty
  `NODE_OPTIONS`/`NODE_PATH`. Code can't read host secrets via env or auto-import dropped modules.
- **Fresh per-run temp workdir** — created with `mkdtemp`, removed after the run.
- **Bounded concurrency + rate limit + payload caps** — one user can't exhaust the host;
  excess load is shed with `429` / `413`.

### POSIX hardening (Layer 2 — automatic on Linux/macOS)

On non-Windows hosts each run is wrapped in `/bin/sh -c` with a `ulimit` preamble:
CPU seconds (`-t`), virtual memory (`-v`), max processes (`-u`), max file size (`-f`).
These are a busy-loop/fork-bomb/OOM backstop. `ulimit` does not exist on Windows, so this
dev host relies on Layer 1 only — which is why **production should not run on Windows bare metal**.

### ⚠️ This is NOT sufficient for production by itself

**Layers 1–2 share the host kernel.** They reduce but do not eliminate risk from a determined
attacker (kernel exploits, side channels, resource starvation). **A production deployment MUST
run each execution inside Layer 3 isolation:**

- a throwaway **container**:
  `docker run --rm --network none --read-only --memory 128m --cpus 0.5 --pids-limit 64 --cap-drop ALL --security-opt no-new-privileges -u nobody …`
- or stronger: **gVisor (`runsc`)** / **Firecracker microVMs** (what real online judges use),
- on an **isolated, network-egress-blocked, ephemeral worker host** with auto-recycling,
- **never** with host privileges and **never** co-located with secrets or other state.

The `Sandbox` interface (`src/sandbox/sandbox.ts`) is designed so a `DockerSandbox` can replace
`ProcessSandbox` with no change to the rest of the app.

**Never `eval` untrusted code in-process.** This backend always shells out to a separate,
killable, resource-capped subprocess.

---

## Project layout

```
108-online-coding/
├── src/
│   ├── index.ts         # server bootstrap
│   ├── config.ts        # env-driven limits
│   ├── types.ts         # shared contracts (RunResult, Problem, …)
│   ├── api/             # express app, zod schemas, rate limiter
│   ├── exec/            # worker pool + executor (temp dir, cleanup)
│   ├── runners/         # PythonRunner, NodeRunner, registry
│   ├── sandbox/         # ProcessSandbox (Layer 1) + ulimit wrapper (Layer 2)
│   ├── result/          # reporter, traceback parser, comparator
│   └── problems/        # store loader + grader
├── problems/            # *.json problem definitions (with hidden test cases)
├── web/                 # editor SPA (index.html, app.js, styles.css)
└── test/                # vitest unit + integration + limits + submit + api(supertest)
```

## Tests

`npm test` runs:

- **unit** — runner registry, reporter status mapping, traceback parsing, comparator, validation.
- **integration** — real subprocesses: Python/Node hello world, multiline, math, stdin echo,
  syntax error (line parsed), runtime `NameError`/`ZeroDivisionError`, JS error.
- **limits** — infinite loop → `timeout` (short deadline, never hangs the suite); output flood →
  capped + `truncated`; deep recursion → contained error; concurrency cap → `QueueFullError`.
- **submit** — correct solution passes all cases; wrong solution gets the right count; hidden-case
  redaction; one case times out while another passes.
- **api** — Supertest over every endpoint incl. 400/413/404 and hidden-case stripping.

## License

MIT.
