"""Scorer tests: aggregation, clamping, threshold boundaries."""

from __future__ import annotations

import pytest

from aidetector.scorer import (
    LABEL_AI,
    LABEL_HUMAN,
    LABEL_POSSIBLE,
    aggregate,
    classify,
)
from aidetector.tokenize import build_document


@pytest.mark.parametrize(
    "score,label",
    [
        (0, LABEL_HUMAN),
        (29, LABEL_HUMAN),
        (30, LABEL_POSSIBLE),
        (59, LABEL_POSSIBLE),
        (60, LABEL_AI),
        (100, LABEL_AI),
    ],
)
def test_classify_boundaries(score: int, label: str) -> None:
    assert classify(score, 30, 60) == label


def test_custom_thresholds_respected() -> None:
    assert classify(40, 50, 80) == LABEL_HUMAN
    assert classify(50, 50, 80) == LABEL_POSSIBLE
    assert classify(80, 50, 80) == LABEL_AI


def test_aggregate_sums_capped_scores_and_clamps() -> None:
    text = (
        "In today's world, it is important to note that we must delve into the "
        "robust, innovative, and seamless realm. Studies show experts say this "
        "plays a crucial role. In conclusion, from start to finish, it is not "
        "only powerful but also transformative."
    )
    result = aggregate(build_document(text))
    assert 0 <= result.score <= 100
    assert result.score == int(
        round(min(100.0, sum(d.capped_score for d in result.detections)))
    )


def test_aggregate_empty_text_is_human() -> None:
    result = aggregate(build_document("The dog napped on the porch all afternoon."))
    assert result.score < 30
    assert result.label == LABEL_HUMAN


def test_result_stats_shape() -> None:
    result = aggregate(build_document("Hello world. A test."))
    assert set(result.stats) == {"characters", "words", "sentences", "syllables"}
