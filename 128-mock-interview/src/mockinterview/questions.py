"""Behavioral question bank, tagged by competency."""

from __future__ import annotations

import random
from dataclasses import dataclass

COMPETENCIES = ["leadership", "conflict", "failure", "teamwork", "delivery"]


@dataclass(frozen=True)
class Question:
    id: str
    competency: str
    text: str


QUESTION_BANK: list[Question] = [
    Question("lead-1", "leadership", "Tell me about a time you led a project or initiative without formal authority."),
    Question("lead-2", "leadership", "Describe a situation where you had to motivate a struggling teammate."),
    Question("lead-3", "leadership", "Tell me about a decision you made that was unpopular with your team."),
    Question("lead-4", "leadership", "Give me an example of a time you set a technical direction others followed."),
    Question("lead-5", "leadership", "Tell me about a time you mentored someone and what came of it."),
    Question("conf-1", "conflict", "Tell me about a time you strongly disagreed with a colleague. How was it resolved?"),
    Question("conf-2", "conflict", "Describe a situation where you had to push back on a decision from your manager."),
    Question("conf-3", "conflict", "Tell me about a time you received harsh criticism of your work."),
    Question("conf-4", "conflict", "Give me an example of resolving a conflict between two other people on your team."),
    Question("conf-5", "conflict", "Tell me about a time a stakeholder demanded something you believed was wrong."),
    Question("fail-1", "failure", "Tell me about a time you failed. What happened and what did you learn?"),
    Question("fail-2", "failure", "Describe a project that did not go as planned. What would you do differently?"),
    Question("fail-3", "failure", "Tell me about a mistake you made that affected other people."),
    Question("fail-4", "failure", "Give me an example of a goal you set and did not reach."),
    Question("fail-5", "failure", "Tell me about a time you shipped something with a serious bug. What did you do?"),
    Question("team-1", "teamwork", "Tell me about a time you had to work closely with someone whose style clashed with yours."),
    Question("team-2", "teamwork", "Describe a time you helped a teammate at cost to your own work."),
    Question("team-3", "teamwork", "Tell me about the best team you have been part of. What made it work?"),
    Question("team-4", "teamwork", "Give me an example of collaborating across teams or departments to get something done."),
    Question("team-5", "teamwork", "Tell me about a time you had to rely on someone else to succeed."),
    Question("deli-1", "delivery", "Tell me about a time you delivered under a tight deadline. How did you manage it?"),
    Question("deli-2", "delivery", "Describe a time you had to cut scope to ship. How did you decide what to cut?"),
    Question("deli-3", "delivery", "Tell me about the most complex project you have delivered end to end."),
    Question("deli-4", "delivery", "Give me an example of juggling multiple competing priorities."),
]


def get_bank() -> list[Question]:
    return list(QUESTION_BANK)


def select_questions(
    competencies: list[str],
    count: int = 3,
    rng: random.Random | None = None,
) -> list[Question]:
    """Pick `count` random questions, spread round-robin across competencies."""
    rng = rng or random.Random()
    comps = [c for c in competencies if c in COMPETENCIES] or list(COMPETENCIES)

    pools: dict[str, list[Question]] = {
        c: [q for q in QUESTION_BANK if q.competency == c] for c in comps
    }
    for pool in pools.values():
        rng.shuffle(pool)

    picked: list[Question] = []
    order = list(comps)
    rng.shuffle(order)
    i = 0
    while len(picked) < count and any(pools.values()):
        comp = order[i % len(order)]
        if pools[comp]:
            picked.append(pools[comp].pop())
        i += 1
        if i > 1000:  # all pools drained
            break
    rng.shuffle(picked)
    return picked[:count]
