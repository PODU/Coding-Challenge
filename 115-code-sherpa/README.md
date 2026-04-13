# Code Sherpa

> Coding Challenge #115 — natural-language Q&A over unfamiliar codebases.

Point Sherpa at a repository. It walks the tree, splits source into logical
chunks (functions / classes / methods / modules), embeds and indexes them with
**both** a vector index and a full-text index, and then answers natural-language
questions like *"where do we handle payment failures?"* using **hybrid retrieval**
(vector + BM25 fused with Reciprocal Rank Fusion) feeding a grounded **LLM**
explanation with **citations** back to exact files and line ranges. It keeps
**episodic + semantic memory** across sessions and supports **multiple projects**
with **incremental re-ingest** of only changed files.

The LLM (Anthropic Claude `claude-opus-4-8`), the embedder, and the vector store
all sit **behind interfaces**, so the whole pipeline runs **fully offline** in
tests with a mock LLM, a deterministic fake embedder, and an in-memory store. No
network and no API key are needed to run the suite.

## Design

## Install

```bash
python -m pip install -e .            # core engine (zero hard deps)
python -m pip install -e ".[all]"     # + anthropic SDK + numpy acceleration
```

The core engine has **no required runtime dependencies** beyond the Python
standard library. `numpy` is used to accelerate the in-memory vector store when
present (a pure-Python cosine fallback is used otherwise). `anthropic` is only
needed for the real LLM client.

## CLI

```bash
sherpa ingest   myproj   /path/to/repo       # walk, chunk, embed, index
sherpa ask      myproj   "where do we handle payment failures?"
sherpa reingest myproj                       # incremental: only changed files
sherpa projects                              # list known projects
sherpa memory   myproj                       # show facts + recent questions
sherpa fact     myproj  gateway "we use Stripe"
```

Example:

```
$ sherpa ask myproj "where do we handle payment failures?"
Based on the retrieved code, here is what handles this. See src/payments.py:24-27.

Citations:
  - src/payments.py:24-27 (PaymentService.handle_payment_failure)
```

> The default in-memory store is process-local. `ask` transparently re-ingests a
> known project's root if the index is empty in the current process. For
> cross-process persistence, plug in a persistent store via `SHERPA_STORE`.

## Configuration

All configuration comes from environment variables (or an optional `sherpa.toml`),
with safe **offline defaults**. Secrets are read from the environment **only** and
never logged or written to disk.

| Variable | Default | Purpose |
|---|---|---|
| `SHERPA_LLM` | `anthropic` if `ANTHROPIC_API_KEY` set, else `mock` | LLM provider |
| `SHERPA_EMBEDDER` | `mock` | `mock` \| `coderank` |
| `SHERPA_STORE` | `memory` | `memory` \| `oracle` |
| `SHERPA_MODEL` | `claude-opus-4-8` | Claude model id |
| `ANTHROPIC_API_KEY` | — | **only** credential read for the LLM |
| `SHERPA_TOP_K`, `SHERPA_RRF_K`, `SHERPA_*_WEIGHT` | see `config.py` | retrieval tuning |
| `SHERPA_DB_DSN` / `SHERPA_DB_USER` / `SHERPA_DB_PASSWORD` | — | Oracle store |

If `SHERPA_LLM=anthropic` but no key is present, the app fails fast at startup
with a clear message rather than mid-query.

### Using the real Anthropic model

```bash
export ANTHROPIC_API_KEY=sk-ant-...
export SHERPA_LLM=anthropic           # auto-selected when the key is present
sherpa ask myproj "how does authentication work?"
```

## Programmatic use

```python
from code_sherpa import build_sherpa, SherpaConfig

sherpa = build_sherpa(SherpaConfig(llm="mock", embedder="mock", store="memory"))
sherpa.ingest("demo", "/path/to/repo")
answer = sherpa.ask("demo", "where do we validate inputs?")
print(answer.text)
for c in answer.citations:
    print(c.label())
```

## Architecture (interfaces)

| Concern | Interface | Offline impl | Production impl |
|---|---|---|---|
| Embedding | `embed.base.Embedder` | `MockEmbedder` (hashing) | `CodeRankEmbedder` |
| Storage | `store.base.VectorStore` | `InMemoryVectorStore` (cosine + BM25) | `OracleVectorStore` (23ai) |
| LLM | `llm.base.LLMClient` | `MockLLMClient` | `AnthropicClient` (`claude-opus-4-8`) |
| Memory | `memory.base.MemoryStore` | `InMemoryMemoryStore` | DB-backed |

Everything is wired by `core.sherpa.Sherpa` (the seam the CLI and a future
web/SSE layer both call) and selected by `config.py`.

## Tests

The suite is **fully offline and deterministic** — it forces the
mock/in-memory trio and makes no network calls.

```bash
python -m pip install -e ".[dev]"
python -m pytest
```

Covered: code chunking (functions/classes/methods, fallback, sub-split, merge),
hybrid retrieval (vector / text / fused, isolation, fallback), context selection
(budget, overlap de-dup, diversity, no-match), prompt building (golden structure,
memory, no-context), answer + citation handling, incremental re-ingest (only
changed files re-embedded; removals delete vectors), memory, and an end-to-end
orchestrator smoke test. Run with `ANTHROPIC_API_KEY` unset to confirm the suite
is genuinely offline.

## License

MIT.
