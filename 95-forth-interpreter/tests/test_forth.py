"""Deterministic input -> (final stack, stdout) tests for the Forth interpreter."""

from __future__ import annotations

import io
from pathlib import Path

import pytest

from forth_interpreter import (
    CompileError,
    ForthError,
    ForthExit,
    Interpreter,
    StackUnderflow,
    UnknownWord,
    run_file,
)

EXAMPLES = Path(__file__).resolve().parent.parent / "examples"


def run(source: str) -> Interpreter:
    """Run *source* on a fresh interpreter; return it for inspection."""
    interp = Interpreter(out=io.StringIO())
    interp.run(source)
    return interp


def out_of(interp: Interpreter) -> str:
    return interp.out.getvalue()  # type: ignore[attr-defined]


# Arithmetic
@pytest.mark.parametrize(
    "source, stack",
    [
        ("1 2 +", [3]),
        ("10 3 -", [7]),
        ("6 7 *", [42]),
        ("20 4 /", [5]),
        ("17 5 mod", [2]),
        ("-7 2 /", [-4]),  # floor division toward negative infinity
        ("-7 2 mod", [1]),
    ],
)
def test_arithmetic_stack(source: str, stack: list[int]) -> None:
    assert run(source).stack == stack


def test_arithmetic_with_print() -> None:
    interp = run("1 2 + 4 * .")
    assert interp.stack == []
    assert out_of(interp) == "12 "


# Stack operations
@pytest.mark.parametrize(
    "source, stack",
    [
        ("1 dup", [1, 1]),
        ("1 2 drop", [1]),
        ("1 2 swap", [2, 1]),
        ("1 2 over", [1, 2, 1]),
        ("1 2 3 rot", [2, 3, 1]),
    ],
)
def test_stack_ops(source: str, stack: list[int]) -> None:
    assert run(source).stack == stack


# Output
@pytest.mark.parametrize(
    "source, stdout",
    [
        ("65 emit", "A"),
        ('." Hello, World!"', "Hello, World!"),
        ("cr", "\n"),
        ("1 . 2 .", "1 2 "),
    ],
)
def test_output(source: str, stdout: str) -> None:
    assert out_of(run(source)) == stdout


# Word definitions & comments
def test_definition_square() -> None:
    assert run(": square dup * ;  5 square").stack == [25]


def test_nested_definitions() -> None:
    assert run(": inc 1 + ;  : inc2 inc inc ;  0 inc2").stack == [2]


def test_comment_ignored() -> None:
    assert run("( this is ignored ) 1 2 +").stack == [3]


def test_definition_prints() -> None:
    assert out_of(run(': greet ." hi" ;  greet')) == "hi"


def test_redefinition_shadows() -> None:
    interp = run(": x 1 ;  : x 2 ;  x")
    assert interp.stack == [2]


def test_case_insensitive() -> None:
    assert run("1 2 DUP Drop +").stack == [3]


# Conditionals IF / ELSE / THEN
def test_if_true_branch() -> None:
    assert out_of(run(': t? 0 > if ." pos" then ;  5 t?')) == "pos"


def test_if_false_no_else() -> None:
    assert out_of(run(': t? 0 > if ." pos" then ;  -3 t?')) == ""


def test_if_else_true() -> None:
    assert run(": sign 0 > if 1 else -1 then ;  7 sign").stack == [1]


def test_if_else_false() -> None:
    assert run(": sign 0 > if 1 else -1 then ;  -7 sign").stack == [-1]


# Comparison & logic
@pytest.mark.parametrize(
    "source, stack",
    [
        ("3 4 <", [-1]),
        ("3 4 >", [0]),
        ("5 5 =", [-1]),
        ("5 6 <>", [-1]),
        ("-1 0 and", [0]),
        ("-1 -1 and", [-1]),
        ("0 -1 or", [-1]),
        ("-1 invert", [0]),
        ("0 invert", [-1]),
    ],
)
def test_comparison_logic(source: str, stack: list[int]) -> None:
    assert run(source).stack == stack


