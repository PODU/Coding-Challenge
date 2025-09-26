"""Integration tests for the CLI front end."""

from __future__ import annotations

import re

import pytest

from spellcorrect import cli


def run(capsys, argv, stdin: str | None = None, monkeypatch=None):
    if stdin is not None and monkeypatch is not None:
        import io

        monkeypatch.setattr("sys.stdin", io.StringIO(stdin))
    code = cli.main(argv)
    out = capsys.readouterr().out
    return code, out


def test_word_mode_known_word(capsys):
    code, out = run(capsys, ["--word", "the"])
    assert code == 0
    assert out == "the\tthe\n"


def test_word_mode_correction(capsys):
    code, out = run(capsys, ["--word", "speling"])
    assert code == 0
    assert out == "speling\tspelling\n"


def test_word_mode_preserves_original_case_in_left_column(capsys):
    code, out = run(capsys, ["--word", "Speling"])
    assert out == "Speling\tspelling\n"


def test_file_mode(capsys, tmp_path):
    p = tmp_path / "in.txt"
    p.write_text("the qick bron fox\n", encoding="utf-8")
    code, out = run(capsys, ["--file", str(p)])
    assert code == 0
    assert out == "the\tthe\nqick\tquick\nbron\tbrown\nfox\tfox\n"


def test_stdin_mode(capsys, monkeypatch):
    code, out = run(
        capsys, ["-"], stdin="the qick bron fox\n", monkeypatch=monkeypatch
    )
    assert code == 0
    assert out == "the\tthe\nqick\tquick\nbron\tbrown\nfox\tfox\n"


def test_check_only_exit_code_when_misspelled(capsys):
    code, out = run(capsys, ["--check-only", "--word", "speling"])
    assert code == 1
    assert out == "speling\tspelling\n"  # only the misspelled word is printed


def test_check_only_exit_zero_when_clean(capsys):
    code, out = run(capsys, ["--check-only", "--word", "the quick brown fox"])
    assert code == 0
    assert out == ""  # nothing misspelled -> no output


def test_metrics_line_format(capsys):
    code, out = run(capsys, ["--metrics", "--word", "the"])
    assert code == 0
    lines = out.splitlines()
    assert lines[0] == "the\tthe"
    assert re.fullmatch(
        r"Time: \d+\.\d+ms  \d+\.\d+ words per second", lines[1]
    ), lines[1]


def test_nonalpha_tokens_skipped(capsys):
    code, out = run(capsys, ["--word", "the 123 !!! fox"])
    assert out == "the\tthe\nfox\tfox\n"


def test_custom_dict(capsys, tmp_path):
    d = tmp_path / "dict.txt"
    d.write_text("hello hello world", encoding="utf-8")
    code, out = run(capsys, ["--dict", str(d), "--word", "helo"])
    assert out == "helo\thello\n"


def test_custom_csv(capsys, tmp_path):
    d = tmp_path / "freq.csv"
    d.write_text("word,count\nhello,500\nworld,300\n", encoding="utf-8")
    code, out = run(capsys, ["--csv", str(d), "--word", "helo"])
    assert out == "helo\thello\n"


def test_mode_is_required(capsys):
    with pytest.raises(SystemExit):
        cli.main([])
