# LLM-Powered AI Chatbot — Coding Challenge #102

A terminal chatbot powered by **Anthropic Claude** (model `claude-opus-4-8`).
It holds multi-turn conversations, stays in a configurable personality defined
by a system prompt, **streams** responses token-by-token, and **remembers**
prior exchanges within a session.

All LLM access sits behind a single `LLMClient` interface, so the whole app —
prompt building, history management, streaming, the REPL — is exercised by a
test suite that runs **fully offline, with no API key and no network**, against
a `MockLLMClient`.

## Features

- REPL loop with streaming responses (`--no-stream` for full-reply mode)
- Built-in personas (`assistant`, `pirate`, `tutor`) or a custom system prompt
- Multi-turn conversation memory with a sliding history window
- Graceful handling of auth, rate-limit, service, and empty-response errors
- In-session commands: `/help`, `/reset`, `/exit` (`/quit`); clean `Ctrl-C` / `Ctrl-D`
- `ANTHROPIC_API_KEY` read from the environment only — never hardcoded or logged

## Setup

Requires Python 3.10+.

```bash
cd 102-llm-powered
python -m pip install -e ".[dev]"
```

Provide your API key via the environment (or a `.env` file):

```bash
export ANTHROPIC_API_KEY="sk-ant-..."        # macOS/Linux
# setx ANTHROPIC_API_KEY "sk-ant-..."        # Windows (new shells)
```

Or copy `.env.example` to `.env` and fill in the key — it is loaded
automatically (and `.env` is git-ignored).

## Run

```bash
# default assistant persona, streaming
chatbot

# pick a persona
chatbot --persona pirate

# custom system prompt
chatbot --system "You are a terse SRE on call. Answer in one sentence."
chatbot --system-file ./my_prompt.txt

# non-streaming, larger history window
chatbot --no-stream --max-turns 50

# Step 1 connectivity smoke check (one hardcoded prompt)
chatbot --selftest
```

You can also run it without installing the console script:

```bash
python -m chatbot.cli --persona tutor
```

Example session:

```
chatbot ready  [persona: assistant]
type /help for commands, /exit to quit
you> hello
bot> Hi! How can I help?
you> /exit
bye
```

## Test (offline — no key, no network)

```bash
python -m pytest
```

The suite uses `MockLLMClient` everywhere and never touches the network or
requires `ANTHROPIC_API_KEY`. It covers system-prompt construction, history
accumulation across turns, streaming aggregation, every error path, and REPL
command handling. `test_anthropic_client.py` injects a *fake* SDK object to
verify the real client's request shape and exception mapping — still offline.

## Architecture

```
src/chatbot/
  config.py            env loading, model id, defaults
  llm.py               LLMClient interface + data types + error taxonomy
  anthropic_client.py  AnthropicLLMClient (the only module importing `anthropic`)
  mock_client.py       MockLLMClient (offline, scripted, records calls)
  persona.py           Persona + built-in system prompts
  history.py           ConversationHistory (sliding window)
  session.py           ChatSession (orchestrates persona + history + client)
  cli.py               argparse, REPL loop, command handling, streaming render
```

`ChatSession` and the REPL depend only on the `LLMClient` protocol. Production
wires in `AnthropicLLMClient`; tests wire in `MockLLMClient`. The model id
`claude-opus-4-8` lives in exactly one place (`config.DEFAULT_MODEL`).

## Error handling

| Situation | Behavior |
|-----------|----------|
| Missing API key | `Config.from_env()` raises `LLMAuthError` at startup; REPL never starts |
| Auth error (401/403) | `LLMAuthError` → "check ANTHROPIC_API_KEY"; loop continues |
| Rate limit (429) | `LLMRateLimitError` → "wait and retry"; history not corrupted |
| Service / network (5xx) | `LLMServiceError` → "try again" |
| Empty response | `LLMEmptyResponse` → "try rephrasing" |
| Mid-stream failure | Partial deltas already shown; assistant turn NOT committed to history |

