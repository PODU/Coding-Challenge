# Mock Interview Coach — Coding Challenge #128

A web app for practicing behavioral interviews **face-to-face with a live AI
video interviewer**, built on [Anam](https://anam.ai)'s real-time persona
platform, with **Claude** (`claude-opus-4-8`) generating a STAR-framework
feedback report after each session.

Challenge: https://codingchallenges.substack.com/p/coding-challenge-128-mock-interview

## Features

- **Live video interviews** — photorealistic interviewer streamed into the
  browser via `@anam-ai/js-sdk`; you answer with your microphone.
- **Two interviewer styles** — *Sam*, a supportive coach, and *Alex*, a formal
  bar-raiser, with distinct voices and personalities.
- **Configurable interviews** — pick target role, seniority, competencies
  (leadership, conflict, failure, teamwork, delivery) and question count;
  questions are drawn randomly from a 24-question bank so every session differs.
- **Skip & early end** — skip questions or stop the interview at any time;
  skipped/unanswered questions are never scored.
- **Transcript capture & history** — every session is persisted and reviewable.
- **STAR feedback reports** — per answer: which of Situation / Task / Action /
  Result were present, a 1–5 score, and one concrete improvement; plus an
  overall strengths assessment and a progress chart across sessions.
- **Works fully offline** — with no API keys the app runs in text-chat mode
  with a deterministic interviewer and heuristic feedback, and the entire test
  suite needs no network.

## Setup

Requires Python 3.10+.

```bash
cd 128-mock-interview
python -m pip install -e ".[dev]"
```

Copy `.env.example` to `.env` (or export the variables) and fill in:

```bash
ANAM_API_KEY=...        # anam.ai → API keys (free tier: 30 min/month, 3-min sessions)
ANTHROPIC_API_KEY=...   # for real STAR feedback (otherwise heuristic mock)
```

Both keys are optional — missing keys switch the corresponding piece into mock
mode. The browser needs microphone access (granted automatically on
`http://localhost`).

## Run

```bash
mockinterview            # http://127.0.0.1:8000
# or: python -m mockinterview --port 8000
```

1. Pick role, seniority, competencies, interviewer style → **Start Interview**.
2. Speak your answers (or type them in offline mode). Skip or end any time;
   sessions auto-end inside the free tier's 3-minute limit.
3. Review your STAR report, then check **History & Progress** to see scores
   trend across sessions.

## Test

```bash
python -m pytest
```

Runs fully offline against `MockAnamClient` and `MockFeedbackClient`.

## Configuration

| Env var | Default | Purpose |
|---|---|---|
| `ANAM_API_KEY` | — | enables live video mode |
| `ANTHROPIC_API_KEY` | — | enables Claude feedback |
| `FEEDBACK_MODEL` | `claude-opus-4-8` | feedback model |
| `MAX_SESSION_SECONDS` | `170` | interview countdown |
| `MOCKINTERVIEW_DATA_DIR` | `data` | session storage |
| `ANAM_AVATAR_ID` / `ANAM_VOICE_ID` / `ANAM_ALT_VOICE_ID` / `ANAM_LLM_ID` | Cara defaults | Anam persona assets (update from Anam docs if ids rotate) |
