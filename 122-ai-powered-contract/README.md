# Contract Review — Coding Challenge #122

An **AI-powered contract review agent**: ingest a contract (PDF or text), split
it into numbered clauses, analyse each clause for risk, aggregate a review
report, run a persisted **human-in-the-loop** approve/reject/annotate workflow,
generate a professional summary that folds in reviewer feedback, and answer
grounded free-form questions about the contract.

The LLM is **Anthropic Claude `claude-opus-4-8`**, hidden behind a small
interface so the **entire test suite runs fully offline** against a scripted
mock — no API key, no network.

## Pipeline

```
contract.pdf/.txt ─▶ ingest.load_contract ─▶ chunking.chunk_document
   ─▶ clauses.extract_clauses ─▶ analysis.analyse_all ─▶ report.build_report
   ─▶ review.ReviewSession (persisted pause/resume waitpoint)
   ─▶ summary.build_summary           qa.answer_question (any time)
```

Every structured artefact is a **Pydantic** model, so malformed LLM output is
rejected at the boundary rather than propagating downstream.

## Install

```bash
python -m pip install -e ".[pdf,test]"   # pdf extra pulls in pypdf
```

Requires Python 3.11+. Core deps: `pydantic`, `anthropic`. `pypdf` is optional
(plain text/markdown always works; PDF tests are skipped if it is absent).

## Configuration

All configuration comes from the environment (with safe defaults):

| Variable | Default | Meaning |
| --- | --- | --- |
| `CONTRACT_LLM_PROVIDER` | `claude` | `claude` (real) or `mock` (offline) |
| `CONTRACT_LLM_MODEL` | `claude-opus-4-8` | Model id |
| `CONTRACT_MAX_CHUNK_TOKENS` | `6000` | Max tokens per chunk |
| `CONTRACT_CHUNK_OVERLAP` | `400` | Token overlap between chunks |
| `CONTRACT_ANALYSIS_WORKERS` | `4` | Concurrent clause-analysis workers |
| `CONTRACT_BATCH_SIZE` | `8` | Analysis batch size |
| `CONTRACT_STORE_DIR` | `./.contract_sessions` | Where session JSON is persisted |

The Anthropic API key is read **only** from `ANTHROPIC_API_KEY`, lazily, the
first time a real network call is made — it is never written to disk, logs, or
session files, and is never a CLI argument. Importing the package and running
the offline test suite require no key.

## CLI

```bash
export ANTHROPIC_API_KEY=sk-ant-...        # only for the real provider

contract-review analyse  contract.pdf --session-id demo        # → pauses for review
contract-review review   --session demo --approve 1,3 --reject 5 --annotate '4=needs a cap'
contract-review resume   --session demo                        # → triggers the summary
contract-review summary  --session demo [--stream]
contract-review ask      --session demo "What is the termination notice period?"
```

`analyse --no-pause` runs straight through (resume + summary) without the
human-in-the-loop pause.

## Library

```python
from contract_review import (
    Config, make_client, load_contract, extract_clauses, analyse_all,
    build_report, new_session, JSONFileStore, apply_decisions, resume,
    build_summary, answer_question,
)
from contract_review.models import ReviewDecision

config = Config.from_env()
llm = make_client(config)                      # ClaudeClient, or MockLLM if provider=mock

doc      = load_contract("contract.pdf")
clauses  = extract_clauses(doc, llm, config)
analyses = analyse_all(clauses, llm, config)
report   = build_report(clauses, analyses)

store   = JSONFileStore(config.store_dir)
session = new_session(report, contract_text=doc.text, store=store)   # waitpoint

apply_decisions(session, [ReviewDecision(clause_number=1, decision="approve")], store=store)
resume(session, store=store)

summary = build_summary(session.report, session.decisions, llm)
answer  = answer_question("What is the notice period?", session.report, llm)
```

## Offline testing

The LLM is the only nondeterministic dependency and is fully isolated behind the
`LLMClient` interface. Tests force `CONTRACT_LLM_PROVIDER=mock` and answer every
call with a scripted `MockLLM`.

```bash
python -m pytest          # 61 tests, fully offline
```

Coverage: document load + normalisation, boundary-aware/overlapping/token-budget
chunking, prompt building, structured-output parsing + Pydantic validation,
clause extraction (incl. cross-chunk dedup), risk analysis + aggregation
(incl. malformed-clause degradation and concurrency ordering), session
pause→resume→annotate persistence, grounded Q&A, summary (incl. streaming), and
the full `analyse → review → resume → summary → ask` CLI flow.

## Design notes

- **Provider abstraction** — swap providers with one env var; `make_client` is
  the single seam.
- **Structured outputs** — clause extraction and per-clause analysis request a
  JSON schema (`output_config.format`); parsing additionally repairs fenced /
  prose-wrapped JSON and normalises out-of-enum risk values (`critical`→`high`).
- **Resilience** — a clause whose analysis can't be parsed degrades to a
  `high`-risk placeholder rather than aborting the run; malformed/encrypted/
  image-only documents raise `MalformedDocumentError` with a clean CLI message.
- **Grounded Q&A** — lexical retrieval narrows context; the prompt constrains
  the model to answer only from supplied clauses and to emit a sentinel
  (`grounded=False`, "This contract does not address that.") when the answer is
  absent — it never fabricates.

## Caveats

- PDF extraction uses `pypdf` and handles text-layer PDFs; image-only (scanned)
  PDFs have no text layer and raise `MalformedDocumentError` (OCR is out of
  scope). PDF ingest tests are skipped when `pypdf` is not installed.
- Offline token counting uses a conservative ~4-chars/token heuristic; the real
  Claude tokenizer would differ slightly but the chunking guarantees
  (boundary-aware, overlap, budget) hold either way.
- The real Anthropic path is implemented but exercised only when
  `CONTRACT_LLM_PROVIDER=claude` and a key is present; CI/tests never touch it.
