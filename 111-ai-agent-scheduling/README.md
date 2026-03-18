# AI Agent Scheduling — Coding Challenge #111

Run AI-powered tasks automatically on a **cron schedule** and deliver the results
by **email**. Each *agent* is a named unit of work (a task description plus an
optional system prompt). On its schedule the agent fires an LLM call, captures a
structured result, persists it to an execution history, and emails the formatted
result to a recipient.

This is a production-ready Python implementation of every step of the brief.

> Source: John Crickett's Coding Challenges, Challenge #111.

## Features (mapped to the challenge steps)

| Step | Feature | Where |
|---|---|---|
| 1 | AI agents (name + task + optional system prompt), structured result | `core/runner.py`, `models.py` |
| 2 | Five-field cron scheduling, enabled/disabled flag, **parallel** dispatch | `cron/expression.py`, `core/scheduler.py` |
| 3 | Natural-language → cron with human-readable confirmation | `cron/parser.py` |
| 4 | Per-execution timeout (default 60s) + exponential-backoff retries, non-blocking | `core/runner.py` |
| 5 | Email delivery with its **own independent** retry loop | `email/client.py`, `core/runner.py` |
| 6 | CRUD over agents (CLI), config + execution history in SQLite | `cli.py`, `core/repository.py` |
| 7 | Monitoring/status; flag agents with 3 consecutive failures as unhealthy | `core/health.py` |

The two external dependencies — the **LLM provider** and the **email provider** —
sit behind the `LLMClient` and `EmailClient` interfaces, so the entire
scheduling / execution / retry / health core is unit-testable **fully offline**
with mocked implementations (no API key, no network, injectable clock + sleep).

The reference LLM is **Anthropic Claude `claude-opus-4-8`** via the official
`anthropic` SDK (streaming + adaptive thinking), reached only through
`AnthropicLLMClient`. The API key comes exclusively from `ANTHROPIC_API_KEY`.

## Install

```bash
python -m pip install -e .          # runtime (pulls in anthropic)
python -m pip install -e ".[dev]"   # + pytest for the test-suite
```

Requires Python 3.11+. There is no `croniter` dependency — the 5-field cron parser
and next-fire calculator are implemented in-house so tests run fully offline.

## CLI

Installed as the `ai-agents` console script (also runnable as
`python -m ai_agent_scheduling.cli`):

```bash
# Preview an NL schedule before saving (Step 3)
ai-agents preview "every weekday at 9am"
#   Interpreted as `0 9 * * 1-5` - at 09:00, on Monday through Friday

# Create an agent — --schedule accepts cron OR natural language
ai-agents create news \
    --task "Summarize today's top AI news in 5 bullets." \
    --system-prompt "You are a concise news editor." \
    --schedule "every weekday at 9am" \
    --email ops@example.com -y

ai-agents list
ai-agents run news          # fire once now (manual)
ai-agents status            # system + per-agent health (Step 7)
ai-agents disable news
ai-agents edit news --schedule "every 30 minutes"
ai-agents delete news

ai-agents serve             # start the background scheduler loop
```

Without `ANTHROPIC_API_KEY` (or with `AI_AGENTS_OFFLINE=1`) the CLI falls back to a
harmless mock LLM so you can drive CRUD/preview/demos without credentials.

## Configuration (environment)

| Variable | Purpose | Default |
|---|---|---|
| `ANTHROPIC_API_KEY` | **Only** source of the LLM key (read by the SDK) | — |
| `LLM_MODEL` | Model id | `claude-opus-4-8` |
| `DB_PATH` | SQLite database file | `ai_agents.db` |
| `DEFAULT_TIMEOUT_S` / `DEFAULT_MAX_RETRIES` | Execution defaults | `60` / `3` |
| `BACKOFF_BASE_S` / `MAX_BACKOFF_S` | Retry backoff tuning | `1` / `60` |
| `SCHEDULER_TICK_S` / `MAX_PARALLEL_AGENTS` | Scheduler tuning | `1` / `8` |
| Email: `RESEND_API_KEY` + `EMAIL_FROM`, **or** `SMTP_HOST`/`SMTP_PORT`/`SMTP_USER`/`SMTP_PASSWORD`/`EMAIL_FROM` | Real email transport | mock (no-op recorder) |

