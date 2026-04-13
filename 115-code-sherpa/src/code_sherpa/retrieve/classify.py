"""Keyword-based query classifier.

The class adjusts retrieval: "where is X" questions get more results and lean
on text search, "how does X work" questions lean on vector search.
"""

from __future__ import annotations

import enum
import re
from dataclasses import dataclass


class QueryClass(str, enum.Enum):
    LOCATE = "locate"  # "where is...", "find..."
    EXPLAIN = "explain"  # "how does...", "why..."
    OVERVIEW = "overview"  # "what does this project do", "architecture"
    GENERIC = "generic"


@dataclass(frozen=True)
class Classification:
    query_class: QueryClass
    top_k: int
    vector_weight: float
    text_weight: float


_LOCATE = re.compile(r"\b(where|find|locate|which file|search for)\b", re.I)
_EXPLAIN = re.compile(r"\b(how|why|explain|walk me through|what happens|does)\b", re.I)
_OVERVIEW = re.compile(
    r"\b(overview|architecture|high[- ]level|structure|what does .* do|map of)\b", re.I
)


class QueryClassifier:
    def __init__(
        self,
        base_top_k: int = 8,
        vector_weight: float = 1.0,
        text_weight: float = 1.0,
    ) -> None:
        self.base_top_k = base_top_k
        self.vector_weight = vector_weight
        self.text_weight = text_weight

    def classify(self, query: str) -> Classification:
        q = query.strip()
        if _OVERVIEW.search(q):
            return Classification(
                QueryClass.OVERVIEW, self.base_top_k + 4, self.vector_weight, self.text_weight
            )
        if _LOCATE.search(q):
            # Usually looking for an identifier, which text search is better at.
            return Classification(
                QueryClass.LOCATE,
                self.base_top_k + 2,
                self.vector_weight,
                self.text_weight + 0.5,
            )
        if _EXPLAIN.search(q):
            return Classification(
                QueryClass.EXPLAIN,
                self.base_top_k,
                self.vector_weight + 0.5,
                self.text_weight,
            )
        return Classification(
            QueryClass.GENERIC, self.base_top_k, self.vector_weight, self.text_weight
        )