# Loops
def test_loop_prints_index() -> None:
    assert out_of(run(": tens 10 0 do I . loop ;  tens")) == "0 1 2 3 4 5 6 7 8 9 "


def test_loop_accumulates() -> None:
    assert run(": sum 0 5 0 do I + loop ;  sum").stack == [10]


# Nested control flow
def test_nested_if_in_do() -> None:
    src = ': fizz 5 0 do I 2 mod 0 = if ." e" else ." o" then loop ;  fizz'
    assert out_of(run(src)) == "eoeoe"


def test_nested_loops() -> None:
    # An inner loop runs entirely for each outer index; I is the innermost.
    src = ": g 2 0 do 2 0 do I . loop loop ;  g"
    assert out_of(run(src)) == "0 1 0 1 "


# Integration programs
def test_fibonacci_example() -> None:
    interp = Interpreter(out=io.StringIO())
    assert run_file(str(EXAMPLES / "fibonacci.fth"), interp) == 0
    assert out_of(interp) == "0 1 1 2 3 5 8 13 21 34 \n"


def test_fizzbuzz_example() -> None:
    interp = Interpreter(out=io.StringIO())
    assert run_file(str(EXAMPLES / "fizzbuzz.fth"), interp) == 0
    assert out_of(interp).strip() == (
        "1 2 Fizz 4 Buzz Fizz 7 8 Fizz Buzz 11 Fizz 13 14 FizzBuzz"
    )


# Error cases
def test_stack_underflow() -> None:
    with pytest.raises(StackUnderflow):
        run("+")


def test_stack_underflow_message() -> None:
    with pytest.raises(StackUnderflow) as exc:
        run("1 +")
    assert str(exc.value) == "+: stack underflow"


def test_unknown_word() -> None:
    with pytest.raises(UnknownWord) as exc:
        run("foo")
    assert str(exc.value) == "foo?"


def test_non_integer_is_unknown_word() -> None:
    with pytest.raises(UnknownWord):
        run("3.5")


def test_division_by_zero() -> None:
    with pytest.raises(ForthError) as exc:
        run("1 0 /")
    assert "division by zero" in str(exc.value)


def test_mod_by_zero() -> None:
    with pytest.raises(ForthError):
        run("1 0 mod")


def test_unterminated_definition() -> None:
    with pytest.raises(CompileError) as exc:
        run(": x 1 +")
    assert "definition" in str(exc.value)


def test_unbalanced_if() -> None:
    with pytest.raises(CompileError) as exc:
        run('5 if ." x"')
    assert "IF/THEN" in str(exc.value)


def test_stray_then() -> None:
    with pytest.raises(CompileError):
        run("then")


def test_unbalanced_do() -> None:
    with pytest.raises(CompileError) as exc:
        run("5 0 do I .")
    assert "DO/LOOP" in str(exc.value)


def test_stray_semicolon() -> None:
    with pytest.raises(CompileError) as exc:
        run(";")
    assert "definition" in str(exc.value)


def test_unterminated_string() -> None:
    with pytest.raises(CompileError) as exc:
        run('." no close')
    assert "unterminated" in str(exc.value)


def test_unterminated_comment() -> None:
    with pytest.raises(CompileError):
        run("( no close")


def test_emit_out_of_range() -> None:
    with pytest.raises(ForthError):
        run("-1 emit")


# System / misc
def test_bye_raises_exit() -> None:
    with pytest.raises(ForthExit):
        run("bye")


def test_blank_and_comment_only_are_noops() -> None:
    interp = run("   \n  ( only a comment )  \n  ")
    assert interp.stack == []
    assert out_of(interp) == ""


def test_state_persists_across_runs() -> None:
    interp = Interpreter(out=io.StringIO())
    interp.run(": double 2 * ;")
    interp.run("21 double")
    assert interp.stack == [42]


def test_stack_summary() -> None:
    interp = run("1 2 3")
    assert interp.stack_summary() == "<3> 1 2 3"
    assert Interpreter().stack_summary() == "<0>"
