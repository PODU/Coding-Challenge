"""LLM providers.

``ScriptedLLM`` doesn't call a model. It reads the ``TASK:`` tag from the
system prompt and answers with simple heuristics, so the pipelines produce
the same output on every run. ``script`` can supply canned results per task.

``AnthropicLLM`` sends the same prompts to Claude.
"""

from __future__ import annotations

import json
import re

from .interfaces import LLMResult, Msg
from .text import content_tokens, numbers, sentences, tokenize

TASK_TAG = "TASK:"
_NEG = {"not", "no", "never", "cannot", "none", "without", "isnt", "wasnt", "didnt", "dont"}
_ORG_HINTS = {
    "inc", "corp", "corporation", "company", "ltd", "llc", "university",
    "institute", "lab", "laboratory", "foundation", "organization", "labs",
}
_PROPER_RE = re.compile(r"[A-Z][A-Za-z0-9]+(?:\s+[A-Z][A-Za-z0-9]+)*")
# Single capitalized words that are usually sentence-initial noise, not entities.
_NOISE = {
    "The", "A", "An", "This", "That", "These", "Those", "It", "He", "She",
    "They", "We", "You", "I", "In", "On", "At", "By", "For", "And", "But",
    "Or", "Their", "His", "Her", "Its", "As", "Both", "Each", "When", "While",
    "However", "Although", "Because", "Unlike", "Some", "Many", "Most",
}


class ScriptedLLM:
    def __init__(self, script: dict | None = None) -> None:
        # {task: {text_or_question: result}}
        self.script = script or {}

    def complete(self, system: str, messages: list[Msg], *, schema: dict | None = None) -> LLMResult:
        task = self._task_of(system)
        payload = self._payload_of(messages)
        override = self._scripted(task, payload)
        if override is not None:
            return LLMResult(text=json.dumps(override), raw={"task": task, "scripted": True})
        handler = getattr(self, f"_task_{task}", None)
        if handler is None:
            raise ValueError(f"ScriptedLLM: unknown task {task!r}")
        return LLMResult(text=json.dumps(handler(payload)), raw={"task": task})

    @staticmethod
    def _task_of(system: str) -> str:
        for line in system.splitlines():
            if line.strip().startswith(TASK_TAG):
                return line.split(TASK_TAG, 1)[1].strip()
        return "unknown"

    @staticmethod
    def _payload_of(messages: list[Msg]) -> dict:
        for msg in reversed(messages):
            if msg.role == "user":
                try:
                    return json.loads(msg.content)
                except json.JSONDecodeError:
                    return {"text": msg.content}
        return {}

    def _scripted(self, task: str, payload: dict):
        table = self.script.get(task)
        if not table:
            return None
        key = payload.get("text") or payload.get("question") or ""
        return table.get(key)

    def _task_extract_entities(self, payload: dict) -> dict:
        text = payload.get("text", "")
        sents = sentences(text)
        found: dict[str, dict] = {}
        for phrase in self._proper_phrases(text):
            if phrase in found:
                continue
            words = phrase.split()
            if any(w.lower() in _ORG_HINTS for w in words):
                etype = "org"
            elif len(words) >= 2:
                etype = "person"
            else:
                etype = "concept"
            claims = [s for s in sents if phrase in s]
            found[phrase] = {"name": phrase, "type": etype, "salient_claims": claims}
        return {"entities": list(found.values())}

    @staticmethod
    def _proper_phrases(text: str) -> list[str]:
        out: list[str] = []
        seen: set[str] = set()
        for m in _PROPER_RE.finditer(text):
            phrase = m.group(0).strip()
            words = phrase.split()
            if len(words) == 1 and phrase in _NOISE:
                continue
            if len(words) == 1 and len(phrase) <= 2:
                continue
            # "However Babbage" -> "Babbage"
            while words and words[0] in _NOISE and len(words) > 1:
                words = words[1:]
            phrase = " ".join(words)
            if not phrase or phrase in seen:
                continue
            seen.add(phrase)
            out.append(phrase)
        return out

    def _task_summarize(self, payload: dict) -> dict:
        text = payload.get("text", "")
        entity_names = payload.get("entities", [])
        sents = sentences(text)
        title = entity_names[0] if entity_names else (sents[0] if sents else "Untitled")
        if len(title) > 80:
            title = title[:77].rstrip() + "..."
        summary = " ".join(sents[:3]) if sents else text.strip()
        freq: dict[str, int] = {}
        for tok in content_tokens(text):
            if len(tok) > 3:
                freq[tok] = freq.get(tok, 0) + 1
        tags = [t for t, _ in sorted(freq.items(), key=lambda kv: (-kv[1], kv[0]))[:3]]
        return {"title": title, "summary": summary, "tags": tags}

    def _task_judge(self, payload: dict) -> dict:
        a, b = payload.get("claim_a", ""), payload.get("claim_b", "")
        ta, tb = set(content_tokens(a)), set(content_tokens(b))
        shared = ta & tb
        if len(shared) < 2:
            return {"relation": "unrelated"}
        na, nb = numbers(a), numbers(b)
        if na and nb and na != nb:
            return {"relation": "contradict"}
        neg_a = bool(set(tokenize(a)) & _NEG)
        neg_b = bool(set(tokenize(b)) & _NEG)
        if neg_a != neg_b:
            return {"relation": "contradict"}
        return {"relation": "agree"}

    def _task_synthesize(self, payload: dict) -> dict:
        question = payload.get("question", "")
        pages = payload.get("pages", [])
        qtok = set(content_tokens(question))
        if not pages:
            return {
                "answer": "No grounded information was found in this wiki for that question.",
                "citations": [],
            }
        scored = []
        for p in pages:
            ptok = set(content_tokens(p.get("body", "")) + content_tokens(p.get("title", "")))
            scored.append((len(qtok & ptok), p))
        scored.sort(key=lambda x: -x[0])
        selected = [p for ov, p in scored if ov > 0][:3]
        if not selected:
            selected = [scored[0][1]]
        parts: list[str] = []
        citations: list[dict] = []
        for i, p in enumerate(selected, start=1):
            sents = _candidate_sentences(p.get("body", "")) or [p.get("title", "")]
            best = max(sents, key=lambda s: len(qtok & set(content_tokens(s))), default=sents[0])
            quote = best.strip()
            parts.append(f"{quote} ^{i}")
            citations.append(
                {
                    "marker": str(i),
                    "page_id": p.get("page_id"),
                    "source_id": (p.get("sources") or [None])[0],
                    "quote": quote,
                }
            )
        return {"answer": " ".join(parts), "citations": citations}


