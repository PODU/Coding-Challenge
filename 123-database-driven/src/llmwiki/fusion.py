"""Reciprocal Rank Fusion of vector and full-text results.

Only ranks are used, because the two searches' scores aren't comparable.
"""

from __future__ import annotations

from .domain import Hit

RRF_K = 60


def reciprocal_rank_fusion(
    result_lists: list[list[Hit]], k: int = RRF_K, top: int | None = None
) -> list[Hit]:
    scores: dict[str, float] = {}
    vias: dict[str, set[str]] = {}
    for hits in result_lists:
        for rank, hit in enumerate(hits):
            scores[hit.page_id] = scores.get(hit.page_id, 0.0) + 1.0 / (k + rank + 1)
            vias.setdefault(hit.page_id, set()).add(hit.via)

    fused = [
        Hit(
            page_id=pid,
            score=score,
            via="both" if len(vias[pid]) > 1 else next(iter(vias[pid])),
        )
        for pid, score in scores.items()
    ]
    fused.sort(key=lambda h: (-h.score, h.page_id))
    return fused[:top] if top is not None else fused
