# Coding Challenge #117 — AI Powered Support Bot

An interactive CLI customer-support bot for the fictional "Coding Challenges"
platform. A user asks a question in natural language; the bot answers **grounded
in a curated knowledge base** using Retrieval-Augmented Generation (RAG), keeps
multi-turn context, politely declines off-topic questions, and **escalates to a
human** when it has no confident, grounded answer.

The LLM is **Anthropic Claude `claude-opus-4-8`** behind a narrow interface.
Every dependency that costs money or needs the network — the LLM client and the
embedding/vector store — sits behind an interface with an in-memory mock, so the
**entire test suite runs fully offline with no API key**.

## Features

- **RAG grounding** — ingest → chunk → embed → retrieve top-k → ground the answer.
- **Grounded system prompt** — strict anti-hallucination + on-topic + abstention contract.
- **Multi-turn memory** — conversation history (and history-aware retrieval for pronoun follow-ups).
- **Escalation / human handoff** — no-match (`NO_CONTEXT`), low-confidence (`LOW_CONFIDENCE`),
  off-topic decline (`OFF_TOPIC`), explicit human request (`HUMAN_REQUEST`),
  repeated failure (`REPEAT_LIMIT`), provider failure (`PROVIDER_ERROR`).
- **Multi-provider fallback** — an ordered client chain fails over on transport errors.
- **Observability** — a per-turn `TurnLog` (request id, model, tokens, latency,
  estimated cost, fallback used, finish reason, escalation) and a `/stats` dashboard.
- **Cost-aware routing** — a query classifier routes simple queries to a cheaper
  model and complex queries to a more capable one.

## Install

Requires Python 3.10+.

```bash
pip install -e .          # core (anthropic + numpy)
pip install -e ".[dev]"   # + pytest
```

## Run the CLI

```bash
support-bot                  # console script
# or:
python -m support_bot.cli
```

Set a real key to use Claude; otherwise the CLI runs on the offline mock LLM:

```bash
export ANTHROPIC_API_KEY=sk-ant-...   # read from env only; never logged or stored
```

REPL commands: `/help`, `/reset`, `/history`, `/sources`, `/stats`, `/quit`.

```
You: How do I build my own Redis server?
Bot: ... (grounded answer)
Sources: Challenge Catalogue (challenges.md)
```

## Tests (fully offline)

```bash
python -m pytest
```

No network and no `ANTHROPIC_API_KEY` are required — all tests use
`MockLLMClient`, `MockEmbeddingProvider`, and `InMemoryVectorStore`. A single
live smoke test is quarantined behind `ANTHROPIC_API_KEY` **and** `RUN_LIVE=1`:

```bash
RUN_LIVE=1 ANTHROPIC_API_KEY=sk-ant-... python -m pytest tests/test_live_smoke.py
```

## Architecture (interfaces + DI)

```
SupportBot (orchestrator)
  Conversation.append(user)
  EscalationPolicy: human-request? off-topic?            (pre-LLM, no tokens spent)
  Retriever.search(query) ── EmbeddingProvider + VectorStore (interfaces)
    no hits ─────────────────────────────► Escalation(NO_CONTEXT)
  PromptBuilder.build(history, hits)                      (grounded system prompt)
  LLMClient.complete(prompt) ── Mock | Claude | Fallback | Routing (interfaces)
  EscalationPolicy: abstained? repeat-limit?             (post-LLM)
  MetricsSink.emit(TurnLog)                               (observability)
  return Answer | Escalation
```

Swappable implementations behind each interface:

| Interface           | Offline / default        | Production                         |
| ------------------- | ------------------------ | ---------------------------------- |
| `EmbeddingProvider` | `MockEmbeddingProvider`  | a real embeddings backend          |
| `VectorStore`       | `InMemoryVectorStore`    | pgvector / Pinecone / …            |
| `LLMClient`         | `MockLLMClient`          | `ClaudeClient` (`claude-opus-4-8`) |
| `MetricsSink`       | `InMemoryMetricsSink`    | `JsonlMetricsSink` / collector     |

The `support_bot.factory.build_bot(...)` function is the single assembly path
shared by production and tests; tests inject mocks, the CLI lets it pick the
real Claude stack (routing → fallback) when a key is present.

## Configuration

All via `support_bot.config.Config` (env-overridable), e.g. `SUPPORT_BOT_MODEL`,
`SUPPORT_BOT_TOP_K`, `SUPPORT_BOT_SIMILARITY_THRESHOLD`,
`SUPPORT_BOT_MAX_HISTORY_TOKENS`, `SUPPORT_BOT_MAX_CONTEXT_TOKENS`,
`SUPPORT_BOT_REPEAT_LIMIT`, `SUPPORT_BOT_KB_PATH`. The price table for cost
estimation lives in `config.PRICES`. `ANTHROPIC_API_KEY` is read from the
environment **only** and is never copied into config or written to a log.

## Knowledge base

Markdown files under `src/support_bot/kb/` (about, getting started, challenge
catalogue, FAQ). Point `SUPPORT_BOT_KB_PATH` at another directory of `*.md`
files to use your own. Ingestion is idempotent (keyed on a content hash).
