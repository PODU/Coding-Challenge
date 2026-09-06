"""The transcript cleanup pipeline: raw ASR text in, text worth pasting out.

Stage order matters and is deliberate:

    tokenize -> commands -> lists -> fillers -> repeats -> dictionary -> casing -> render

Spoken commands run first because the list stage needs to see the breaks they create.
The dictionary runs before casing so a correction like "github" -> "GitHub" is not
undone by the sentence-caser.
"""

from __future__ import annotations

from dataclasses import dataclass

from . import cleanup, listfmt
from .commands import apply_commands
from .config import Config
from .tokens import Token, render, tokenize


@dataclass
class PipelineTrace:
    """What each stage produced — `ccvoice format --explain` prints this."""

    stages: list[tuple[str, str]]

    def render(self) -> str:
        width = max((len(name) for name, _ in self.stages), default=0)
        return "\n".join(f"{name.ljust(width)} | {text!r}" for name, text in self.stages)


class TextPipeline:
    def __init__(self, config: Config | None = None):
        self.config = config or Config()

    def run(self, raw: str) -> str:
        return self.run_traced(raw)[0]

    def run_traced(self, raw: str) -> tuple[str, PipelineTrace]:
        cfg = self.config
        trace: list[tuple[str, str]] = [("raw", raw)]
        tokens: list[Token] = tokenize(raw)

        def step(name: str, new: list[Token]) -> list[Token]:
            trace.append((name, render(new)))
            return new

        if cfg.spoken_commands:
            tokens = step("commands", apply_commands(tokens))
        if cfg.detect_lists:
            tokens = step(
                "lists",
                listfmt.apply_lists(tokens, style=cfg.list_style, bullet=cfg.bullet_marker),
            )
        if cfg.cleanup:
            tokens = step("fillers", cleanup.remove_fillers(tokens, cfg.fillers, cfg.filler_phrases))
            if cfg.collapse_repeats:
                tokens = step("repeats", cleanup.collapse_repeats(tokens))
        if cfg.dictionary:
            tokens = step("dictionary", cleanup.apply_dictionary(tokens, cfg.dictionary))
        if cfg.detect_lists:
            tokens = step("list intro", listfmt.punctuate_intro(tokens))
        if cfg.cleanup:
            tokens = step("punctuation", cleanup.normalize_punctuation(tokens))
            if cfg.terminal_period and not listfmt.ends_with_list_item(tokens):
                tokens = step("terminal", cleanup.ensure_terminal_punctuation(tokens))
        if cfg.sentence_case:
            tokens = step("casing", cleanup.apply_casing(tokens))

        text = render(tokens)
        trace.append(("final", text))
        return text, PipelineTrace(trace)


def word_count(text: str) -> int:
    return len([w for w in text.split() if any(ch.isalnum() for ch in w)])
