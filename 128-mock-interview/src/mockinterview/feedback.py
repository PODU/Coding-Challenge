"""Turn an interview transcript into a STAR feedback report.

Questions that were skipped or never answered are left out rather than scored.
"""

from __future__ import annotations

from .llm import FeedbackClient, FeedbackError

# Stems shared by nearly every behavioral question - useless for telling
# questions apart, so excluded from the fuzzy match.
_COMMON_WORDS = {
    "tell", "about", "time", "describe", "example", "give", "what", "when",
    "situation", "where", "your", "have", "been", "that", "this", "with",
    "would", "could", "did", "how", "was", "were", "you",
}

PROMPT_HEADER = """You are an expert interview coach. Analyze the following mock \
behavioral interview transcript. For EACH answered question, evaluate the candidate's \
answer against the STAR framework:
- Situation: did they set the scene with concrete context?
- Task: did they state their specific responsibility or goal?
- Action: did they describe the specific actions THEY took?
- Result: did they give a concrete outcome (ideally measurable)?

For each answer report which STAR elements are present, a score from 1 (poor) to 5 \
(excellent), and ONE specific, actionable improvement. Also write a short overall \
strengths assessment (2-3 sentences). Judge only what is in the transcript; do not \
invent details. Base scores on structure, specificity, ownership ("I" vs "we") and \
measurable results.
"""


def pair_questions_with_answers(
    questions: list[dict], transcript: list[dict], skipped_ids: list[str] | None = None
) -> list[dict]:
    """Match each planned question to the candidate speech that followed it.

    The avatar paraphrases, so an interviewer turn counts as asking a question
    when it shares at least half of the question's distinctive words.
    """
    skipped = set(skipped_ids or [])
    pairs: list[dict] = []
    turns = [t for t in transcript if t.get("content", "").strip()]

    for q in questions:
        if q["id"] in skipped:
            continue
        raw_words = {w.strip("?,.!") for w in q["text"].lower().split()}
        q_words = {w for w in raw_words if len(w) > 3 and w not in _COMMON_WORDS}
        if not q_words:
            q_words = {w for w in raw_words if len(w) > 3}
        asked_at = None
        for i, t in enumerate(turns):
            if t.get("role") != "user":
                t_words = {w.strip("?,.!") for w in t["content"].lower().split()}
                if q_words and len(q_words & t_words) / len(q_words) >= 0.5:
                    asked_at = i
                    break
        if asked_at is None:
            continue
        answer_parts = _answer_until_next_question(turns, asked_at)
        answer = " ".join(answer_parts).strip()
        if len(answer.split()) < 5:  # e.g. just "skip"
            continue
        pairs.append({"question_id": q["id"], "question": q["text"], "answer": answer})
    return pairs


def _answer_until_next_question(turns: list[dict], asked_at: int) -> list[str]:
    parts: list[str] = []
    for t in turns[asked_at + 1 :]:
        if t.get("role") == "user":
            parts.append(t["content"])
        elif t.get("content", "").rstrip().endswith("?") and parts:
            break  # interviewer moved on to the next question
    return parts


def build_feedback_prompt(pairs: list[dict], role: str, seniority: str) -> str:
    sections = [PROMPT_HEADER, f"Target position: {seniority} {role}\n"]
    for p in pairs:
        sections.append(f"### QUESTION: {p['question']}\nANSWER: {p['answer']}\n")
    return "\n".join(sections)


def generate_report(
    client: FeedbackClient,
    questions: list[dict],
    transcript: list[dict],
    role: str,
    seniority: str,
    skipped_ids: list[str] | None = None,
) -> dict:
    pairs = pair_questions_with_answers(questions, transcript, skipped_ids)
    if not pairs:
        return {
            "answers": [],
            "overall_strengths": "",
            "average_score": None,
            "note": "No answered questions found in this session.",
        }
    raw = client.analyze(build_feedback_prompt(pairs, role, seniority))
    # Trust our pairing, not the model, on how many questions were answered.
    answers = raw.get("answers", [])[: len(pairs)]
    for ans, pair in zip(answers, pairs):
        ans["question"] = pair["question"]
        ans["question_id"] = pair["question_id"]
        if not isinstance(ans.get("score"), int) or not 1 <= ans["score"] <= 5:
            raise FeedbackError(f"Invalid score in feedback: {ans.get('score')!r}")
    avg = round(sum(a["score"] for a in answers) / len(answers), 2) if answers else None
    return {
        "answers": answers,
        "overall_strengths": raw.get("overall_strengths", ""),
        "average_score": avg,
    }
