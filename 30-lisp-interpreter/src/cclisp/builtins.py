"""The standard environment: arithmetic, comparison, list and I/O builtins."""

from __future__ import annotations

import functools
import operator
import sys
from typing import Callable

from .environment import Environment
from .evaluator import is_truthy, lisp_str
from .types import (
    Lambda,
    LispArityError,
    LispError,
    LispTypeError,
    Symbol,
)

Number = (int, float)


def _named(name: str, fn: Callable) -> Callable:
    """Record the Lisp name of a builtin so it prints as ``#<builtin name>``."""
    fn.lisp_name = name  # type: ignore[attr-defined]
    return fn


def _check_numbers(name: str, args: tuple) -> None:
    for a in args:
        if isinstance(a, bool) or not isinstance(a, Number):
            raise LispTypeError(f"{name}: expected a number, got {lisp_str(a)}")


def _add(*args):
    _check_numbers("+", args)
    return sum(args) if args else 0


def _mul(*args):
    _check_numbers("*", args)
    return functools.reduce(operator.mul, args, 1)


def _sub(*args):
    _check_numbers("-", args)
    if not args:
        raise LispArityError("- expects at least 1 argument")
    if len(args) == 1:
        return -args[0]
    return functools.reduce(operator.sub, args)


def _div(*args):
    _check_numbers("/", args)
    if not args:
        raise LispArityError("/ expects at least 1 argument")
    try:
        if len(args) == 1:
            return operator.truediv(1, args[0])
        return functools.reduce(operator.truediv, args)
    except ZeroDivisionError:
        raise LispError("division by zero") from None


def _abs(x):
    _check_numbers("abs", (x,))
    return abs(x)


def _min(*args):
    _check_numbers("min", args)
    if not args:
        raise LispArityError("min expects at least 1 argument")
    return min(args)


def _max(*args):
    _check_numbers("max", args)
    if not args:
        raise LispArityError("max expects at least 1 argument")
    return max(args)


def _mod(a, b):
    _check_numbers("mod", (a, b))
    try:
        return a % b
    except ZeroDivisionError:
        raise LispError("division by zero") from None


def _comparison(name: str, op: Callable[[object, object], bool]) -> Callable:
    def compare(*args):
        _check_numbers(name, args)
        if len(args) < 2:
            raise LispArityError(f"{name} expects at least 2 arguments")
        return all(op(args[i], args[i + 1]) for i in range(len(args) - 1))

    return _named(name, compare)


def _car(lst):
    if not isinstance(lst, list) or not lst:
        raise LispTypeError(f"car: expected a non-empty list, got {lisp_str(lst)}")
    return lst[0]


def _cdr(lst):
    if not isinstance(lst, list) or not lst:
        raise LispTypeError(f"cdr: expected a non-empty list, got {lisp_str(lst)}")
    return lst[1:]


def _cons(x, lst):
    if isinstance(lst, list):
        return [x, *lst]
    if lst is False:  # cons onto nil
        return [x]
    return [x, lst]


def _append(*lists):
    out: list = []
    for lst in lists:
        if lst is False:
            continue
        if not isinstance(lst, list):
            raise LispTypeError(f"append: expected a list, got {lisp_str(lst)}")
        out.extend(lst)
    return out


def _length(lst):
    if lst is False:
        return 0
    if not isinstance(lst, (list, str)):
        raise LispTypeError(f"length: expected a list, got {lisp_str(lst)}")
    return len(lst)


def _is_null(x):
    return x is False or (isinstance(x, list) and len(x) == 0)


def _eq(a, b) -> bool:
    return type(a) is type(b) and a == b


def _equal(a, b) -> bool:
    """Structural equality. Python says ``True == 1``; Lisp's t is not 1, so
    booleans only ever equal other booleans. 1 and 1.0 are still equal."""
    if isinstance(a, list) and isinstance(b, list):
        return len(a) == len(b) and all(_equal(x, y) for x, y in zip(a, b))
    return isinstance(a, bool) == isinstance(b, bool) and a == b


