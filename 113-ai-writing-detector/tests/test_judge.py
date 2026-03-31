"""Optional-judge tests: only the offline MockJudge is exercised."""

from __future__ import annotations

import pytest

from aidetector import judge as judge_mod
from aidetector.cli import main
from aidetector.judge import Judge, JudgeOpinion, MockJudge, make_judge


def test_mockjudge_is_deterministic() -> None:
    mj = MockJudge()
    a = mj.assess("Some sample text to assess.")
    b = mj.assess("Some sample text to assess.")
    assert a == b
    assert 0.0 <= a.probability_ai <= 1.0
    assert a.source == "mock"


def test_mockjudge_satisfies_protocol() -> None:
    assert isinstance(MockJudge(), Judge)


def test_mockjudge_empty_text() -> None:
    op = MockJudge().assess("   ")
    assert op.probability_ai == 0.0


def test_make_judge_none_returns_none() -> None:
    assert make_judge("none") is None


def test_make_judge_mock_is_offline() -> None:
    assert isinstance(make_judge("mock"), MockJudge)


def test_default_cli_never_constructs_anthropic(monkeypatch: pytest.MonkeyPatch) -> None:
    """The default path must never instantiate the networked judge."""

    def _boom(*_args: object, **_kwargs: object) -> None:
        raise AssertionError("AnthropicJudge must not be constructed in tests")

    monkeypatch.setattr(judge_mod.AnthropicJudge, "__init__", _boom)
    code = main(["Just a plain sentence for analysis here."])
    assert code == 0  # ran fine without touching the Anthropic judge


def test_cli_mock_judge_path(capsys: pytest.CaptureFixture[str]) -> None:
    code = main(["--judge", "mock", "A sentence to be judged by the mock."])
    out = capsys.readouterr().out
    assert code == 0
    assert "Optional judge (mock)" in out


def test_judgeopinion_fields() -> None:
    op = JudgeOpinion(0.5, "why", "mock")
    assert op.probability_ai == 0.5 and op.rationale == "why" and op.source == "mock"
