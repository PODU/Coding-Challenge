"""GF(256) arithmetic using QR's primitive polynomial 0x11D."""
from __future__ import annotations

from typing import List

PRIMITIVE = 0x11D  # x^8 + x^4 + x^3 + x^2 + 1

EXP: List[int] = [0] * 512  # antilog, doubled so EXP[i+255]==EXP[i]
LOG: List[int] = [0] * 256


def _build_tables() -> None:
    x = 1
    for i in range(255):
        EXP[i] = x
        LOG[x] = i
        x <<= 1
        if x & 0x100:
            x ^= PRIMITIVE
    for i in range(255, 512):
        EXP[i] = EXP[i - 255]


_build_tables()


def mul(a: int, b: int) -> int:
    if a == 0 or b == 0:
        return 0
    return EXP[LOG[a] + LOG[b]]


def div(a: int, b: int) -> int:
    if b == 0:
        raise ZeroDivisionError("GF(256) division by zero")
    if a == 0:
        return 0
    return EXP[(LOG[a] - LOG[b]) % 255]


def pow_(a: int, n: int) -> int:
    if a == 0:
        return 0 if n != 0 else 1
    return EXP[(LOG[a] * n) % 255]


def inverse(a: int) -> int:
    if a == 0:
        raise ZeroDivisionError("GF(256) has no inverse for 0")
    return EXP[(255 - LOG[a]) % 255]
