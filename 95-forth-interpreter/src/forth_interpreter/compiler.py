"""Compile tokens into instructions.

Top-level code and ``: ... ;`` bodies go through the same routine. Control
flow is compiled recursively, so nesting works and unbalanced structures are
caught.
"""

from __future__ import annotations

from .errors import CompileError
from .instructions import Call, If, Instruction, Loop, Push, PrintStr
from .tokenizer import Token

_TERMINATORS = {";", "then", "else", "loop"}


def parse_int(text: str) -> int | None:
    """Parse a decimal integer, or return ``None``.

    Python's ``int`` accepts ``1_2``, but in Forth that is a word, not a number.
    """
    if "_" in text:
        return None
    try:
        return int(text)
    except ValueError:
        return None


def compile_until(
    tokens: list[Token], i: int, terminators: set[str]
) -> tuple[list[Instruction], int, str | None]:
    """Compile from *i* up to one of *terminators*, a ``:``, or the end.

    Returns ``(instructions, next_index, stop_word)``. ``stop_word`` is the
    terminator (consumed), ``":"`` (not consumed), or ``None`` at the end.
    """
    instrs: list[Instruction] = []
    n = len(tokens)
    while i < n:
        tok = tokens[i]
        if tok.kind == "string":
            instrs.append(PrintStr(tok.text))
            i += 1
            continue

        word = tok.text
        lw = word.lower()

        if lw == ":":
            # Definitions are handled by Interpreter.run.
            return instrs, i, ":"
        if lw in terminators:
            return instrs, i + 1, lw
        if lw == ";":
            raise CompileError("unbalanced definition")
        if lw in ("then", "else"):
            raise CompileError("unbalanced IF/THEN")
        if lw == "loop":
            raise CompileError("unbalanced DO/LOOP")

        if lw == "if":
            then_block, i, stop = compile_until(tokens, i + 1, {"else", "then"})
            if stop == "else":
                else_block, i, stop2 = compile_until(tokens, i, {"then"})
                if stop2 != "then":
                    raise CompileError("unbalanced IF/THEN")
                instrs.append(If(then_block, else_block))
            elif stop == "then":
                instrs.append(If(then_block, None))
            else:
                raise CompileError("unbalanced IF/THEN")
            continue

        if lw == "do":
            body, i, stop = compile_until(tokens, i + 1, {"loop"})
            if stop != "loop":
                raise CompileError("unbalanced DO/LOOP")
            instrs.append(Loop(body))
            continue

        value = parse_int(word)
        if value is not None:
            instrs.append(Push(value))
        else:
            instrs.append(Call(lw))
        i += 1

    return instrs, i, None
