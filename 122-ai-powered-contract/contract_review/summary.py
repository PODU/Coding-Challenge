"""Writing the final review summary."""

from __future__ import annotations

from typing import Iterator

from .llm import LLMClient
from .models import ReviewDecision, ReviewReport, Summary
from .parsing import parse_summary
from .prompts import SUMMARY_SCHEMA, build_summary_prompt

_SUMMARY_MAX_TOKENS = 4096


def stream_summary(
    report: ReviewReport,
    decisions: list[ReviewDecision],
    llm: LLMClient,
) -> Iterator[str]:
    """Yield the model's raw summary text as it streams in."""

    system, messages = build_summary_prompt(report, decisions)
    yield from llm.stream(system, messages, max_tokens=_SUMMARY_MAX_TOKENS, intent="summary")


def build_summary(
    report: ReviewReport,
    decisions: list[ReviewDecision],
    llm: LLMClient,
    stream: bool = False,
) -> Summary:
    if stream:
        raw = "".join(stream_summary(report, decisions, llm))
    else:
        system, messages = build_summary_prompt(report, decisions)
        raw = llm.complete(
            system,
            messages,
            output_schema=SUMMARY_SCHEMA,
            max_tokens=_SUMMARY_MAX_TOKENS,
            intent="summary",
        )
    return parse_summary(raw)
