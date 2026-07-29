from mockinterview.personas import (
    PERSONA_STYLES,
    build_persona_config,
    build_system_prompt,
    list_styles,
)
from mockinterview.questions import Question

QUESTIONS = [
    Question("a", "conflict", "Tell me about a disagreement."),
    Question("b", "failure", "Tell me about a failure."),
]


def test_prompt_contains_questions_in_order():
    prompt = build_system_prompt("Engineer", "senior", QUESTIONS)
    assert prompt.index("1. Tell me about a disagreement.") < prompt.index(
        "2. Tell me about a failure."
    )


def test_prompt_contains_behavior_rules():
    prompt = build_system_prompt("Engineer", "senior", QUESTIONS)
    assert "ONE question at a time" in prompt
    assert "NEVER suggest answers" in prompt
    assert "skip" in prompt
    assert "follow-up" in prompt


def test_prompt_includes_role_and_seniority():
    prompt = build_system_prompt("Data Scientist", "junior", QUESTIONS)
    assert "junior Data Scientist" in prompt


def test_styles_have_distinct_flavor():
    coach = build_system_prompt("E", "mid", QUESTIONS, "supportive_coach")
    bar = build_system_prompt("E", "mid", QUESTIONS, "bar_raiser")
    assert coach != bar
    assert "Sam" in coach
    assert "Alex" in bar


def test_unknown_style_falls_back_to_default():
    prompt = build_system_prompt("E", "mid", QUESTIONS, "nonsense")
    assert "Sam" in prompt


def test_persona_config_shape():
    cfg = build_persona_config("Engineer", "mid-level", QUESTIONS, "bar_raiser")
    assert set(cfg) == {"name", "avatarId", "voiceId", "llmId", "systemPrompt"}
    assert cfg["name"] == PERSONA_STYLES["bar_raiser"].name
    assert "behavioral" in cfg["systemPrompt"]


def test_list_styles():
    styles = list_styles()
    assert {s["id"] for s in styles} == set(PERSONA_STYLES)
