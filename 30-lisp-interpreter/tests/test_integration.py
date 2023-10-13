"""End-to-end tests: file runner, REPL smoke test, challenge acceptance."""

from __future__ import annotations

import io
import os

import pytest

from cclisp import run
from cclisp.builtins import standard_env
from cclisp.cli import repl, run_file

EXAMPLES = os.path.join(os.path.dirname(__file__), "examples")


# File runner / golden output
def test_run_factorial_file(capsys):
    run_file(os.path.join(EXAMPLES, "factorial.lisp"))
    assert capsys.readouterr().out == "5! = 120\n10! = 3628800\n"


def test_run_fib_file(capsys):
    run_file(os.path.join(EXAMPLES, "fib.lisp"))
    assert capsys.readouterr().out == "fib(10) = 55\nfib(20) = 6765\n"


def test_run_counter_file(capsys):
    run_file(os.path.join(EXAMPLES, "counter.lisp"))
    assert capsys.readouterr().out == "1\n2\n3\n"


# Shared global env across forms
def test_later_forms_see_earlier_defuns():
    env = standard_env()
    result = run(
        "(defun sq (x) (* x x)) (defun sum-sq (a b) (+ (sq a) (sq b))) (sum-sq 3 4)",
        env,
    )
    assert result == 25


# REPL smoke test
def _drive_repl(lines):
    it = iter(lines)

    def fake_input(_prompt):
        try:
            return next(it)
        except StopIteration:
            raise EOFError

    out = io.StringIO()
    repl(input_fn=fake_input, out=out)
    return out.getvalue()


def test_repl_acceptance_examples():
    out = _drive_repl(
        [
            "(+ 1 2)",
            "(defun doublen (n) (* n 2))",
            "(doublen 21)",
        ]
    )
    lines = out.splitlines()
    assert lines == ["3", "doublen", "42"]


def test_repl_multiline_form():
    out = _drive_repl(["(+ 1", "   2", "   3)"])
    assert out.splitlines() == ["6"]


def test_repl_error_does_not_crash_loop():
    out = _drive_repl(["undefined-var", "(+ 1 2)"])
    lines = out.splitlines()
    assert "unbound symbol" in lines[0]
    assert lines[1] == "3"


def test_repl_blank_lines_ignored():
    out = _drive_repl(["", "   ", "(+ 2 2)"])
    assert out.splitlines() == ["4"]


def test_repl_string_output_quoted():
    out = _drive_repl(['(quote "hello")'])
    assert out.splitlines() == ['"hello"']
