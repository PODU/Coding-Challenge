"""Interactive coach REPL.

Runs live with ANTHROPIC_API_KEY (``pip install .[live]``), otherwise falls
back to a scripted offline demo via the mock client so the whole flow —
persona, tools, sessions, memory, costs, traces — is demonstrable with no
accounts.
"""

from __future__ import annotations

import argparse
import os
from pathlib import Path

from .agent import CoachAgent
from .costs import CostCapExceeded, CostTracker
from .llm import ToolCall
from .memory import MemoryBank
from .mock_client import MockLLMClient
from .sessions import SessionStore
from .telemetry import Tracer
from .tools import ToolRegistry
from .tools import executor as executor_tool
from .tools import specs as specs_tool
from .tools import test_runner as test_runner_tool
from .tools import web_search as web_search_tool

PACKAGE_ROOT = Path(__file__).resolve().parents[2]


def _demo_client() -> MockLLMClient:
    return MockLLMClient(
        script=[
            "Hi! I'm your Coding Challenges coach. What languages do you enjoy, "
            "and what would you like to get better at?",
            [ToolCall(id="t1", name="list_challenges", arguments={})],
            "Great choice. I can see a few specs locally — try asking me about "
            "one of them, e.g. 'tell me about the wc challenge'.",
            [ToolCall(id="t2", name="read_challenge_spec", arguments={"name": "01-wc.md"})],
            "The wc challenge asks you to rebuild the Unix word-count tool. "
            "Start with counting bytes (-c). Hint: read the file in binary "
            "mode. What's your plan for the first step?",
        ]
    )


def build_agent(
    *,
    data_dir: Path,
    challenges_dir: Path,
    user_id: str,
    session_id: str | None,
    cost_cap: float | None,
    live: bool,
) -> tuple[CoachAgent, CostTracker, Tracer]:
    registry = ToolRegistry()
    store = specs_tool.SpecStore(challenges_dir)
    specs_tool.register(registry, store)
    if live:
        web_search_tool.register(registry, web_search_tool.DuckDuckGoSearcher())
    else:
        web_search_tool.register(registry, web_search_tool.MockWebSearcher())
    sandbox = executor_tool.LocalCodeExecutor()
    executor_tool.register(registry, sandbox)
    test_runner_tool.register(registry, sandbox)

    sessions = SessionStore(data_dir / "sessions")
    session = (
        sessions.load_or_create(user_id, session_id)
        if session_id
        else sessions.create(user_id)
    )
    memory = MemoryBank(data_dir / "memory")
    costs = CostTracker(cap_usd=cost_cap)
    tracer = Tracer()

    if live:
        from .anthropic_client import AnthropicClient

        client = AnthropicClient()
    else:
        client = _demo_client()

    agent = CoachAgent(
        client,
        registry,
        session=session,
        session_store=sessions,
        memory=memory,
        costs=costs,
        tracer=tracer,
        catalog=store.list(),
    )
    return agent, costs, tracer


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="coach", description=__doc__)
    parser.add_argument("--user", default="local-dev")
    parser.add_argument("--session", default=None, help="resume a session by id")
    parser.add_argument(
        "--challenges", default=str(PACKAGE_ROOT / "challenges"), type=Path
    )
    parser.add_argument("--data-dir", default=str(PACKAGE_ROOT / ".coach-data"), type=Path)
    parser.add_argument("--cost-cap", type=float, default=1.00, help="USD per session")
    parser.add_argument("--traces", default=None, help="write spans to this JSONL file")
    args = parser.parse_args(argv)

    live = bool(os.environ.get("ANTHROPIC_API_KEY"))
    agent, costs, tracer = build_agent(
        data_dir=Path(args.data_dir),
        challenges_dir=Path(args.challenges),
        user_id=args.user,
        session_id=args.session,
        cost_cap=args.cost_cap,
        live=live,
    )
    mode = "live (Anthropic)" if live else "offline demo (mock client)"
    print(f"Coding Challenges Coach — {mode}")
    print(f"session: {agent.session.id}  (resume later with --session {agent.session.id})")
    print("type 'quit' to exit, '/costs' for the cost summary\n")

    try:
        while True:
            try:
                user_text = input("you> ").strip()
            except (EOFError, KeyboardInterrupt):
                break
            if not user_text:
                continue
            if user_text.lower() in {"quit", "exit"}:
                break
            if user_text == "/costs":
                print(costs.summary())
                continue
            try:
                result = agent.turn(user_text)
            except CostCapExceeded as exc:
                print(f"coach> [stopped] {exc}")
                break
            print(f"coach> {result.text}\n")
    finally:
        if args.traces:
            tracer.export_jsonl(args.traces)
            print(f"traces written to {args.traces}")
        print(f"cost summary: {costs.summary()}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
