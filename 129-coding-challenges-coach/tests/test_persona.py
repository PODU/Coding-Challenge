"""Coaching persona."""

from coach.persona import build_system_prompt


def test_persona_asks_about_developer():
    prompt = build_system_prompt()
    for topic in ("interests", "learning", "experience level"):
        assert topic in prompt
    assert "follow-up questions" in prompt


def test_persona_refuses_full_solutions():
    prompt = build_system_prompt()
    assert "NEVER provide a full solution" in prompt
    assert "hints" in prompt.lower()


def test_catalog_listed():
    prompt = build_system_prompt(catalog=["01-wc.md", "03-huffman.yaml"])
    assert "01-wc.md" in prompt and "03-huffman.yaml" in prompt


def test_memory_notes_injected_with_no_repeat_rule():
    prompt = build_system_prompt(memory_notes=["Prefers Go", "Advised: start with -c"])
    assert "Prefers Go" in prompt
    assert "do not repeat" in prompt
