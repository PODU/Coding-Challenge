"""End-to-end CLI + fixture tests."""

from __future__ import annotations

import io
import json
from pathlib import Path

import pytest

from aidetector.cli import main
from aidetector.scorer import aggregate
from aidetector.tokenize import build_document

FIXTURES = Path(__file__).parent / "fixtures"
HUMAN_FILES = sorted((FIXTURES / "human").glob("*.txt"))
AI_FILES = sorted((FIXTURES / "ai").glob("*.txt"))


@pytest.mark.parametrize("path", HUMAN_FILES, ids=lambda p: p.name)
def test_human_fixtures_score_below_30(path: Path) -> None:
    result = aggregate(build_document(path.read_text(encoding="utf-8")))
    assert result.score < 30, f"{path.name} scored {result.score}"
    assert result.label == "likely human"


@pytest.mark.parametrize("path", AI_FILES, ids=lambda p: p.name)
def test_ai_fixtures_score_at_least_60(path: Path) -> None:
    result = aggregate(build_document(path.read_text(encoding="utf-8")))
    assert result.score >= 60, f"{path.name} scored {result.score}"
    assert result.label == "likely AI"


def test_fixtures_exist() -> None:
    assert HUMAN_FILES, "expected human fixtures"
    assert AI_FILES, "expected AI fixtures"


# CLI surface


def test_cli_positional_text(capsys: pytest.CaptureFixture[str]) -> None:
    code = main(["The cat sat quietly on the warm windowsill all afternoon."])
    out = capsys.readouterr().out
    assert code == 0
    assert "AI-likelihood score" in out
    assert "LIKELY HUMAN" in out


def test_cli_empty_input_exit_1(capsys: pytest.CaptureFixture[str]) -> None:
    code = main(["   "])
    err = capsys.readouterr().err
    assert code == 1
    assert "No text to analyse" in err


def test_cli_bad_file_exit_2(capsys: pytest.CaptureFixture[str]) -> None:
    code = main(["--file", "does-not-exist-xyz.txt"])
    assert code == 2
    assert "cannot read file" in capsys.readouterr().err


def test_cli_json_shape(capsys: pytest.CaptureFixture[str]) -> None:
    code = main(["--format", "json", AI_FILES[0].read_text(encoding="utf-8")])
    assert code == 0
    data = json.loads(capsys.readouterr().out)
    assert set(data) >= {"score", "label", "stats", "detections", "thresholds"}
    assert isinstance(data["detections"], list)
    first = data["detections"][0]
    assert set(first) >= {"name", "step", "capped_score", "cap", "findings"}


def test_cli_file_stdin_positional_agree(
    tmp_path: Path,
    capsys: pytest.CaptureFixture[str],
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    text = AI_FILES[0].read_text(encoding="utf-8")

    main(["--format", "json", text])
    positional = json.loads(capsys.readouterr().out)["score"]

    f = tmp_path / "sample.txt"
    f.write_text(text, encoding="utf-8")
    main(["--format", "json", "--file", str(f)])
    from_file = json.loads(capsys.readouterr().out)["score"]

    # STDIN path: simulate a non-tty pipe.
    monkeypatch.setattr("sys.stdin", io.StringIO(text))
    monkeypatch.setattr("sys.stdin.isatty", lambda: False, raising=False)
    main(["--format", "json"])
    from_stdin = json.loads(capsys.readouterr().out)["score"]

    assert positional == from_file == from_stdin


def test_cli_custom_thresholds(capsys: pytest.CaptureFixture[str]) -> None:
    # Force everything into "likely AI" with an absurdly low AI threshold.
    code = main(
        ["--threshold-human", "0", "--threshold-ai", "0", "Plain ordinary sentence here today."]
    )
    assert code == 0
    assert "LIKELY AI" in capsys.readouterr().out


def test_cli_explain_lists_findings(capsys: pytest.CaptureFixture[str]) -> None:
    text = AI_FILES[0].read_text(encoding="utf-8")
    main(["--explain", text])
    out = capsys.readouterr().out
    assert "signature" in out.lower() or "Flesch-Kincaid" in out
