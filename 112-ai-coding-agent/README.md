# AI Coding Agent — Coding Challenges #112

A small command-line **AI coding agent**: a minimal Claude Code in spirit. It pairs
Anthropic's Claude (`claude-opus-4-8`) with a set of sandboxed file-system and shell
tools and drives an **agentic loop** — the model decides which tools to call, the
harness executes them against a **workspace directory**, returns the results, and
repeats until the model finishes or a safety limit is hit.

The model talks to the harness through Anthropic's native **tool use** protocol:
it emits `tool_use` content blocks; the harness executes each tool and replies with
matching `tool_result` blocks in a single user turn. Conversation state is carried
across iterations because the Messages API is stateless.

The LLM is abstracted behind an interface (`LLMClient`). The real implementation
calls the Anthropic SDK; the test suite injects a **mock** that emits scripted
`tool_use` blocks, so the entire suite runs **offline** — no API key, no network.

## Install

```bash
# from this directory
python -m pip install -e .          # installs the package + the anthropic SDK
# or, for development (adds pytest):
python -m pip install -e ".[dev]"
```

The agent reads its API key from the `ANTHROPIC_API_KEY` environment variable only:

```bash
export ANTHROPIC_API_KEY=sk-ant-...        # macOS / Linux
$env:ANTHROPIC_API_KEY = "sk-ant-..."      # PowerShell
```

Copy `.env.example` to `.env` (git-ignored) as a reminder of what to set.

## Usage

```bash
# Run a task against the current directory; prompt before each shell command.
ai-coding-agent "Add a docstring to utils.py" --workspace . --permission prompt

# Headless / CI: allow shell commands without prompting.
ai-coding-agent "Run the tests and fix the failing one" --permission allow

# Equivalent module form:
python -m agent.cli "List the Python files" --workspace ./myproject --permission deny
```

### Options

| Flag | Default | Meaning |
|------|---------|---------|
| `task` (positional) | — | The coding task to perform. |
| `--workspace` | `.` | Sandbox root. Resolved to an absolute real path at startup. |
| `--permission` | `prompt` | `run_command` policy: `allow` / `prompt` / `deny`. |
| `--model` | `claude-opus-4-8` | Anthropic model id. |
| `--max-tokens` | `16000` | Max output tokens per API call. |
| `--max-iterations` | `25` | Hard cap on agentic loop iterations. |

## Tools

| Tool | Input | Behaviour |
|------|-------|-----------|
| `read_file` | `{path}` | Return a file's contents (workspace-confined). |
| `write_file` | `{path, content}` | Create/overwrite a file; makes parent dirs. |
| `list_files` | `{path?}` | Recursive listing relative to the workspace. |
| `run_command` | `{command}` | Run a shell command in the workspace, permission-gated. |

## Security notes

Security centres on **confining every tool to the workspace directory** and
**guarding shell execution**.

- **Workspace path confinement.** Every model-supplied path is untrusted output.
  All file operations resolve through `Workspace.resolve`, which canonicalises the
  path and rejects anything that escapes the root — defeating `..` traversal,
  absolute paths, and symlink escapes (`SandboxViolation`). File tools never call
  `open()` on a raw model path.
- **`run_command` permission gate (allow / prompt / deny).**
  - `deny` — shell execution is disabled entirely (the safe default in tests).
  - `prompt` (default) — the human confirms each command before it runs; declining
    returns a recoverable error to the model.
  - `allow` — runs without prompting (CI / headless; opt-in only).
  Commands run with `cwd` pinned to the workspace root, `shell=False` (argv split,
  no shell-metacharacter interpretation), a wall-clock timeout, and truncated output.
- **Recoverable vs. fatal errors.** Tool failures (missing file, bad input, sandbox
  violation, denied command) are returned to the model as `tool_result` errors with
  `is_error: true` — they never crash the agent. The model can see them and adapt.
- **`max_iterations`** bounds the loop so a misbehaving model cannot tool-call forever.
- **File reads/writes are size-capped**, and non-UTF-8 reads are rejected with a
  clear tool error.
- **The API key is never hardcoded, logged, or written into conversation history** —
  it is read from `ANTHROPIC_API_KEY` by the SDK at call time.

> The `run_command` argv split uses POSIX rules; commands containing Windows-style
> backslash paths should be written with forward slashes or quoted accordingly.

## Tests

The whole suite is **offline** — it injects a `MockLLMClient` that returns scripted
`tool_use` blocks, and runs every tool against a temporary `tmp_path` workspace. No
`ANTHROPIC_API_KEY` is needed.

```bash
python -m pip install -e ".[dev]"
python -m pytest
```

Coverage includes: each tool against a temp workspace, path-escape rejection,
the permission gate (allow/prompt/deny + timeout + cwd), registry dispatch and
tool-error handling, and the full agentic loop (read→write→stop, conversation-state
correctness, tool-error recovery, the max-iteration guard, and end-to-end sandbox
enforcement).

## Layout

```
src/agent/
├── cli.py          # argparse entry point (console script: ai-coding-agent)
├── config.py       # model id / defaults
├── llm.py          # LLMClient interface + AnthropicLLMClient
├── loop.py         # Agent: the agentic loop + conversation state
├── workspace.py    # Workspace: path-confinement sandbox
├── permissions.py  # PermissionPolicy: allow / prompt / deny
├── errors.py       # ToolError, SandboxViolation, MaxIterationsError
└── tools/          # read_file, write_file, list_files, run_command + registry
tests/              # offline pytest suite (MockLLMClient + tmp_path workspace)
```
