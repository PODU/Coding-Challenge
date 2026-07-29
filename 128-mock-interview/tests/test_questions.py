import random

from mockinterview.questions import COMPETENCIES, QUESTION_BANK, select_questions


def test_bank_has_at_least_20_questions():
    assert len(QUESTION_BANK) >= 20


def test_every_question_tagged_with_valid_competency():
    for q in QUESTION_BANK:
        assert q.competency in COMPETENCIES
        assert q.id and q.text


def test_ids_unique():
    ids = [q.id for q in QUESTION_BANK]
    assert len(ids) == len(set(ids))


def test_selection_respects_count_and_competencies():
    picked = select_questions(["conflict", "failure"], 4, random.Random(1))
    assert len(picked) == 4
    assert {q.competency for q in picked} <= {"conflict", "failure"}


def test_selection_covers_multiple_competencies():
    picked = select_questions(["leadership", "teamwork"], 4, random.Random(2))
    assert {q.competency for q in picked} == {"leadership", "teamwork"}


def test_repeated_sessions_differ():
    sets = {
        tuple(q.id for q in select_questions(COMPETENCIES, 3, random.Random(seed)))
        for seed in range(10)
    }
    assert len(sets) > 1


def test_empty_competencies_falls_back_to_all():
    picked = select_questions([], 5, random.Random(3))
    assert len(picked) == 5


def test_never_returns_duplicates():
    picked = select_questions(COMPETENCIES, 8, random.Random(4))
    ids = [q.id for q in picked]
    assert len(ids) == len(set(ids))
