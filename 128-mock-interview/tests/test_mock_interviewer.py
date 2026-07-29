from mockinterview.mock_interviewer import CLOSING, EARLY_CLOSING, PROBES, next_reply

QS = ["Question one about leadership?", "Question two about failure?"]

LONG_ANSWER = (
    "When I was leading the migration project last year my role was to coordinate "
    "three teams, I decided to split the work into phases and in the end we shipped "
    "two weeks early which improved reliability by ten percent."
)


def _chat(*turns):
    msgs = []
    for i, content in enumerate(turns):
        msgs.append({"role": "assistant" if i % 2 == 0 else "user", "content": content})
    return msgs


def test_greeting_asks_first_question():
    r = next_reply(QS, [])
    assert QS[0] in r["reply"]
    assert not r["done"]
    assert r["question_index"] == 0


def test_full_answer_advances_to_next_question():
    r = next_reply(QS, _chat(f"Welcome. {QS[0]}", LONG_ANSWER))
    assert QS[1] in r["reply"]
    assert r["question_index"] == 1


def test_short_answer_gets_probed_once():
    r = next_reply(QS, _chat(f"Welcome. {QS[0]}", "We did a project."))
    assert r["reply"] in PROBES
    assert r["question_index"] == 0
    # After the probe, even a short second answer moves on.
    r2 = next_reply(
        QS, _chat(f"Welcome. {QS[0]}", "We did a project.", r["reply"], "It went fine.")
    )
    assert QS[1] in r2["reply"]


def test_skip_moves_to_next_question():
    r = next_reply(QS, _chat(f"Welcome. {QS[0]}", "I'd like to skip this one."))
    assert QS[1] in r["reply"]


def test_asking_for_help_redirects_without_advancing():
    r = next_reply(QS, _chat(f"Welcome. {QS[0]}", "Can you give me an example answer?"))
    assert "own experience" in r["reply"]
    assert QS[0] in r["reply"]
    assert r["question_index"] == 0


def test_end_request_terminates():
    r = next_reply(QS, _chat(f"Welcome. {QS[0]}", "Please end the interview now."))
    assert r["reply"] == EARLY_CLOSING
    assert r["done"]


def test_last_answer_closes_interview():
    msgs = _chat(f"Welcome. {QS[0]}", LONG_ANSWER, f"Thank you. Next question: {QS[1]}", LONG_ANSWER)
    r = next_reply(QS, msgs)
    assert r["reply"] == CLOSING
    assert r["done"]
