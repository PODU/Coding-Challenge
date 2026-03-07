# RTFM For Me Agent — Coding Challenge #110

An AI documentation assistant. Ask plain-English questions about a body of
technical docs (man pages, READMEs, API docs, runbooks) and get a **grounded,
cited** answer instead of grepping the docs by hand.

It is a **Retrieval-Augmented Generation (RAG)** system:

1. **Ingest** docs → split into ~500-token chunks → embed → store in a vector index.
2. **Answer** a question → embed it → retrieve the most relevant chunks → build a
   prompt with those chunks as grounded context → ask the LLM → return an answer
   **with source citations**.
3. **Cache** semantically-similar questions so near-duplicate questions skip the LLM.
4. **Remember** the conversation (session memory) and the user (long-term memory).

The LLM is **Anthropic Claude (`claude-opus-4-8`)** via the official `anthropic`
SDK. The key architectural rule: **every dependency that costs money or needs the
network — the LLM, the embedder, and the vector store — sits behind a narrow
interface, so the full test suite runs offline against deterministic fakes.**

---

## Architecture

```
 CLI / REST ─▶ Ingestion ─▶ Chunker ─▶ Embedder(iface) ─▶ VectorStore(iface) ─▶ Redis Stack
                                                                  │
 ask(question) ─▶ SemanticCache(iface) ──hit──▶ answer            │
        │ miss                                                     ▼
        ▼                                                      Retriever (KNN + hybrid filter)
   PromptBuilder ◀── SessionMemory + LongTermMemory                │
        │                                                          ▼
        ▼                                                   retrieved chunks
   LLMClient(iface) ─▶ AnthropicClient ──▶ Anthropic API (claude-opus-4-8)
        │            └▶ MockLLMClient (tests)
        ▼
   AnswerFormatter ─▶ Answer + citations
```

| Module | Responsibility |
|---|---|
| `config.py` | Env-first config. The **only** place `ANTHROPIC_API_KEY` is read. |
| `models.py` | Pure dataclasses: `Chunk`, `RetrievedChunk`, `Answer`, `Citation`, `Turn`, `AskResult`, `CacheStats`, `LLMResponse`. |
| `chunker.py` | ~500-token overlapping chunks + metadata (filename/section/position). Pure. |
| `embedder.py` | `Embedder` iface · `HashEmbedder` (deterministic, tests) · `LocalEmbedder` (offline model). |
| `vector_store.py` | `VectorStore` iface · `InMemoryVectorStore` (cosine/numpy) · `RedisVectorStore`. |
| `prompt_builder.py` | Pure: `(question, chunks, turns, memories) → (system, user)`. No assistant prefill. |
| `llm.py` | `LLMClient` iface · `AnthropicClient` (`claude-opus-4-8`) · `MockLLMClient` (tests). |
| `answer_formatter.py` | Pure: raw LLM text + chunks → `Answer` with deduped citations; handles not-found / refusal. |
| `semantic_cache.py` | `SemanticCache` iface · in-memory + Redis impls + metrics. |
| `session_memory.py` | Per-session turns with TTL. In-memory (testable clock) + Redis. |
| `long_term_memory.py` | Durable user facts. In-memory + Redis. |
| `retriever.py` / `ingest.py` / `agent.py` | Wire the core. `agent.ask()` does cache → retrieve → memory → prompt → LLM → format → persist, with graceful degradation. |
| `cli.py` / `api.py` | `rtfm` CLI and a thin FastAPI REST API. |

---

## Setup

Requires Python 3.12+.

```bash
# From this directory.
python -m pip install -e .            # core (numpy only)
python -m pip install -e '.[anthropic,redis,api,dev]'   # + real backends, REST API, test tools
```

Optional extras: `anthropic` (real LLM), `redis` (Redis Stack backends),
`api` (FastAPI REST), `local-embed` (offline embedding model), `dev` (pytest).

### `ANTHROPIC_API_KEY`

The real LLM backend reads the key from the **environment only**:

