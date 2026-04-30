"""Interactive command-line chat."""

from __future__ import annotations

import os
import sys

from .config import Config
from .factory import build_bot
from .models import Answer, Escalation
from .observability import InMemoryMetricsSink

_BANNER = "Coding Challenges Support Bot - type /help for commands, /quit to exit."
_HELP = """\
Commands:
  /help      show this help
  /reset     clear conversation history
  /history   show the conversation so far
  /sources   show sources cited in the last answer
  /stats     show observability stats for this session
  /quit      exit"""


def _print_answer(answer: Answer) -> None:
    print(f"\nBot: {answer.text}")
    if answer.citations:
        sources = dict.fromkeys(f"{c.title} ({c.source})" for c in answer.citations)
        print("Sources: " + "; ".join(sources))


def _print_escalation(esc: Escalation) -> None:
    label = "Escalation" if esc.human_handoff else "Notice"
    print(f"\nBot [{label}: {esc.reason.value}]: {esc.message}")


def main(argv: list[str] | None = None) -> int:
    config = Config.from_env()
    metrics = InMemoryMetricsSink()
    bot = build_bot(config=config, metrics=metrics)

    print(_BANNER)
    if not os.environ.get("ANTHROPIC_API_KEY"):
        print("(No ANTHROPIC_API_KEY set - running with the offline mock LLM.)")

    last_answer: Answer | None = None
    while True:
        try:
            line = input("\nYou: ").strip()
        except (EOFError, KeyboardInterrupt):
            print()
            break

        if not line:
            continue
        if line in ("/quit", "/exit"):
            break
        if line == "/help":
            print(_HELP)
            continue
        if line == "/reset":
            bot.reset()
            last_answer = None
            print("(history cleared)")
            continue
        if line == "/history":
            for turn in bot.conversation.turns:
                print(f"  {turn.role}: {turn.content}")
            continue
        if line == "/sources":
            if last_answer and last_answer.citations:
                for c in last_answer.citations:
                    print(f"  - {c.title} ({c.source})")
            else:
                print("  (no sources from the last turn)")
            continue
        if line == "/stats":
            for key, value in metrics.stats().items():
                print(f"  {key}: {value}")
            continue

        result = bot.turn(line)
        if isinstance(result, Answer):
            last_answer = result
            _print_answer(result)
        else:
            last_answer = None
            _print_escalation(result)

    return 0


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
