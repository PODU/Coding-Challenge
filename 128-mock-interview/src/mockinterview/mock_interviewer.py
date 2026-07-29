"""Text-chat interviewer used when no ANAM_API_KEY is set.

It follows the same rules as the Anam persona: one question at a time, probe a
short answer once, never coach, allow skipping.
"""

from __future__ import annotations

SKIP_WORDS = ("skip", "next question", "pass on this")
END_WORDS = ("end the interview", "stop the interview", "i'm done", "im done", "quit")
HELP_WORDS = ("help me", "what should i say", "give me an answer", "example answer", "right answer")

PROBES = [
    "What was the outcome?",
    "What was your specific role in that?",
    "Can you walk me through what you did next?",
]

GREETING = (
    "Hello, I'm your interviewer for today. I'll ask you {n} behavioral questions - "
    "answer from your real experience, and say 'skip' if you'd like to move on. "
    "Let's begin. {q}"
)
CLOSING = "That's the end of the interview. Thank you for your time, and well done for completing it."
EARLY_CLOSING = "Understood, we'll stop here. Thanks for your time today."
REDIRECT = "I can't help with answers during the interview - please answer from your own experience. {q}"


def next_reply(questions: list[str], messages: list[dict]) -> dict:
    """Return {"reply", "done", "question_index"} for the conversation so far."""
    if not messages:
        return {
            "reply": GREETING.format(n=len(questions), q=questions[0]),
            "done": False,
            "question_index": 0,
        }

    idx = _current_question_index(questions, messages)
    last_user = next(
        (m["content"] for m in reversed(messages) if m["role"] == "user"), ""
    )
    low = last_user.lower()

    if any(w in low for w in END_WORDS):
        return {"reply": EARLY_CLOSING, "done": True, "question_index": idx}

    if any(w in low for w in HELP_WORDS):
        return {
            "reply": REDIRECT.format(q=questions[idx]),
            "done": False,
            "question_index": idx,
        }

    if any(w in low for w in SKIP_WORDS):
        return _advance(questions, idx)

    # Probe a short answer once, then move on.
    if len(last_user.split()) < 25 and not _already_probed(questions, messages, idx):
        return {"reply": PROBES[idx % len(PROBES)], "done": False, "question_index": idx}

    return _advance(questions, idx)


def _advance(questions: list[str], idx: int) -> dict:
    if idx + 1 >= len(questions):
        return {"reply": CLOSING, "done": True, "question_index": idx}
    nxt = idx + 1
    return {
        "reply": f"Thank you. Next question: {questions[nxt]}",
        "done": False,
        "question_index": nxt,
    }


def _current_question_index(questions: list[str], messages: list[dict]) -> int:
    idx = 0
    for m in messages:
        if m["role"] != "assistant":
            continue
        for i, q in enumerate(questions):
            if q in m["content"]:
                idx = max(idx, i)
    return idx


def _already_probed(questions: list[str], messages: list[dict], idx: int) -> bool:
    asked_at = None
    for i, m in enumerate(messages):
        if m["role"] == "assistant" and questions[idx] in m["content"]:
            asked_at = i
    if asked_at is None:
        return False
    return any(
        m["role"] == "assistant" and any(p in m["content"] for p in PROBES)
        for m in messages[asked_at + 1 :]
    )