```bash
export ANTHROPIC_API_KEY="sk-ant-..."   # never hard-code it
```

Copy `.env.example` → `.env` (git-ignored) to document your local vars. The model
defaults to `claude-opus-4-8` (`RTFM_MODEL`). **Tests need no key.**

### Optional: Redis Stack

The Redis-backed store/cache/memory need Redis Stack (Redis + RediSearch):

```bash
docker compose up -d        # brings up Redis Stack on :6379 (UI on :8001)
export REDIS_URL=redis://localhost:6379
```

---

## Run

The default backends are **in-memory** (no Redis), so ingest and ask in one
process. The CLI exposes `--mock-llm` for a key-less demo and `--redis` to use
Redis Stack.

```bash
# Key-less demo against the bundled sample docs (mock LLM, in-memory backends):
python -m rtfm.cli --mock-llm ask "How do I view the commit log?" --ingest sample_docs

# Real answer (needs ANTHROPIC_API_KEY):
rtfm ask "What does grep -r do?" --ingest sample_docs
rtfm ask "and its -i flag?" --ingest sample_docs --session s1   # follow-up w/ session memory

# Persistent corpus across calls (needs Redis Stack):
rtfm --redis ingest sample_docs
rtfm --redis ask "How do I undo the last commit?"
rtfm --redis reset      # clear index + cache + memory

# REST API (needs the 'api' extra):
rtfm serve              # POST /ingest, POST /ask, GET /health, GET /metrics on :8000
```

`rtfm` is the installed console script; `python -m rtfm.cli` is equivalent.

### REST API

```bash
curl -s localhost:8000/ingest -H 'content-type: application/json' \
  -d '{"paths": ["sample_docs"]}'
curl -s localhost:8000/ask -H 'content-type: application/json' \
  -d '{"question": "How do I commit changes?", "session_id": "s1"}'
```

---

## Test

The **entire suite runs offline** — no `ANTHROPIC_API_KEY`, no network, no Redis —
by injecting `MockLLMClient`, `HashEmbedder`, and `InMemoryVectorStore`.

```bash
python -m pytest          # excludes the 'integration' marker by default
```

Covered: doc/man-page chunking (~500-token + metadata, edge cases, man-page
normalization), embedding + retrieval (relevant chunk retrieved, hybrid filter),
prompt construction (context + sources + grounding, rank order, memory injection,
no prefill, round-trip), answer + citation handling (used/deduped citations,
not-found, refusal), cache hit/miss/below-threshold, session memory
(order/isolation/TTL via a fake clock), long-term memory (extraction/recall/
dedupe), graceful degradation (a raising cache still answers), and the REST API.

Integration tests against a real Redis Stack and/or a real Anthropic call are
marked `@pytest.mark.integration` and excluded from the default run.

---

## Design notes / caveats

- **`claude-opus-4-8` specifics** baked into `AnthropicClient`: streaming +
  `get_final_message()` (timeout-safe), adaptive thinking only
  (`thinking={"type": "adaptive"}` — `budget_tokens`/`temperature` are removed on
  this model), no assistant prefill, and a `stop_reason == "refusal"` check before
  reading content.
- **Tokenization** for chunking is deliberately whitespace-based — deterministic
  and dependency-free, so chunk boundaries are reproducible and unit-testable. A
  production deployment can swap in a model tokenizer behind the same `count_tokens`.
- **`HashEmbedder`** is a deterministic bag-of-words hashing embedder used in tests
  and as a zero-dependency default. For real semantic quality, install the
  `local-embed` extra and pass `--local-embedder` (or wire a provider embedder
  behind the `Embedder` interface).
- **In-memory backends don't persist across processes.** For a durable corpus and
  cross-process cache/memory, use `--redis` with Redis Stack running.
- **Graceful degradation:** if the cache or long-term memory raises, `agent.ask`
  still answers via the normal retrieve→LLM path and records the fallback in
  `AskResult.degraded`. The vector store and LLM are treated as critical.
