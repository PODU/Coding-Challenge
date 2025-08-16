"""Compiled instructions.

``IF/ELSE/THEN`` and ``DO/LOOP`` are compiled into nested :class:`If` and
:class:`Loop` instructions, so running code is a simple recursive walk.
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class Push:
    """Push a number."""

    value: int


@dataclass(frozen=True)
class Call:
    """Call a word by its lower-case name. It is looked up when it runs, so a
    word can be redefined after something that uses it."""

    name: str


@dataclass(frozen=True)
class PrintStr:
    """Print the text of a ``." ..."`` string."""

    text: str


@dataclass(frozen=True)
class If:
    """Pop a flag and run ``then_block`` if it is non-zero, else ``else_block``."""

    then_block: list["Instruction"]
    else_block: list["Instruction"] | None


@dataclass(frozen=True)
class Loop:
    """A ``DO ... LOOP`` body, run with the index going from start up to limit."""

    body: list["Instruction"]


Instruction = Push | Call | PrintStr | If | Loop
