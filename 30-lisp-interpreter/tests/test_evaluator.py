"""Evaluator tests: special forms, builtins, scoping."""

from __future__ import annotations

import pytest

from cclisp import run
from cclisp.builtins import standard_env
from cclisp.evaluator import evaluate, is_truthy, lisp_str
from cclisp.reader import parse
from cclisp.types import Lambda, Symbol


def ev(src: str, env=None):
    env = env or standard_env()
    return evaluate(parse(src), env)


# Arithmetic
def test_add():
    assert ev("(+ 1 2)") == 3


def test_add_variadic():
    assert ev("(+ 1 2 3 4)") == 10


def test_sub_chain():
    assert ev("(- 10 3 2)") == 5


def test_sub_unary_negation():
    assert ev("(- 5)") == -5


def test_mul():
    assert ev("(* 2 3 4)") == 24


def test_nested_arithmetic():
    assert ev("(+ 1 (* 2 3))") == 7


def test_division_float():
    assert ev("(/ 10 4)") == 2.5


def test_mod():
    assert ev("(mod 17 5)") == 2


def test_float_arithmetic():
    assert ev("(+ 1.5 2.5)") == 4.0


# Comparison
def test_lt_true():
    assert ev("(< 1 2)") is True


def test_le_true():
    assert ev("(<= 2 2)") is True


def test_lt_false():
    assert ev("(< 3 2)") is False


def test_chained_comparison():
    assert ev("(< 1 2 3 4)") is True
    assert ev("(< 1 2 2 4)") is False


def test_equality():
    assert ev("(= 2 2)") is True
    assert ev("(= 2 3)") is False


# if
def test_if_true_branch():
    assert ev("(if (< 1 2) 10 20)") == 10


def test_if_false_branch():
    assert ev("(if (< 2 1) 10 20)") == 20


def test_if_missing_else_is_nil():
    assert ev("(if (< 2 1) 10)") is False


def test_if_lazy_branches():
    # The unused branch must not be evaluated (else it would divide by zero).
    assert ev("(if t 1 (/ 1 0))") == 1


# define / defun / lambda / set! / let / begin / quote
def test_define_and_lookup():
    env = standard_env()
    assert ev("(define x 5)", env) == "x"
    assert ev("x", env) == 5


def test_defun_and_call():
    env = standard_env()
    ev("(defun doublen (n) (* n 2))", env)
    assert ev("(doublen 21)", env) == 42


def test_lambda_is_first_class():
    val = ev("((lambda (x y) (+ x y)) 3 4)")
    assert val == 7


def test_lambda_value_type():
    assert isinstance(ev("(lambda (x) x)"), Lambda)


def test_set_mutates_existing():
    env = standard_env()
    ev("(define x 1)", env)
    ev("(set! x 99)", env)
    assert ev("x", env) == 99


def test_begin_returns_last():
    assert ev("(begin 1 2 3)") == 3


def test_let_local_bindings():
    assert ev("(let ((a 1) (b 2)) (+ a b))") == 3


def test_let_does_not_leak():
    env = standard_env()
    ev("(let ((tmp 5)) tmp)", env)
    with pytest.raises(Exception):
        ev("tmp", env)


def test_quote():
    assert ev("(quote (1 2 3))") == [1, 2, 3]
    assert ev("'(a b c)") == ["a", "b", "c"]


def test_multi_expression_body():
    env = standard_env()
    ev("(defun f (x) (define y (* x x)) (+ y 1))", env)
    assert ev("(f 4)", env) == 17


# Scoping & closures
def test_inner_param_shadows_global():
    env = standard_env()
    ev("(define n 100)", env)
    ev("(defun f (n) (* n 2))", env)
    assert ev("(f 3)", env) == 6
    assert ev("n", env) == 100


def test_lookup_falls_through_to_global():
    env = standard_env()
    ev("(define g 7)", env)
    ev("(defun add-g (x) (+ x g))", env)
    assert ev("(add-g 3)", env) == 10


def test_closure_captures_defining_scope():
    env = standard_env()
    ev("(defun adder (n) (lambda (x) (+ x n)))", env)
    ev("(define add5 (adder 5))", env)
    ev("(define add10 (adder 10))", env)
    assert ev("(add5 100)", env) == 105
    assert ev("(add10 100)", env) == 110


