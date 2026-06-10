"""Typed wrappers around ``LLMProvider.complete`` for the ingest/query graphs.

Each call sends a ``TASK:``-tagged system prompt and a JSON payload and expects
a JSON object back. ``ScriptedLLM`` dispatches on that tag.
"""

from __future__ import annotations

import json

from .interfaces import LLMProvider, Msg

_SYS = {
    "extract_entities": (
        "TASK: extract_entities\n"
        "You extract the distinct named entities (people, organizations, "
        "concepts) from a source document, each with its salient factual claims."
    ),
    "summarize": (
        "TASK: summarize\n"
        "You write a concise, faithful summary of a source document and assign "
        "topic tags."
    ),
    "judge": (
        "TASK: judge\n"
        "You judge whether two claims about the same subject 'agree', "
        "'contradict', or are 'unrelated'."
    ),
    "synthesize": (
        "TASK: synthesize\n"
        "You answer a question grounded ONLY in the provided wiki pages, citing "
        "each page you use with a numbered marker."
    ),
}


class Entity:
    __slots__ = ("name", "type", "salient_claims")

    def __init__(self, name: str, type: str, salient_claims: list[str]) -> None:
        self.name = name
        self.type = type
        self.salient_claims = salient_claims


class Agent:
    def __init__(self, llm: LLMProvider) -> None:
        self.llm = llm

    def _call(self, task: str, payload: dict) -> dict:
        msgs = [Msg(role="user", content=json.dumps(payload))]
        # Models occasionally return broken JSON, so allow one retry.
        for _ in range(2):
            res = self.llm.complete(_SYS[task], msgs, schema=None)
            try:
                return json.loads(res.text)
            except json.JSONDecodeError:
                pass
        raise ValueError(f"LLM returned malformed JSON for task {task!r}")

    def extract_entities(self, text: str) -> list[Entity]:
        data = self._call("extract_entities", {"text": text})
        return [
            Entity(e["name"], e.get("type", "concept"), e.get("salient_claims", []))
            for e in data.get("entities", [])
        ]

    def summarize(self, text: str, entity_names: list[str]) -> dict:
        return self._call("summarize", {"text": text, "entities": entity_names})

    def judge(self, claim_a: str, claim_b: str) -> str:
        return self._call("judge", {"claim_a": claim_a, "claim_b": claim_b}).get(
            "relation", "unrelated"
        )

    def synthesize(self, question: str, pages: list[dict]) -> dict:
        return self._call("synthesize", {"question": question, "pages": pages})
