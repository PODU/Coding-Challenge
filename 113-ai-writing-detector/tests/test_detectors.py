"""Detector tests."""

from __future__ import annotations

from aidetector.detectors.base import Detection, Finding
from aidetector.detectors.emphasis import EmphasisDetector
from aidetector.detectors.linguistic import LinguisticDetector
from aidetector.detectors.signature import SignatureDetector
from aidetector.detectors.structural import StructuralDetector
from aidetector.detectors.vague import VagueLanguageDetector
from aidetector.tokenize import build_document


def _doc(text: str):
    return build_document(text)


def _by_pattern(det: Detection, pattern: str) -> list[Finding]:
    return [f for f in det.findings if f.pattern == pattern]


def _weight(det: Detection, pattern: str) -> float:
    return sum(f.weight for f in _by_pattern(det, pattern))


# Signature words / phrases


def test_signature_detects_phrase_and_words_deduped() -> None:
    det = SignatureDetector().run(
        _doc("We delve into the robust, innovative realm. Delve delve robust.")
    )
    phrases = _by_pattern(det, "signature_phrase")
    words = _by_pattern(det, "signature_word")
    assert {f.evidence for f in phrases} == {"delve into"}
    # Distinct words only; repeated "delve"/"robust" counted once.
    assert {f.evidence for f in words} == {"delve", "robust", "innovative", "realm"}
    assert det.raw_score == 4.0 + 3 * 4


def test_signature_caps_at_20() -> None:
    text = (
        "delve robust innovative navigate leverage seamless holistic "
        "multifaceted underscore myriad plethora intricate"
    )
    det = SignatureDetector().run(_doc(text))
    assert det.raw_score > 20
    assert det.capped_score == 20.0


def test_signature_clean_text_scores_zero() -> None:
    det = SignatureDetector().run(_doc("I walked the dog and bought some milk."))
    assert det.raw_score == 0.0


# Structural patterns


def test_structural_all_four_patterns() -> None:
    text = (
        "It was fast, reliable, and scalable. In conclusion, we win. "
        "From apples to oranges. It is not only smart but also kind."
    )
    det = StructuralDetector().run(_doc(text))
    patterns = {f.pattern for f in det.findings}
    assert patterns == {
        "rule_of_three",
        "outline_conclusion",
        "false_range",
        "negative_parallelism",
    }


def test_structural_clean_text_triggers_none() -> None:
    det = StructuralDetector().run(
        _doc("I went to the store and then I came home for dinner.")
    )
    assert det.findings == []
    assert det.raw_score == 0.0


def test_structural_false_range_ignores_numeric() -> None:
    det = StructuralDetector().run(_doc("The temperature rose from three to ten degrees."))
    assert _by_pattern(det, "false_range") == []


# Vague language (per-category caps)


def test_vague_three_categories_each_capped_at_six() -> None:
    text = (
        "Studies show experts say research suggests it plays a crucial role "
        "in todays world throughout history many believe it is worth noting "
        "in the modern era."
    )
    det = VagueLanguageDetector().run(_doc(text))
    assert _weight(det, "vague_attribution") == 6.0
    assert _weight(det, "superficial_analysis") == 6.0
    assert _weight(det, "overgeneralization") == 6.0
    assert det.capped_score == 18.0


def test_vague_clean_text_scores_zero() -> None:
    det = VagueLanguageDetector().run(_doc("The cat sat on the warm windowsill."))
    assert det.raw_score == 0.0


# Excessive emphasis


def test_emphasis_all_categories() -> None:
    text = "This REVOLUTIONARY game-changer is very very extremely good. It is important, crucial, and vital."
    det = EmphasisDetector().run(_doc(text))
    patterns = {f.pattern for f in det.findings}
    assert "promotional" in patterns
    assert "undue_emphasis" in patterns
    assert "elegant_variation" in patterns


def test_emphasis_respects_category_cap() -> None:
    text = (
        "revolutionary cutting-edge state-of-the-art world-class groundbreaking "
        "unparalleled transformative"
    )
    det = EmphasisDetector().run(_doc(text))
    promo = _weight(det, "promotional")
    assert promo == 7.0  # many promo terms clamped to the 7-point category cap


def test_emphasis_clean_text_scores_zero() -> None:
    det = EmphasisDetector().run(_doc("We met at noon and talked about the weather."))
    assert det.raw_score == 0.0


# Linguistic factors


def test_low_sentence_variation_scores_more_than_high() -> None:
    uniform = (
        "The cat sat on the mat. The dog ran in the park. "
        "The bird flew to the tree. The fish swam in the pond."
    )
    bursty = (
        "Yes. I went downtown yesterday afternoon to buy a few groceries and "
        "ended up wandering for hours. Tired. The little market on the corner "
        "had everything I could possibly want and then some, honestly. Wow."
    )
    low = LinguisticDetector().run(_doc(uniform))
    high = LinguisticDetector().run(_doc(bursty))
    assert _weight(low, "sentence_variation") > _weight(high, "sentence_variation")


def test_passive_voice_factor() -> None:
    passive = (
        "The report was written by the committee. The decision was made last week. "
        "The results were analyzed carefully. The project was completed on time. "
        "The data was collected over several long months."
    )
    active = (
        "The committee wrote the report. They reached a decision last week. "
        "We studied the results carefully. The team finished the project early. "
        "I gathered the data myself over several long months."
    )
    p = LinguisticDetector().run(_doc(passive))
    a = LinguisticDetector().run(_doc(active))
    assert _weight(p, "passive_voice") > _weight(a, "passive_voice")


def test_transition_density_factor() -> None:
    heavy = (
        "However, the plan worked. Moreover, it scaled. Furthermore, it was cheap. "
        "Additionally, it was fast. Consequently, everyone was happy."
    )
    light = (
        "The plan worked and it scaled well. It was cheap and fast. "
        "Everyone on the team felt happy about the result that day."
    )
    h = LinguisticDetector().run(_doc(heavy))
    light_doc = LinguisticDetector().run(_doc(light))
    assert _weight(h, "transition_density") > _weight(light_doc, "transition_density")


def test_linguistic_emits_seven_factors() -> None:
    det = LinguisticDetector().run(_doc("A reasonably long sentence here. " * 6))
    factor_patterns = {
        "lexical_diversity",
        "sentence_variation",
        "passive_voice",
        "transition_density",
        "flesch_kincaid",
        "punctuation",
        "rare_word",
    }
    present = {f.pattern for f in det.findings}
    assert factor_patterns <= present


def test_short_text_low_confidence_and_downweight() -> None:
    short = LinguisticDetector().run(_doc("Short uniform text here today."))
    assert any(f.pattern == "low_confidence" for f in short.findings)
