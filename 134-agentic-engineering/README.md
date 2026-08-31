# Agentic Engineering Graph (Coding Challenge #134)

A small orchestration system that runs an autonomous coding loop — plan,
write code, test, review, fix, retry — with durable file-based state, crash
recovery, a human approval gate, deterministic quality gates, and parallel
candidate fan-out with winner selection.

Challenge: <https://codingchallenges.substack.com/p/coding-challenge-134-agentic-engineering>

## Install & test

```sh
pip install -e .[dev]
pytest            # 33 tests, fully offline (mock agent + mock gate)
```

## Offline demo (no API key, no SonarQube)

The repo ships a fixture project with two seeded bugs and a scripted agent
session. The scripted "code" step fixes only one bug, so you can watch the
test-fail → fix → retest loop run:

```sh
agentloop start --task "fix the calculator bugs" \
  --project fixture/buggy_calc \
  --backend mock --mock-dir fixture/mock_session
# -> blocked: awaiting approval; plan is in runs/<id>/plan.md
agentloop approve <run-id>
agentloop resume <run-id>
agentloop runs                 # manifest of past runs + status
agentloop timeline <run-id>    # node transitions with durations (--mermaid for gantt)
```

Crash recovery: add `--stop-after 3` to `start` (simulated crash mid-run),
then `agentloop resume <run-id>` — it continues from the persisted state.

Fan-out: `--candidates 3` runs three isolated copies in parallel and the
join node copies the best one (fewest fix iterations, fastest) to
`runs/<id>/winner/`.

## Real integrations

```sh
# real agent: any agentic CLI supporting non-interactive prompts
agentloop start --task "..." --project path/ --backend claude --auto-approve

# deterministic quality gate via SonarQube
agentloop start ... --gate sonar \
  --sonar-host http://localhost:9000 --sonar-token $SONAR_TOKEN \
  --sonar-project-key my-key
```

`--gate mock --gate-fail-times N` exercises the gate-violation → fix →
re-scan loop without a SonarQube instance.

## Run directory layout

```
runs/<run-id>/
  state.json          # status, iteration, config — atomic writes
  workspace/          # private copy of the target project
  plan.md             # what the human approves
  last_failure.txt    # handed to the fix node
  transitions.jsonl   # timeline log
  artifacts/          # test outputs, gate results
  candidates/cand-N/  # nested full runs (fan-out)
  winner/             # join node's selected result
```

