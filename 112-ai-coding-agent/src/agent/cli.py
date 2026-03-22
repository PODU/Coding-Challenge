"""Command-line entry point.

Usage::

    ai-coding-agent "Add a docstring to utils.py" --workspace . --permission prompt
    python -m agent.cli "Run the tests and fix the failure" --permission allow

Tool errors go back to the model. The process only exits non-zero for things
the agent can't recover from: a bad workspace, no API client, or hitting the
iteration cap.
"""

from __future__ import annotations

import argparse
import sys

from agent.config import (
    DEFAULT_MAX_ITERATIONS,
    DEFAULT_MAX_TOKENS,
    DEFAULT_MODEL,
    DEFAULT_PERMISSION_MODE,
)
from agent.errors import MaxIterationsError
from agent.loop import Agent
from agent.permissions import VALID_MODES, PermissionPolicy
from agent.tools import default_registry
from agent.workspace import Workspace


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="ai-coding-agent",
        description="A CLI AI coding agent driving Claude with sandboxed tools.",
    )
    parser.add_argument("task", help="The coding task for the agent to perform.")
    parser.add_argument(
        "--workspace",
        default=".",
        help="Sandbox root directory (default: current directory).",
    )
    parser.add_argument(
        "--permission",
        choices=VALID_MODES,
        default=DEFAULT_PERMISSION_MODE,
        help="run_command policy: allow / prompt / deny (default: prompt).",
    )
    parser.add_argument(
        "--model",
        default=DEFAULT_MODEL,
        help=f"Anthropic model id (default: {DEFAULT_MODEL}).",
    )
    parser.add_argument(
        "--max-tokens",
        type=int,
        default=DEFAULT_MAX_TOKENS,
        help=f"Max output tokens per call (default: {DEFAULT_MAX_TOKENS}).",
    )
    parser.add_argument(
        "--max-iterations",
        type=int,
        default=DEFAULT_MAX_ITERATIONS,
        help=f"Agentic loop iteration cap (default: {DEFAULT_MAX_ITERATIONS}).",
    )
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)

    if not args.task or not args.task.strip():
        print("error: task must not be empty", file=sys.stderr)
        return 2

    try:
        workspace = Workspace(args.workspace)
    except (FileNotFoundError, ValueError) as exc:
        print(f"error: invalid workspace: {exc}", file=sys.stderr)
        return 2

    policy = PermissionPolicy(mode=args.permission)
    registry = default_registry(policy)

    # Imported late so a broken anthropic install only breaks real runs.
    try:
        from agent.llm import AnthropicLLMClient

        llm = AnthropicLLMClient(model=args.model, max_tokens=args.max_tokens)
    except Exception as exc:  # noqa: BLE001 - surface a friendly setup hint
        print(
            f"error: could not initialise the Anthropic client: {exc}\n"
            "Make sure the 'anthropic' package is installed and ANTHROPIC_API_KEY "
            "is set in your environment.",
            file=sys.stderr,
        )
        return 1

    agent = Agent(
        llm=llm,
        registry=registry,
        workspace=workspace,
        max_iterations=args.max_iterations,
    )

    try:
        final = agent.run(args.task)
    except MaxIterationsError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1
    except Exception as exc:  # noqa: BLE001 - e.g. SDK auth/network errors
        print(f"error: agent run failed: {exc}", file=sys.stderr)
        return 1

    print(final)
    return 0


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