Nothing reads environment variables deep in the call tree — a `Settings` object is
injected, and the API key is read by `anthropic.Anthropic()` itself.

## Library usage

```python
from ai_agent_scheduling import (
    Settings, Agent, AgentRepository, AgentRunner, Scheduler, HealthService,
    AnthropicLLMClient,
)
from ai_agent_scheduling.email.client import SmtpEmailClient

settings = Settings.from_env()
repo = AgentRepository(settings.db_path)
runner = AgentRunner(AnthropicLLMClient(), SmtpEmailClient("smtp.example.com"), repo, settings)
scheduler = Scheduler(repo, runner, settings)
scheduler.run_forever()
```

## Architecture

```
CLI ──▶ AgentRepository (SQLite: agents + executions)
          ▲
Scheduler ┴─▶ AgentRunner ──▶ LLMClient   (AnthropicLLMClient | MockLLMClient)
 cron next-fire   timeout+retry └─▶ EmailClient (Smtp/Resend | MockEmailClient)
 parallel dispatch              HealthService ──▶ status/health
```

- **Clean seams** (`LLMClient`, `EmailClient`, `AgentRepository`, injected
  `clock`/`sleep`) make the core deterministic, fast, and offline-testable.
- **Non-blocking**: the scheduler fans each due agent out onto a bounded
  `ThreadPoolExecutor`, so a slow or failing agent never blocks others or future
  runs. Two agents firing at the same instant run in parallel.
- **Independent retries**: the LLM execution loop and the email-delivery loop have
  separate attempt budgets; an email failure does not fail the execution.

## Testing

The whole suite is **offline** — no API key, no network, no real waiting (sleep and
the cron clock are injected):

```bash
python -m pytest
```

Coverage:

- `test_cron_parse.py` — cron parsing, validation, next-fire vs an injected clock
- `test_nl_cron.py` — NL→cron deterministic fallback **and** LLM path (scripted
  JSON via the mock), ambiguous-input rejection, confirmation string
- `test_runner.py` — agent run with the mock LLM produces a structured `Execution`
- `test_retry.py` — timeout/error retry with asserted exponential-backoff delays;
  a failed run does not affect other agents
- `test_email.py` — email delivery + independent retry via the mock; email failure
  doesn't fail the execution
- `test_health.py` — flagging after 3 consecutive failures; a later success clears
- `test_crud.py` — CRUD + history persistence against a temp SQLite DB; end-to-end
  create → run → persist → status
- `test_scheduler.py` — parallel dispatch with an injected clock; disabled agents
  skipped; a slow agent doesn't block others
- `test_cli.py` — CLI smoke tests (create/list/run/status/delete, all offline)

There is one optional, key-gated live smoke test path (the real Anthropic + SMTP
clients) that is simply never constructed by the offline suite.

## Notes / caveats

- **`croniter` intentionally not used.** The brief mentioned it, but the test
  requirement is a fully offline run with only `anthropic` + `pytest` installed, so
  the cron engine is hand-rolled. It supports `*`, `*/n`, `a-b`, `a-b/n`, lists, and
  standard day-of-month/day-of-week OR semantics. DST is handled naively (cron is
  evaluated against the datetime's own tzinfo; UTC is recommended).
- **FastAPI HTTP API** from the design is omitted to keep the install dependency-free
  and the suite offline; the management surface is the CLI (the brief asks for
  "API **or** CLI"). The `HealthService.report()` / `AgentRepository` objects are a
  ready seam to mount a FastAPI app on if desired.
- The real `AnthropicLLMClient` streams with `thinking={"type": "adaptive"}` (no
  `budget_tokens`, which 400s on Opus 4.8) and raises on `stop_reason == "refusal"`.
```
