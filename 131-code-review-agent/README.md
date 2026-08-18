# Coding Challenge #131 — Automated Code Review Agent

Solution to [Coding Challenge #131](https://codingchallenges.substack.com/p/coding-challenge-131-automated-code):
an agent that watches a repository, reviews every pull request that opens,
and posts its findings as a single comment — surviving provider outages,
coping with 60-file PRs, and grounding its opinions in real linter output
rather than vibes.

## Layout

```
fragile/review.py     Step 1 — the <100-line fragile script (documented flaws)
flows/                Steps 2-9 — Kestra workflow YAML, one file per step
src/reviewagent/      Testable Python core used by the CLI and step 8/9 flows
tests/                59 offline tests (mock provider + fake GitHub transport)
docker-compose.yml    Kestra OSS with SECRET_-prefixed env vars
.env.example          Every credential and knob
```

### The Python core (`review-agent` CLI)

| Module         | Step | What it does |
|----------------|------|--------------|
| `github.py`    | 0/1/4| PR metadata, changed files (paginated), post comment |
| `retry.py`     | 7    | Exponential backoff, Retry-After aware, never retries 401s |
| `http.py`      | 7    | Classifies 429/5xx/network as retryable, 4xx as fatal |
| `skiplist.py`  | 8    | Skips lock/generated/vendored/binary files with reasons |
| `prompts.py`   | 3    | Structured-output system prompt; guidelines are a parameter |
| `review.py`    | 3/4/8| Parses model JSON, merges per-file reviews, sorts CRITICAL-first, decides post-or-skip |
| `providers/`   | 5    | anthropic / openai (+Gemini-compatible) / ollama / mock — swap via 3 env vars |
| `linter.py`    | 9    | Clones PR head, runs `ruff --output-format json`, feeds findings as facts |
| `pipeline.py`  | 8    | Parallel per-file review (ThreadPoolExecutor), one merged comment |

## Quick start (no credentials needed)

```bash
cd 131-code-review-agent
pip install -e .[dev]
pytest                      # 59 tests, all offline
```

Real run against a PR:

```bash
export GITHUB_TOKEN=ghp_...
export REVIEW_PROVIDER=anthropic REVIEW_API_KEY=sk-ant-...
review-agent <owner> ReviewMe <pr-number> --dry-run   # print, don't post
review-agent <owner> ReviewMe <pr-number>             # post the comment
review-agent <owner> ReviewMe <pr-number> --lint ./checkout  # + ruff facts
```

Local model, zero cloud: `REVIEW_PROVIDER=ollama REVIEW_MODEL=llama3.1`.

## Step-by-step walkthrough

**Step 0 — sanity.** `curl -H "Authorization: Bearer $GITHUB_TOKEN" https://api.github.com/repos/<owner>/ReviewMe/pulls/1/files` must return a JSON array with `patch` fields. Fork [ReviewMe](https://github.com/JohnCrickett/ReviewMe) and open its flawed PRs.

**Step 1 — fragile script.** `python fragile/review.py <owner> ReviewMe 1`. It catches the Stripe key; break the API key and it dies with a bare stack trace. Its documented flaws (no retries, no rate-limit handling, no history, no visibility, one giant prompt) are the roadmap for every later step.

**Step 2 — workflow engine.** `docker compose up -d`, open http://localhost:8080, import `flows/step2_fetch_pr.yml`. Executions now persist with inputs, outputs, and logs.

**Step 3 — model integration.** `flows/step3_llm_review.yml` adds a structured-output review task. Guidelines are a flow *input*: delete the nesting rule at execution time and the nesting complaint disappears — no flow edit.

**Step 4 — post with judgment.** `flows/step4_post_review.yml` posts via the issues-comments endpoint, using `secret()` for tokens (OSS: `SECRET_`-prefixed, base64-encoded env vars — see `docker-compose.yml`). If the model reports `NO_ISSUES`, nothing posts and the log says why — no "looks good!" spam.

**Step 5 — provider swap.** Three lines change per provider (URI, auth header, model). The cheat-sheet and a token/latency comparison table are under [Provider swap cheat-sheet](#provider-swap-cheat-sheet). The Python core mirrors it: `REVIEW_PROVIDER` / `REVIEW_MODEL` / `REVIEW_API_KEY`.

**Steps 6+7 — webhook + resilience.** `flows/step6_7_webhook_resilient.yml`:
- Webhook trigger fires on `opened`/`reopened`/`synchronize` only; labels and comments are filtered out by an expression condition.
- `concurrency: {limit: 1, behavior: CANCEL}` — three rapid pushes run one review; superseded ones cancel.
- Every network task: 3 attempts, exponential backoff. Model call: 120 s read timeout.
- `errors:` block posts execution ID to Slack, so failure is loud.
- Tunnel with `ngrok http 8080`; webhook URL is `/api/v1/executions/webhook/reviewme/code_review_webhook/<key>`.

**Step 8 — large PRs.** `flows/step8_parallel_files.yml` fans out with `ForEach` (`concurrencyLimit: 4`) — one model call per file, visible as parallel branches in the execution graph. `uv.lock` and friends are skipped by pattern and listed in the comment. A merge task sorts all issues CRITICAL-first and posts exactly one comment.

**Step 9 — real static analysis.** `flows/step9_full_agent.yml` runs in a `python:3.12-slim` container: shallow-clone the PR head SHA, `ruff check . --output-format json`, feed findings into the prompt as *facts* (the system prompt orders the model not to re-derive or contradict them, and to spend its own commentary on intent/naming/architecture). Findings appear with exact `file:line` in the comment. Update the `pip install` line to point at your fork.

## Provider swap cheat-sheet

The `review` task in `step4_post_review.yml` is a plain HTTP call, so swapping providers touches the URI, the auth header and the model name, plus the response-extraction path once.

### Anthropic (baseline)

```yaml
uri: https://api.anthropic.com/v1/messages
headers: { x-api-key: "{{ secret('ANTHROPIC_API_KEY') }}", anthropic-version: "2023-06-01" }
model: "claude-sonnet-5"
# text at: json(outputs.review.body).content[0].text
```

### OpenAI

```yaml
uri: https://api.openai.com/v1/chat/completions
headers: { Authorization: "Bearer {{ secret('OPENAI_API_KEY') }}" }
model: "gpt-4o-mini"
# body uses messages: [{role: system, ...}, {role: user, ...}]
# text at: json(outputs.review.body).choices[0].message.content
```

### Google Gemini (OpenAI-compatible endpoint)

```yaml
uri: https://generativelanguage.googleapis.com/v1beta/openai/chat/completions
headers: { Authorization: "Bearer {{ secret('GEMINI_API_KEY') }}" }
model: "gemini-2.0-flash"
# text at: json(outputs.review.body).choices[0].message.content
```

### Local Ollama (no cloud)

```yaml
uri: http://host.docker.internal:11434/api/chat   # Kestra runs in Docker
headers: { }                                       # no key needed
model: "llama3.1"
# body needs "stream": false
# text at: json(outputs.review.body).message.content
```

### Comparison log (fill in per run against the same PR)

| Provider  | Model            | Input tokens | Output tokens | Wall clock | Caught Stripe key? | Notes |
|-----------|------------------|--------------|---------------|------------|--------------------|-------|
| Anthropic | claude-sonnet-5  |              |               |            |                    |       |
| OpenAI    | gpt-4o-mini      |              |               |            |                    |       |
| Gemini    | gemini-2.0-flash |              |               |            |                    |       |
| Ollama    | llama3.1         |              |               |            |                    |       |

Token counts: Anthropic `usage.input_tokens/output_tokens`; OpenAI/Gemini
`usage.prompt_tokens/completion_tokens`; Ollama `prompt_eval_count/eval_count`.

## Design decisions

- **Deterministic logic lives in tested Python, not in workflow templating.** Kestra owns triggering, concurrency, retries, secrets, and visibility; parsing/merging/skipping/formatting are code with 59 tests.
- **Providers are plain HTTP.** No SDKs — the diff between providers is genuinely three lines, and the mock provider makes the whole pipeline run offline.
- **Retry only what can heal.** 429/5xx/network errors retry with backoff (honouring `Retry-After`); 401/403/404 fail immediately.
- **Silence is a feature.** No issues → no comment, but the logs always say what was reviewed, what was skipped, and why.

