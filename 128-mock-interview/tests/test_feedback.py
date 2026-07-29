import pytest

from mockinterview.feedback import (
    build_feedback_prompt,
    generate_report,
    pair_questions_with_answers,
)
from mockinterview.llm import FeedbackError, MockFeedbackClient

QUESTIONS = [
    {"id": "q1", "competency": "conflict", "text": "Tell me about a time you strongly disagreed with a colleague."},
    {"id": "q2", "competency": "failure", "text": "Tell me about a time you failed and what you learned."},
]

ANSWER_1 = (
    "When I was working on the billing system we had a disagreement about the schema. "
    "My role was tech lead so I had to decide. I talked to both sides and I decided to "
    "prototype both options. In the end the outcome was we picked the simpler one and shipped on time."
)

TRANSCRIPT = [
    {"role": "assistant", "content": "Welcome! Tell me about a time you strongly disagreed with a colleague."},
    {"role": "user", "content": ANSWER_1},
    {"role": "assistant", "content": "Thanks. Tell me about a time you failed and what you learned?"},
    {"role": "user", "content": "During a launch I missed a config flag. I owned it, fixed it, and eventually we improved our checklist."},
    {"role": "assistant", "content": "That's the end of the interview. Thank you."},
]


def test_pairing_matches_questions_to_answers():
    pairs = pair_questions_with_answers(QUESTIONS, TRANSCRIPT)
    assert len(pairs) == 2
    assert pairs[0]["question_id"] == "q1"
    assert ANSWER_1 in pairs[0]["answer"]


def test_skipped_questions_excluded():
    pairs = pair_questions_with_answers(QUESTIONS, TRANSCRIPT, skipped_ids=["q1"])
    assert [p["question_id"] for p in pairs] == ["q2"]


def test_unanswered_questions_excluded():
    transcript = TRANSCRIPT[:2]  # only q1 asked and answered
    pairs = pair_questions_with_answers(QUESTIONS, transcript)
    assert [p["question_id"] for p in pairs] == ["q1"]


def test_trivial_answers_excluded():
    transcript = [
        {"role": "assistant", "content": QUESTIONS[0]["text"]},
        {"role": "user", "content": "skip"},
    ]
    assert pair_questions_with_answers(QUESTIONS, transcript) == []


def test_prompt_contains_star_and_answers():
    pairs = pair_questions_with_answers(QUESTIONS, TRANSCRIPT)
    prompt = build_feedback_prompt(pairs, "Engineer", "senior")
    assert "STAR" in prompt
    assert "senior Engineer" in prompt
    assert ANSWER_1 in prompt


def test_generate_report_with_mock_client():
    report = generate_report(
        MockFeedbackClient(), QUESTIONS, TRANSCRIPT, "Engineer", "senior"
    )
    assert len(report["answers"]) == 2
    for ans in report["answers"]:
        assert 1 <= ans["score"] <= 5
        assert set(ans["star"]) == {"situation", "task", "action", "result"}
        assert ans["improvement"]
    assert report["average_score"] is not None
    assert report["overall_strengths"]


def test_empty_transcript_produces_note_not_scores():
    report = generate_report(MockFeedbackClient(), QUESTIONS, [], "Engineer", "senior")
    assert report["answers"] == []
    assert report["average_score"] is None
    assert "note" in report


def test_invalid_score_from_llm_raises():
    class BadClient:
        def analyze(self, prompt):
            return {"answers": [{"question": "x", "star": {}, "score": 9, "improvement": ""}], "overall_strengths": ""}

    with pytest.raises(FeedbackError):
        generate_report(BadClient(), QUESTIONS, TRANSCRIPT, "Engineer", "senior")
