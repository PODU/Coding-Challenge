"""Interviewer personas and the system prompt sent to Anam."""

from __future__ import annotations

from dataclasses import dataclass

from . import config
from .questions import Question

CORE_RULES = """\
Rules you must always follow:
- Ask exactly ONE question at a time, then stop and wait for the candidate's answer.
- If an answer is vague or incomplete, ask ONE short follow-up probe (for example: \
"What was the outcome?", "What was your specific role?", "What did you do next?").
- NEVER suggest answers, complete the candidate's stories, or give examples of good answers.
- Stay strictly in your role as an interviewer. If the candidate asks for help, coaching, \
or the "right answer", politely decline and redirect them back to the question.
- If the candidate goes off-topic, briefly acknowledge and steer back to the current question.
- If the candidate asks to skip the current question, accept gracefully and move to the next one.
- If the candidate says they want to end the interview, thank them and close politely.
- Keep your own speech short: one or two sentences before each question, no long monologues.
- After the final question is answered, thank the candidate and say the interview is complete."""


@dataclass(frozen=True)
class PersonaStyle:
    id: str
    name: str
    description: str
    voice_id: str
    flavor: str


PERSONA_STYLES: dict[str, PersonaStyle] = {
    "supportive_coach": PersonaStyle(
        id="supportive_coach",
        name="Sam - Supportive Coach",
        description="Warm, encouraging interviewer who puts candidates at ease.",
        voice_id=config.DEFAULT_VOICE_ID,
        flavor=(
            "Your name is Sam. You are warm and encouraging. Use a friendly, "
            "relaxed tone. Briefly acknowledge answers positively ('Thanks, that's "
            "helpful') before moving on - but never evaluate or coach."
        ),
    ),
    "bar_raiser": PersonaStyle(
        id="bar_raiser",
        name="Alex - Formal Bar-Raiser",
        description="Formal, rigorous interviewer who probes hard on specifics.",
        voice_id=config.ALT_VOICE_ID,
        flavor=(
            "Your name is Alex. You are formal, concise and rigorous. Do not use "
            "small talk. Probe firmly on specifics, data and the candidate's "
            "individual contribution versus the team's."
        ),
    ),
}

DEFAULT_STYLE = "supportive_coach"


def build_system_prompt(
    role: str,
    seniority: str,
    questions: list[Question],
    style_id: str = DEFAULT_STYLE,
) -> str:
    style = PERSONA_STYLES.get(style_id, PERSONA_STYLES[DEFAULT_STYLE])
    numbered = "\n".join(f"{i + 1}. {q.text}" for i, q in enumerate(questions))
    return f"""You are a professional behavioral interviewer conducting a live spoken mock interview.

{style.flavor}

The candidate is interviewing for a {seniority} {role} position.

Start by greeting the candidate, introducing yourself, and explaining the format in \
two sentences: you will ask {len(questions)} behavioral questions, they should answer \
from their real experience, and they can say "skip" to move on. Then ask question 1.

Ask these questions IN ORDER, one at a time:
{numbered}

{CORE_RULES}

Your responses are spoken aloud, so use natural spoken language with no markdown, \
lists, or stage directions."""


def build_persona_config(
    role: str,
    seniority: str,
    questions: list[Question],
    style_id: str = DEFAULT_STYLE,
) -> dict:
    """Build the personaConfig body for Anam's session-token endpoint."""
    style = PERSONA_STYLES.get(style_id, PERSONA_STYLES[DEFAULT_STYLE])
    return {
        "name": style.name,
        "avatarId": config.DEFAULT_AVATAR_ID,
        "voiceId": style.voice_id,
        "llmId": config.DEFAULT_LLM_ID,
        "systemPrompt": build_system_prompt(role, seniority, questions, style_id),
    }


def list_styles() -> list[dict]:
    return [
        {"id": s.id, "name": s.name, "description": s.description}
        for s in PERSONA_STYLES.values()
    ]
