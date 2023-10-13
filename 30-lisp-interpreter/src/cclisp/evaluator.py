"""The evaluator: ``evaluate(x, env)`` plus value formatting helpers."""

from __future__ import annotations

from typing import Callable

from .environment import Environment
from .types import (
    Lambda,
    LispArityError,
    LispError,
    LispSyntaxError,
    Symbol,
)

SPECIAL_FORMS = frozenset(
    {
        "quote",
        "if",
        "define",
        "defun",
        "lambda",
        "set!",
        "begin",
        "let",
        "and",
        "or",
    }
)


def is_truthy(value: object) -> bool:
    """Only ``nil`` (False) and the empty list are false."""
    return not (value is False or (isinstance(value, list) and not value))


def lisp_str(value: object, *, repr_strings: bool = False) -> str:
    """Format *value* the way Lisp would print it.

    With ``repr_strings`` strings are quoted and escaped (REPL output, ``~S``);
    otherwise they are printed bare (``~A``).
    """
    if value is True:
        return "t"
    if value is False:
        return "nil"
    if isinstance(value, Symbol):
        return str(value)
    if isinstance(value, str):
        if repr_strings:
            escaped = (
                value.replace("\\", "\\\\")
                .replace('"', '\\"')
                .replace("\n", "\\n")
                .replace("\t", "\\t")
            )
            return f'"{escaped}"'
        return value
    if isinstance(value, list):
        if not value:
            return "nil"
        inner = " ".join(lisp_str(v, repr_strings=True) for v in value)
        return f"({inner})"
    if isinstance(value, Lambda):
        return repr(value)
    if callable(value):
        name = getattr(value, "lisp_name", getattr(value, "__name__", "builtin"))
        return f"#<builtin {name}>"
    return str(value)


def evaluate(x: object, env: Environment) -> object:
    """Evaluate expression *x* in environment *env*."""
    while True:
        # Symbol subclasses str, so this has to come before the atom check.
        if isinstance(x, Symbol):
            return env.find(x)[x]

        if not isinstance(x, list):
            return x

        if len(x) == 0:
            return False

        op = x[0]

        if isinstance(op, Symbol) and op in SPECIAL_FORMS:
            if op == "quote":
                _require_args(x, 1, "quote")
                return x[1]

            if op == "if":
                if len(x) not in (3, 4):
                    raise LispSyntaxError("if requires (if test conseq [alt])")
                test = evaluate(x[1], env)
                if is_truthy(test):
                    x = x[2]
                elif len(x) == 4:
                    x = x[3]
                else:
                    return False
                continue

            if op == "define":
                _require_args(x, 2, "define")
                name = x[1]
                if not isinstance(name, Symbol):
                    raise LispSyntaxError("define: first argument must be a symbol")
                env[name] = evaluate(x[2], env)
                return name

            # (defun name (params) body...)
            if op == "defun":
                if len(x) < 4:
                    raise LispSyntaxError(
                        "defun requires (defun name (params) body...)"
                    )
                name = x[1]
                params = x[2]
                if not isinstance(name, Symbol):
                    raise LispSyntaxError("defun: name must be a symbol")
                if not isinstance(params, list):
                    raise LispSyntaxError("defun: params must be a list")
                env[name] = Lambda(
                    list(params), _wrap_body(x[3:]), env, name=str(name)
                )
                return name

            if op == "lambda":
                if len(x) < 3:
                    raise LispSyntaxError("lambda requires (lambda (params) body...)")
                params = x[1]
                if not isinstance(params, list):
                    raise LispSyntaxError("lambda: params must be a list")
                return Lambda(list(params), _wrap_body(x[2:]), env)

            if op == "set!":
                _require_args(x, 2, "set!")
                name = x[1]
                if not isinstance(name, Symbol):
                    raise LispSyntaxError("set!: first argument must be a symbol")
                value = evaluate(x[2], env)
                env.find(name)[name] = value
                return value

            if op == "begin":
                if len(x) == 1:
                    return False
                for expr in x[1:-1]:
                    evaluate(expr, env)
                x = x[-1]
                continue

            # (let ((a 1) (b 2)) body...)
            if op == "let":
                if len(x) < 2 or not isinstance(x[1], list):
                    raise LispSyntaxError("let requires (let ((name val)...) body...)")
                child = Environment(parent=env)
                for binding in x[1]:
                    if not isinstance(binding, list) or len(binding) != 2:
                        raise LispSyntaxError("let: malformed binding")
                    bname, bexpr = binding
                    if not isinstance(bname, Symbol):
                        raise LispSyntaxError("let: binding name must be a symbol")
                    child[bname] = evaluate(bexpr, env)
                env = child
                x = _wrap_body(x[2:])
                continue

            if op == "and":
                result: object = True
                for expr in x[1:]:
                    result = evaluate(expr, env)
                    if not is_truthy(result):
                        return False
                return result

            if op == "or":
                for expr in x[1:]:
                    result = evaluate(expr, env)
                    if is_truthy(result):
                        return result
                return False

        proc = evaluate(op, env)
        args = [evaluate(arg, env) for arg in x[1:]]

        if isinstance(proc, Lambda):
            if len(args) != len(proc.params):
                raise LispArityError(
                    f"{proc.name} expects {len(proc.params)} argument(s), "
                    f"got {len(args)}"
                )
            env = Environment(proc.params, args, proc.env)
            x = proc.body
            continue

        if callable(proc):
            return proc(*args)

        raise LispError(f"not a procedure: {lisp_str(proc)}")


def _wrap_body(body: list) -> object:
    """Turn a body of one or more expressions into a single form."""
    if len(body) == 1:
        return body[0]
    return [Symbol("begin"), *body]


def _require_args(form: list, count: int, name: str) -> None:
    if len(form) != count + 1:
        raise LispSyntaxError(f"{name} expects {count} argument(s)")
