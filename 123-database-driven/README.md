# llmwiki — Database-Driven LLM Wiki

> Coding Challenge #123. An agent-maintained, interlinked Markdown knowledge
> base. Sources are *compiled once* into curated wiki pages (summaries, entity
> pages, topic roll-ups), interlinked, contradiction-checked, embedded, and
> answered over with **hybrid search** (vector + full-text) and cited synthesis.

Unlike classic RAG — which re-discovers knowledge from raw documents on every
query — this system writes knowledge down, keeps it current via an append-only
operation log, and synthesizes answers across the already-distilled pages.

## Why it runs offline by default

Every storage, embedding, and LLM access goes through an **interface seam**, and
ships a hermetic default so the whole test suite runs **offline, deterministic,
in-memory** — no Docker, no Oracle, no Ollama, no network, no API key.

| Concern    | Production seam                         | Hermetic default (tests)            |
|------------|-----------------------------------------|-------------------------------------|
| Store      | `OracleStore` (Oracle 26ai VECTOR+Text) | `SqliteStore` (`:memory:`/file, FTS5) |
| Embeddings | `OllamaEmbedder` (`nomic-embed-text`)   | `HashingEmbedder` (deterministic)   |
| LLM        | `AnthropicLLM` (`claude-opus-4-8`)      | `ScriptedLLM` (deterministic)       |

The wiki model, ingestion/query graphs, linter, and CLI are 100%
backend-agnostic; the base install needs only **stdlib + numpy**.

## Install

```bash
python -m pip install -e .          # base (numpy only)
python -m pip install -e '.[test]'  # + pytest
# optional production seams:
python -m pip install -e '.[anthropic]'  # Claude
python -m pip install -e '.[ollama]'     # local embeddings
python -m pip install -e '.[oracle]'     # Oracle 26ai
python -m pip install -e '.[web]'        # FastAPI chat
```

## Run the tests (offline)

```bash
python -m pytest
```

All tests use `SqliteStore(":memory:")` + `HashingEmbedder` + `ScriptedLLM`.

## CLI

```bash
wiki project create ml-history
wiki ingest  ml-history ./docs/            # file or directory
wiki ask     ml-history "Who invented backpropagation?" --save --json
wiki lint    ml-history --fix
wiki browse  ml-history --type entity
wiki page    ml-history entities/ada-lovelace
wiki log     ml-history --json
wiki chat    ml-history                    # interactive REPL
wiki serve   --port 8000                   # optional web chat ([web] extra)
```

Seams are environment-selected (defaults are hermetic):

```bash
WIKI_STORE=sqlite|oracle            # default sqlite
WIKI_EMBEDDER=hashing|ollama        # default hashing
WIKI_LLM=scripted|anthropic|ollama  # default scripted
WIKI_PROJECTS_ROOT=./projects       # on-disk wiki + db location (default: <project dir>/projects)
ANTHROPIC_API_KEY=...               # only read by the anthropic seam
```

## Programmatic API

```python
from llmwiki import Wiki, SqliteStore, HashingEmbedder, ScriptedLLM

wiki = Wiki(store=SqliteStore(":memory:"),
            embedder=HashingEmbedder(),
            llm=ScriptedLLM(),
            projects_root="projects")
wiki.create_project("p")
wiki.ingest("p", "Ada Lovelace wrote the first algorithm in 1843. ...")
ans = wiki.ask("p", "Who wrote the first algorithm?")
print(ans.answer, ans.citations)
findings = wiki.lint("p")
```

## On-disk layout

```
projects/<project>/
  wiki/{summaries,entities,topics,answers}/*.md   # Markdown + YAML frontmatter
  raw/<source_id>.txt                             # verbatim ingested sources
  index.md                                        # browse index (Step 3)
  log.jsonl                                        # append-only op log (Step 3)
  wiki.db                                          # SQLite store (file backend)
```

Each page carries YAML frontmatter (`id`, `type`, `title`, `sources`, `links`,
`tags`, `status`, `content_sha256`, timestamps) and a Markdown body that may
contain `[[wikilinks]]` and `^citation` markers.

## How it works

- **Ingestion graph (Step 2):** `load_source → extract_entities → summarize →
  upsert_pages → update_topics → detect_contradictions → embed_changed → log`.
  Re-ingesting an unchanged source is a no-op (sha256 match); only changed pages
  are re-embedded.
- **Query graph (Step 5):** `plan → hybrid_retrieve → synthesize →
  attach_citations → maybe_save_answer`. Answers are grounded **only** in
  retrieved pages; with no results an explicit empty-citation answer is returned.
- **Hybrid search (Step 4):** in-process cosine over per-page vectors + FTS5
  full-text, fused with **Reciprocal Rank Fusion** (backend-independent).
- **Linter (Step 7):** orphans, missing/broken links, stale claims, data gaps,
  and contradictions; `--fix` applies safe, idempotent repairs.
- **Projects (Step 6):** every project has isolated pages, embeddings, FTS, log,
  and index — no cross-project leakage.

Orchestration uses a tiny built-in state-graph runner so the offline suite has
zero third-party orchestration deps; a LangGraph-backed runner can implement the
same graph build for production.

## Production notes

- Claude usage lives behind `AnthropicLLM`: `model="claude-opus-4-8"`,
  `thinking={"type":"adaptive"}`, JSON-enveloped structured outputs for
  entity/contradiction extraction. Set `WIKI_LLM=anthropic` + `ANTHROPIC_API_KEY`.
- `OracleStore` mirrors the `Store` interface with `VECTOR(768, FLOAT32)` +
  `VECTOR_DISTANCE(..., COSINE)` and Oracle Text. It is a production seam and is
  never exercised by the offline gate.
- The append-only `log.jsonl` + `oplog` table is the audit trail; every mutation
  is one structured operation.