def _format(destination, control: str, *args) -> object:
    """``(format dest control args...)``, roughly as in Common Lisp.

    ``dest`` is ``t`` to print (returning nil) or ``nil`` to return the string.
    Supports ``~A``, ``~S``, ``~%`` and ``~~``; other directives are copied
    through unchanged.
    """
    if not isinstance(control, str):
        raise LispTypeError("format: control string must be a string")
    out: list[str] = []
    arg_iter = iter(args)
    i = 0
    n = len(control)
    while i < n:
        ch = control[i]
        if ch != "~":
            out.append(ch)
            i += 1
            continue
        if i + 1 >= n:
            out.append("~")
            break
        directive = control[i + 1].upper()
        if directive == "A":
            out.append(lisp_str(_next_arg(arg_iter), repr_strings=False))
        elif directive == "S":
            out.append(lisp_str(_next_arg(arg_iter), repr_strings=True))
        elif directive == "%":
            out.append("\n")
        elif directive == "~":
            out.append("~")
        else:
            out.append(control[i : i + 2])
        i += 2

    text = "".join(out)
    if destination is True:
        sys.stdout.write(text)
        return False
    if destination is False:
        return text
    raise LispTypeError("format: destination must be t or nil")


def _next_arg(arg_iter) -> object:
    try:
        return next(arg_iter)
    except StopIteration:
        raise LispError("format: not enough arguments for control string") from None


def _print(value):
    sys.stdout.write(lisp_str(value, repr_strings=True) + "\n")
    return value


def standard_env() -> Environment:
    """Build the global environment pre-populated with builtins."""
    env = Environment()
    env.update(
        {
            # arithmetic
            Symbol("+"): _named("+", _add),
            Symbol("-"): _named("-", _sub),
            Symbol("*"): _named("*", _mul),
            Symbol("/"): _named("/", _div),
            Symbol("mod"): _named("mod", _mod),
            Symbol("modulo"): _named("modulo", _mod),
            # comparison
            Symbol("<"): _comparison("<", operator.lt),
            Symbol("<="): _comparison("<=", operator.le),
            Symbol(">"): _comparison(">", operator.gt),
            Symbol(">="): _comparison(">=", operator.ge),
            Symbol("="): _comparison("=", operator.eq),
            # logic
            Symbol("not"): _named("not", lambda x: not is_truthy(x)),
            # lists
            Symbol("list"): _named("list", lambda *xs: list(xs)),
            Symbol("cons"): _named("cons", _cons),
            Symbol("car"): _named("car", _car),
            Symbol("cdr"): _named("cdr", _cdr),
            Symbol("first"): _named("first", _car),
            Symbol("rest"): _named("rest", _cdr),
            Symbol("append"): _named("append", _append),
            Symbol("length"): _named("length", _length),
            Symbol("null?"): _named("null?", _is_null),
            # predicates
            Symbol("number?"): _named(
                "number?",
                lambda x: isinstance(x, Number) and not isinstance(x, bool),
            ),
            Symbol("symbol?"): _named("symbol?", lambda x: isinstance(x, Symbol)),
            Symbol("string?"): _named(
                "string?", lambda x: isinstance(x, str) and not isinstance(x, Symbol)
            ),
            Symbol("list?"): _named("list?", lambda x: isinstance(x, list)),
            Symbol("procedure?"): _named(
                "procedure?", lambda x: isinstance(x, Lambda) or callable(x)
            ),
            Symbol("eq?"): _named("eq?", _eq),
            Symbol("equal?"): _named("equal?", _equal),
            # i/o
            Symbol("format"): _named("format", _format),
            Symbol("print"): _named("print", _print),
            # misc
            Symbol("abs"): _named("abs", _abs),
            Symbol("min"): _named("min", _min),
            Symbol("max"): _named("max", _max),
            # constants
            Symbol("else"): True,
            Symbol("pi"): 3.141592653589793,
        }
    )
    return env
