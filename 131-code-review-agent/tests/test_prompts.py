from reviewagent.prompts import SYSTEM_PROMPT, build_review_prompt


def test_guidelines_are_a_parameter_not_hardcoded():
    prompt = build_review_prompt(
        pr_title="t", pr_body="b",
        guidelines="- Only rule: no tabs.",
        diffs=[("a.py", "+x = 1")],
    )
    assert "Only rule: no tabs." in prompt
    assert "no tabs" not in SYSTEM_PROMPT  # rules never leak into system


def test_diffs_rendered_per_file():
    prompt = build_review_prompt(
        pr_title="t", pr_body="",
        guidelines="g",
        diffs=[("a.py", "+1"), ("b.py", "+2")],
    )
    assert "### a.py" in prompt and "### b.py" in prompt
    assert prompt.count("```diff") == 2


def test_linter_section_only_when_present():
    without = build_review_prompt(pr_title="t", pr_body="", guidelines="g",
                                  diffs=[("a.py", "+1")])
    assert "Tool findings" not in without
    with_lint = build_review_prompt(pr_title="t", pr_body="", guidelines="g",
                                    diffs=[("a.py", "+1")],
                                    linter_findings="- a.py:1 [E501] too long")
    assert "Tool findings" in with_lint and "E501" in with_lint


def test_system_prompt_defines_structure_and_severities():
    for token in ("summary", "issues", "CRITICAL", "MAJOR", "MINOR", "verdict"):
        assert token in SYSTEM_PROMPT
