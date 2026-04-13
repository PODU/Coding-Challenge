"""Reciprocal rank fusion.

A document scores ``sum(weight / (k + rank))`` over the lists it appears in
(ranks are 1-based). Only ranks matter, so cosine and BM25 scores can be
combined without normalising them first.
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class FusedItem:
    key: str
    score: float
    ranks: dict[str, int]  # channel name -> 1-based rank


def reciprocal_rank_fusion(
    ranked_lists: dict[str, list[str]],
    *,
    k: int = 60,
    weights: dict[str, float] | None = None,
) -> list[FusedItem]:
    """Merge ``{channel: [key, ...]}`` rankings into one list, best first."""
    weights = weights or {}
    scores: dict[str, float] = {}
    ranks: dict[str, dict[str, int]] = {}
    for channel, keys in ranked_lists.items():
        w = weights.get(channel, 1.0)
        for rank, key in enumerate(keys, start=1):
            scores[key] = scores.get(key, 0.0) + w / (k + rank)
            ranks.setdefault(key, {})[channel] = rank
    items = [FusedItem(key=key, score=score, ranks=ranks[key]) for key, score in scores.items()]
    items.sort(key=lambda it: (-it.score, it.key))
    return items