def test_counter_closure_mutable_state():
    env = standard_env()
    src = """
    (defun make-counter ()
      (let ((count 0))
        (lambda ()
          (set! count (+ count 1))
          count)))
    """
    run(src, env)
    ev("(define c (make-counter))", env)
    assert ev("(c)", env) == 1
    assert ev("(c)", env) == 2
    assert ev("(c)", env) == 3


# Recursion
def test_factorial():
    env = standard_env()
    ev("(defun fact (n) (if (<= n 1) 1 (* n (fact (- n 1)))))", env)
    assert ev("(fact 10)", env) == 3628800


def test_fibonacci():
    env = standard_env()
    ev("(defun fib (n) (if (< n 2) n (+ (fib (- n 1)) (fib (- n 2)))))", env)
    assert ev("(fib 10)", env) == 55


def test_deep_tail_recursion():
    # Tail-recursive countdown should not blow the Python stack thanks to TCO.
    env = standard_env()
    ev("(defun loop (n) (if (= n 0) 'done (loop (- n 1))))", env)
    assert ev("(loop 100000)", env) == "done"


# Logic & predicates
def test_and_short_circuit():
    assert ev("(and t t 5)") == 5
    assert ev("(and t nil (/ 1 0))") is False


def test_or_short_circuit():
    assert ev("(or nil 7)") == 7
    assert ev("(or nil nil)") is False


def test_not():
    assert ev("(not nil)") is True
    assert ev("(not t)") is False


def test_predicates():
    assert ev("(number? 3)") is True
    assert ev("(string? \"x\")") is True
    assert ev("(symbol? 'a)") is True
    assert ev("(list? (list 1 2))") is True
    assert ev("(procedure? +)") is True
    assert ev("(null? nil)") is True
    assert ev("(null? (list))") is True


# List builtins
def test_list_ops():
    assert ev("(list 1 2 3)") == [1, 2, 3]
    assert ev("(car (list 1 2 3))") == 1
    assert ev("(cdr (list 1 2 3))") == [2, 3]
    assert ev("(cons 0 (list 1 2))") == [0, 1, 2]
    assert ev("(length (list 1 2 3))") == 3
    assert ev("(append (list 1 2) (list 3 4))") == [1, 2, 3, 4]


# Truthiness
def test_truthiness_zero_is_true():
    assert is_truthy(0) is True
    assert ev("(if 0 'yes 'no)") == "yes"


def test_truthiness_nil_and_empty_list_false():
    assert is_truthy(False) is False
    assert is_truthy([]) is False


# format
def test_format_to_string():
    assert ev('(format nil "n = ~A~%" 5)') == "n = 5\n"


def test_format_directives():
    assert ev('(format nil "~A and ~S" "x" "x")') == 'x and "x"'
    assert ev('(format nil "100~~")') == "100~"


def test_format_to_stdout(capsys):
    ev('(format t "hi ~A~%" 42)')
    assert capsys.readouterr().out == "hi 42\n"


# Value formatting
def test_lisp_str():
    assert lisp_str(True) == "t"
    assert lisp_str(False) == "nil"
    assert lisp_str([1, 2, 3]) == "(1 2 3)"
    assert lisp_str("hi") == "hi"
    assert lisp_str("hi", repr_strings=True) == '"hi"'
    assert lisp_str(Symbol("foo")) == "foo"


# Equality (bool/number must not be conflated the way Python's True == 1 is)
def test_equal_number_not_boolean():
    assert ev("(equal? 1 t)") is False
    assert ev("(equal? 0 nil)") is False
    assert ev("(eq? 1 t)") is False
    assert ev("(eq? 0 nil)") is False


def test_equal_same_values():
    assert ev("(equal? 1 1)") is True
    assert ev("(equal? 1 1.0)") is True  # structural numeric equality
    assert ev('(equal? "x" "x")') is True
    assert ev("(equal? t t)") is True
    assert ev("(equal? nil nil)") is True


def test_equal_recurses_into_lists():
    assert ev("(equal? (list 1 2 3) (list 1 2 3))") is True
    assert ev("(equal? (list 1 t) (list 1 0))") is False
    assert ev("(equal? (list 1 2) (list 1 2 3))") is False


def test_eq_requires_same_kind():
    assert ev("(eq? 1 1.0)") is False  # int vs float are distinct under eq?
    assert ev("(eq? (quote a) (quote a))") is True
