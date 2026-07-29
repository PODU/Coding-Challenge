"""Feedback clients: Claude with a JSON schema, or an offline heuristic mock."""

from __future__ import annotations

import json
from typing import Protocol


class FeedbackError(Exception):
    pass


FEEDBACK_SCHEMA = {
    "type": "object",
    "properties": {
        "answers": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "question": {"type": "string"},
                    "star": {
                        "type": "object",
                        "properties": {
                            "situation": {"type": "boolean"},
                            "task": {"type": "boolean"},
                            "action": {"type": "boolean"},
                            "result": {"type": "boolean"},
                        },
                        "required": ["situation", "task", "action", "result"],
                        "additionalProperties": False,
                    },
                    "score": {"type": "integer", "enum": [1, 2, 3, 4, 5]},
                    "improvement": {"type": "string"},
                },
                "required": ["question", "star", "score", "improvement"],
                "additionalProperties": False,
            },
        },
        "overall_strengths": {"type": "string"},
    },
    "required": ["answers", "overall_strengths"],
    "additionalProperties": False,
}


class FeedbackClient(Protocol):
    def analyze(self, prompt: str) -> dict: ...


class AnthropicFeedbackClient:
    def __init__(self, api_key: str, model: str = "claude-opus-4-8"):
        import anthropic

        self._client = anthropic.Anthropic(api_key=api_key)
        self._model = model

    def analyze(self, prompt: str) -> dict:
        import anthropic

        try:
            response = self._client.messages.create(
                model=self._model,
                max_tokens=8000,
                thinking={"type": "adaptive"},
                output_config={"format": {"type": "json_schema", "schema": FEEDBACK_SCHEMA}},
                messages=[{"role": "user", "content": prompt}],
            )
        except anthropic.AuthenticationError as exc:
            raise FeedbackError("Anthropic API key invalid.") from exc
        except anthropic.RateLimitError as exc:
            raise FeedbackError("Anthropic rate limit hit; try again shortly.") from exc
        except anthropic.APIStatusError as exc:
            raise FeedbackError(f"Anthropic API error {exc.status_code}.") from exc
        except anthropic.APIConnectionError as exc:
            raise FeedbackError("Could not reach the Anthropic API.") from exc

        if response.stop_reason == "refusal":
            raise FeedbackError("The model declined to analyze this transcript.")
        text = next((b.text for b in response.content if b.type == "text"), "")
        try:
            return json.loads(text)
        except json.JSONDecodeError as exc:
            raise FeedbackError("Model returned malformed feedback JSON.") from exc


class MockFeedbackClient:
    """Keyword-based STAR scoring so the app works without an API key.

    It parses the "### QUESTION:" blocks that feedback.build_feedback_prompt writes.
    """

    def analyze(self, prompt: str) -> dict:
        answers = []
        for chunk in prompt.split("### QUESTION: ")[1:]:
            lines = chunk.split("\n", 1)
            question = lines[0].strip()
            answer_text = lines[1] if len(lines) > 1 else ""
            low = answer_text.lower()
            star = {
                "situation": any(w in low for w in ("when", "while", "during", "at the time", "we had")),
                "task": any(w in low for w in ("my job", "my role", "responsible", "needed to", "had to", "goal")),
                "action": any(w in low for w in (" i ", "i decided", "i built", "i talked", "i led", "i wrote")),
                "result": any(w in low for w in ("result", "outcome", "in the end", "eventually", "improved", "shipped", "%")),
            }
            hits = sum(star.values())
            score = max(1, min(5, 1 + hits))
            missing = [k for k, v in star.items() if not v]
            improvement = (
                f"Make the {missing[0].capitalize()} explicit in your answer."
                if missing
                else "Tighten the answer: lead with the result, then compress the story."
            )
            answers.append(
                {"question": question, "star": star, "score": score, "improvement": improvement}
            )
        return {
            "answers": answers,
            "overall_strengths": "You answered every question and kept your stories concrete."
            if answers
            else "No answered questions to assess.",
        }
