"""The interpreter: data stack, dictionary and execution."""

from __future__ import annotations

import operator
import sys
from dataclasses import dataclass, field
from typing import Callable, TextIO

from .compiler import compile_until
from .errors import (
    CompileError,
    ForthError,
    ForthExit,
    StackUnderflow,
    UnknownWord,
)
from .instructions import Call, If, Instruction, Loop, Push, PrintStr
from .tokenizer import Token, tokenize

Primitive = Callable[["Interpreter"], None]


@dataclass(frozen=True)
class UserWord:
    """A word defined with ``: name ... ;``."""

    name: str
    body: list[Instruction]


@dataclass(frozen=True)
class PrimitiveWord:
    """A built-in word."""

    name: str
    fn: Primitive


Word = UserWord | PrimitiveWord


@dataclass
class Interpreter:
    """Feed it source with :meth:`run`; output goes to :attr:`out`."""

    out: TextIO = field(default_factory=lambda: sys.stdout)
    stack: list[int] = field(default_factory=list)
    dictionary: dict[str, Word] = field(default_factory=dict)
    loop_stack: list[int] = field(default_factory=list)

    def __post_init__(self) -> None:
        self._install_builtins()

    def push(self, value: int) -> None:
        self.stack.append(value)

    def pop(self, word: str) -> int:
        if not self.stack:
            raise StackUnderflow(word)
        return self.stack.pop()

    def _need(self, count: int, word: str) -> None:
        if len(self.stack) < count:
            raise StackUnderflow(word)

    def run(self, source: str) -> None:
        """Run *source*. Definitions are stored; everything else runs at once.

        ``bye`` raises :class:`ForthExit`.
        """
        tokens = tokenize(source)
        i = 0
        n = len(tokens)
        while i < n:
            tok = tokens[i]
            if tok.kind == "word" and tok.text == ":":
                i = self._compile_definition(tokens, i + 1)
            else:
                instrs, i, _stop = compile_until(tokens, i, set())
                self.execute(instrs)

    def _compile_definition(self, tokens: list[Token], i: int) -> int:
        n = len(tokens)
        if i >= n:
            raise CompileError("unterminated definition")
        name_tok = tokens[i]
        if name_tok.kind != "word" or name_tok.text == ";":
            raise CompileError("expected a name after ':'")
        name = name_tok.text.lower()
        body, i, stop = compile_until(tokens, i + 1, {";"})
        if stop != ";":
            raise CompileError("unterminated definition")
        self.dictionary[name] = UserWord(name, body)
        return i

    def execute(self, instrs: list[Instruction]) -> None:
        for ins in instrs:
            if isinstance(ins, Push):
                self.stack.append(ins.value)
            elif isinstance(ins, Call):
                self._call(ins.name)
            elif isinstance(ins, PrintStr):
                self.out.write(ins.text)
            elif isinstance(ins, If):
                flag = self.pop("if")
                if flag != 0:
                    self.execute(ins.then_block)
                elif ins.else_block is not None:
                    self.execute(ins.else_block)
            elif isinstance(ins, Loop):
                start = self.pop("do")
                limit = self.pop("do")
                for index in range(start, limit):
                    self.loop_stack.append(index)
                    try:
                        self.execute(ins.body)
                    finally:
                        self.loop_stack.pop()

    def _call(self, name: str) -> None:
        word = self.dictionary.get(name)
        if word is None:
            raise UnknownWord(name)
        if isinstance(word, PrimitiveWord):
            word.fn(self)
        else:
            self.execute(word.body)

    def stack_summary(self) -> str:
        """The ``<depth> v1 v2 ...`` text shown in the REPL prompt."""
        if not self.stack:
            return "<0>"
        return f"<{len(self.stack)}> " + " ".join(str(v) for v in self.stack)

    def _install_builtins(self) -> None:
        d = self.dictionary

        def prim(name: str, fn: Primitive) -> None:
            d[name] = PrimitiveWord(name, fn)

        def binary(name: str, op: Callable[[int, int], int]) -> None:
            def fn(it: Interpreter) -> None:
                b = it.pop(name)
                a = it.pop(name)
                it.push(op(a, b))

            prim(name, fn)

        def flag(test: Callable[[int, int], bool]) -> Callable[[int, int], int]:
            return lambda a, b: -1 if test(a, b) else 0

        binary("+", operator.add)
        binary("-", operator.sub)
        binary("*", operator.mul)

        def w_div(it: Interpreter) -> None:
            b = it.pop("/"); a = it.pop("/")
            if b == 0:
                raise ForthError("/: division by zero")
            it.push(a // b)  # rounds toward negative infinity

        def w_mod(it: Interpreter) -> None:
            b = it.pop("mod"); a = it.pop("mod")
            if b == 0:
                raise ForthError("mod: division by zero")
            it.push(a % b)

        prim("/", w_div)
        prim("mod", w_mod)

        def w_dup(it: Interpreter) -> None:
            it._need(1, "dup"); it.push(it.stack[-1])

        def w_drop(it: Interpreter) -> None:
            it.pop("drop")

        def w_swap(it: Interpreter) -> None:
            it._need(2, "swap")
            it.stack[-1], it.stack[-2] = it.stack[-2], it.stack[-1]

        def w_over(it: Interpreter) -> None:
            it._need(2, "over"); it.push(it.stack[-2])

        def w_rot(it: Interpreter) -> None:
            it._need(3, "rot")
            a, b, c = it.stack[-3], it.stack[-2], it.stack[-1]
            it.stack[-3], it.stack[-2], it.stack[-1] = b, c, a

        prim("dup", w_dup)
        prim("drop", w_drop)
        prim("swap", w_swap)
        prim("over", w_over)
        prim("rot", w_rot)

        def w_dot(it: Interpreter) -> None:
            it.out.write(f"{it.pop('.')} ")

        def w_emit(it: Interpreter) -> None:
            n = it.pop("emit")
            try:
                ch = chr(n)
            except (ValueError, OverflowError):
                raise ForthError("emit: codepoint out of range")
            it.out.write(ch)

        def w_cr(it: Interpreter) -> None:
            it.out.write("\n")

        prim(".", w_dot)
        prim("emit", w_emit)
        prim("cr", w_cr)

        # Forth's true is -1 (all bits set), so and/or/invert work on flags.
        binary("<", flag(operator.lt))
        binary(">", flag(operator.gt))
        binary("=", flag(operator.eq))
        binary("<>", flag(operator.ne))
        binary("and", operator.and_)
        binary("or", operator.or_)

        def w_invert(it: Interpreter) -> None:
            it.push(~it.pop("invert"))

        prim("invert", w_invert)

        def w_i(it: Interpreter) -> None:
            if not it.loop_stack:
                raise ForthError("i: not inside a loop")
            it.push(it.loop_stack[-1])

        prim("i", w_i)

        def w_bye(it: Interpreter) -> None:
            raise ForthExit()

        prim("bye", w_bye)
