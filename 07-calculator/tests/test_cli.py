"""CLI / integration tests exercising argument, pipe and REPL modes."""

from __future__ import annotations

import io

import pytest

from calc import cli
from calc.cli import main


def test_argument_mode_prints_result(capsys: pytest.CaptureFixture[str]) -> None:
    code = main(["(1 + 1) * 5"])
    out = capsys.readouterr().out.strip()
    assert code == cli.EXIT_OK
    assert out == "10"


def test_argument_mode_div_by_zero_exit_1(
    capsys: pytest.CaptureFixture[str],
) -> None:
    code = main(["1 / 0"])
    err = capsys.readouterr().err
    assert code == cli.EXIT_EVAL
    assert "division by zero" in err


def test_argument_mode_syntax_error_exit_2(
    capsys: pytest.CaptureFixture[str],
) -> None:
    code = main(["1 2"])
    err = capsys.readouterr().err
    assert code == cli.EXIT_SYNTAX
    assert "ParseError" in err
    assert "^" in err  # caret pointer present


def test_argument_mode_lex_error_exit_2(
    capsys: pytest.CaptureFixture[str],
) -> None:
    code = main(["1 + @"])
    assert code == cli.EXIT_SYNTAX
    assert "LexError" in capsys.readouterr().err


def test_empty_argument_is_usage_error(
    capsys: pytest.CaptureFixture[str],
) -> None:
    code = main(["   "])
    assert code == cli.EXIT_USAGE


def test_precision_flag(capsys: pytest.CaptureFixture[str]) -> None:
    code = main(["--precision", "3", "1 / 3"])
    out = capsys.readouterr().out.strip()
    assert code == cli.EXIT_OK
    assert out == "0.333"


def test_invalid_precision_is_usage_error() -> None:
    assert main(["--precision", "0", "1 + 1"]) == cli.EXIT_USAGE


def _run_repl_session(text: str, quiet: bool = True) -> tuple[int, str, str]:
    in_stream = io.StringIO(text)
    out = io.StringIO()
    err = io.StringIO()
    code = cli._run_repl(None, quiet, in_stream, out, err)
    return code, out.getvalue(), err.getvalue()


def test_pipe_mode_multiline() -> None:
    code, out, _ = _run_repl_session("1 + 1\n2 * 3\n")
    assert code == cli.EXIT_OK
    assert out.split() == ["2", "6"]


def test_repl_error_does_not_terminate_session() -> None:
    code, out, err = _run_repl_session("1 / 0\n2 + 2\n")
    # The session continued past the error and evaluated the next line.
    assert "4" in out.split()
    assert "division by zero" in err


def test_repl_blank_lines_skipped() -> None:
    code, out, _ = _run_repl_session("\n  \n7\n")
    assert out.split() == ["7"]


def test_repl_quit_ends_session() -> None:
    code, out, _ = _run_repl_session("1 + 1\nquit\n9 + 9\n")
    # Nothing after quit is evaluated.
    assert out.split() == ["2"]


def test_version_flag(capsys: pytest.CaptureFixture[str]) -> None:
    with pytest.raises(SystemExit) as exc:
        main(["--version"])
    assert exc.value.code == 0
    assert "calc" in capsys.readouterr().out
