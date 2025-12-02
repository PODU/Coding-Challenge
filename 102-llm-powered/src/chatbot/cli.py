"""Command-line entry point and the REPL loop."""

from __future__ import annotations

import argparse
import sys
from typing import TextIO

from .config import DEFAULT_MODEL, Config
from .history import ConversationHistory
from .llm import (
    LLMAuthError,
    LLMEmptyResponse,
    LLMRateLimitError,
    LLMServiceError,
    Message,
)
from .persona import BUILT_IN, Persona
from .session import ChatSession

HELP_TEXT = """\
commands:
  /help    show this help
  /reset   clear the conversation history
  /exit    quit (aliases: /quit; or Ctrl-C / Ctrl-D)
"""


def print_banner(persona: Persona, *, out: TextIO) -> None:
    out.write(f"chatbot ready  [persona: {persona.name}]\n")
    out.write("type /help for commands, /exit to quit\n")
    out.flush()


def print_help(*, out: TextIO) -> None:
    out.write(HELP_TEXT)
    out.flush()


def repl(
    session: ChatSession,
    *,
    stream: bool = True,
    out: TextIO | None = None,
    in_: TextIO | None = None,
) -> int:
    """Run the chat loop until the user quits. Returns the exit code."""
    out = out if out is not None else sys.stdout
    in_ = in_ if in_ is not None else sys.stdin

    print_banner(session.persona, out=out)
    while True:
        out.write("you> ")
        out.flush()
        try:
            line = in_.readline()
        except (EOFError, KeyboardInterrupt):
            out.write("\nbye\n")
            out.flush()
            return 0

        if line == "":  # Ctrl-D
            out.write("\nbye\n")
            out.flush()
            return 0

        cmd = line.strip()
        if cmd in ("/exit", "/quit"):
            out.write("bye\n")
            out.flush()
            return 0
        if cmd == "/reset":
            session.history.clear()
            out.write("history cleared\n")
            out.flush()
            continue
        if cmd == "/help":
            print_help(out=out)
            continue
        if not cmd:
            continue

        try:
            out.write("bot> ")
            if stream:
                for delta in session.send_stream(cmd):
                    out.write(delta)
                    out.flush()
                out.write("\n")
            else:
                out.write(session.send(cmd) + "\n")
            out.flush()
        except LLMAuthError:
            out.write("\n[auth error] check ANTHROPIC_API_KEY\n")
            out.flush()
        except LLMRateLimitError:
            out.write("\n[rate limited] wait and retry\n")
            out.flush()
        except LLMEmptyResponse:
            out.write("\n[no response] try rephrasing\n")
            out.flush()
        except LLMServiceError as e:
            out.write(f"\n[service error] {e}; try again\n")
            out.flush()


def _resolve_persona(args: argparse.Namespace) -> Persona:
    if args.system is not None:
        return Persona("custom", args.system)
    if args.system_file is not None:
        with open(args.system_file, "r", encoding="utf-8") as fh:
            return Persona("custom", fh.read())
    return BUILT_IN[args.persona]


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="chatbot",
        description="Terminal chatbot powered by Anthropic Claude.",
    )
    parser.add_argument(
        "--persona",
        choices=sorted(BUILT_IN),
        default="assistant",
        help="built-in persona / system prompt (default: assistant)",
    )
    parser.add_argument(
        "--system", help="custom system prompt (overrides --persona)"
    )
    parser.add_argument(
        "--system-file",
        help="read the custom system prompt from a file (overrides --persona)",
    )
    parser.add_argument(
        "--no-stream",
        dest="stream",
        action="store_false",
        help="disable streaming output",
    )
    parser.add_argument(
        "--max-tokens", type=int, default=1024, help="max tokens per reply"
    )
    parser.add_argument(
        "--max-turns",
        type=int,
        default=20,
        help="conversation history window, in turns",
    )
    parser.add_argument(
        "--model", default=DEFAULT_MODEL, help="model id override"
    )
    parser.add_argument(
        "--selftest",
        action="store_true",
        help="smoke check: send a hardcoded prompt and print the reply",
    )
    return parser


def _load_dotenv() -> None:
    """Load .env if python-dotenv happens to be installed."""
    try:
        from dotenv import load_dotenv  # type: ignore
    except Exception:
        return
    load_dotenv()


def _selftest(client, model: str, out: TextIO) -> int:
    out.write(f"[selftest] model={model}: sending a hardcoded prompt...\n")
    out.flush()
    try:
        result = client.complete(
            system="You are a helpful assistant.",
            messages=[Message("user", "Reply with exactly: connection ok")],
            max_tokens=64,
        )
    except LLMAuthError:
        out.write("[selftest] auth error: check ANTHROPIC_API_KEY\n")
        return 1
    except LLMRateLimitError:
        out.write("[selftest] rate limited: wait and retry\n")
        return 1
    except LLMEmptyResponse:
        out.write("[selftest] model returned no text\n")
        return 1
    except LLMServiceError as e:
        out.write(f"[selftest] service error: {e}\n")
        return 1
    out.write(f"[selftest] reply: {result.text}\n")
    out.flush()
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = _build_parser()
    args = parser.parse_args(argv)
    _load_dotenv()

    out = sys.stdout

    # Imported late so library/mock users never need the SDK installed.
    from .anthropic_client import AnthropicLLMClient

    try:
        cfg = Config.from_env(
            model=args.model,
            max_tokens=args.max_tokens,
            max_turns=args.max_turns,
        )
    except LLMAuthError as e:
        out.write(f"[fatal] {e}. Set it in your environment or a .env file.\n")
        return 1

    client = AnthropicLLMClient(cfg)

    if args.selftest:
        return _selftest(client, cfg.model, out)

    persona = _resolve_persona(args)
    history = ConversationHistory(max_turns=cfg.max_turns)
    session = ChatSession(client, persona, history, max_tokens=cfg.max_tokens)
    return repl(session, stream=args.stream, out=out, in_=sys.stdin)


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
