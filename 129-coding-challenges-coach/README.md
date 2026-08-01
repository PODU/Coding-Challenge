# Coding Challenges Coach (Coding Challenge #129)

An AI coaching agent that helps developers learn through Coding Challenges:
it converses about your goals, recommends challenges, reads local challenge
specs, searches the web, runs your code in a sandbox, runs tests against it,
and remembers what it already told you across sessions — with cost caps,
tracing, policy filtering and a security audit on top.

Challenge: https://codingchallenges.substack.com/p/coding-challenge-129-coding-challenges

## Run it

No dependencies. Offline demo mode (scripted mock model, real tools):

```bash
cd 129-coding-challenges-coach
PYTHONPATH=src python -m coach
```

Live mode against the Anthropic API:

```bash
pip install .[live]
ANTHROPIC_API_KEY=... PYTHONPATH=src python -m coach --cost-cap 0.50 --traces traces.jsonl
```

Useful flags: `--session <id>` resume a session, `--cost-cap <usd>`
per-session spend cap, `--traces <file>` export spans as JSONL, `/costs` in
the REPL for the running total.

## Security audit (Step 8)

```bash
PYTHONPATH=src python -m coach.audit
```

Checks: zero static credentials in the tree, least-privilege IAM roles,
encrypted state storage, debug endpoints answer 404 — exit 0 only if all pass.

## Cloud deployment (Steps 6-7)

Requires a Google Cloud account — see `deploy/README.md` for the
`adk deploy` / Agent Identity (SPIFFE) procedure and
`deploy/agent_config.yaml` for the audited deployment posture.

## Tests

```bash
python -m pytest
```

46 tests, fully offline (JavaScript/Go executor tests auto-skip if node/go
are not installed).