class AnthropicLLM:  # pragma: no cover - production seam
    """Claude via the Anthropic SDK. Needs the ``[anthropic]`` extra and
    ``ANTHROPIC_API_KEY``.
    """

    def __init__(self, model: str = "claude-opus-4-8", max_tokens: int = 4096) -> None:
        import anthropic

        self.model = model
        self.max_tokens = max_tokens
        self._client = anthropic.Anthropic()

    def complete(self, system: str, messages: list[Msg], *, schema: dict | None = None) -> LLMResult:
        sys = (
            system
            + "\n\nRespond with a single valid JSON object only — no prose, no "
            "code fences. Ground every claim strictly in the provided input."
        )
        resp = self._client.messages.create(
            model=self.model,
            max_tokens=self.max_tokens,
            system=sys,
            thinking={"type": "adaptive"},
            messages=[{"role": m.role, "content": m.content} for m in messages],
        )
        text = "".join(b.text for b in resp.content if getattr(b, "type", "") == "text")
        return LLMResult(text=_extract_json(text), raw={"model": self.model})


def _candidate_sentences(body: str) -> list[str]:
    """Split a page body into sentences, skipping headings and bullet markers."""
    cleaned_lines: list[str] = []
    for line in body.splitlines():
        s = line.strip()
        if not s or s.startswith("#"):
            continue
        if s.startswith("- "):
            s = s[2:].strip()
        if s.startswith("**"):  # answer pages: "**A:** ..."
            s = re.sub(r"\*\*[^*]+\*\*", "", s).strip()
        if s:
            cleaned_lines.append(s)
    return sentences(" ".join(cleaned_lines))


def _extract_json(text: str) -> str:  # pragma: no cover - prod helper
    text = text.strip()
    if text.startswith("```"):
        text = text.split("```", 2)[1]
        if text.startswith("json"):
            text = text[4:]
    start, end = text.find("{"), text.rfind("}")
    return text[start : end + 1] if start != -1 and end != -1 else text
