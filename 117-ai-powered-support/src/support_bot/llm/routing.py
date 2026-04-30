"""Send simple questions to the cheap model and complex ones to the capable
one."""

from __future__ import annotations

from ..models import LLMResponse, Prompt
from ..text import raw_tokens
from .base import LLMClient

_COMPLEX_MARKERS = frozenset(
    """
    why how explain compare comparison difference differences versus vs design
    architecture architect tradeoff tradeoffs debug optimize optimise recommend
    recommendation best walkthrough walk strategy approach implement implementing
    """.split()
)


class QueryClassifier:
    """Labels a query 'simple' or 'complex' by length and keywords."""

    def __init__(self, *, word_limit: int = 18) -> None:
        self.word_limit = word_limit

    def classify(self, query: str) -> str:
        if len(query.split()) > self.word_limit:
            return "complex"
        if any(t in _COMPLEX_MARKERS for t in raw_tokens(query)):
            return "complex"
        return "simple"


class RoutingLLMClient:
    name = "routing"

    def __init__(
        self,
        delegate: LLMClient,
        *,
        simple_model: str,
        complex_model: str,
        classifier: QueryClassifier | None = None,
    ) -> None:
        self.delegate = delegate
        self.simple_model = simple_model
        self.complex_model = complex_model
        self.classifier = classifier or QueryClassifier()

    def select_model(self, prompt: Prompt) -> str:
        label = self.classifier.classify(prompt.last_user_text())
        return self.simple_model if label == "simple" else self.complex_model

    def complete(self, prompt: Prompt, *, model: str | None = None) -> LLMResponse:
        return self.delegate.complete(prompt, model=model or self.select_model(prompt))
