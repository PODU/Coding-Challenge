"""Reed-Solomon error correction codewords."""
from __future__ import annotations

from functools import lru_cache
from typing import List, Sequence, Tuple

from . import galois


@lru_cache(maxsize=None)
def generator_poly(degree: int) -> Tuple[int, ...]:
    """Generator polynomial ``(x-a^0)(x-a^1)...(x-a^(degree-1))``.

    Coefficients are highest order first, including the leading 1.
    """
    poly: List[int] = [1]
    for i in range(degree):
        # Subtraction is XOR in GF(256), so (x - a^i) == (x + a^i).
        root = galois.EXP[i]
        new = [0] * (len(poly) + 1)
        for j, coef in enumerate(poly):
            new[j] ^= coef  # times x
            new[j + 1] ^= galois.mul(coef, root)
        poly = new
    return tuple(poly)


def ec_codewords(data: Sequence[int], num_ec: int) -> List[int]:
    """The ``num_ec`` EC codewords for ``data``: the remainder of polynomial
    division by the generator."""
    gen = generator_poly(num_ec)
    remainder = [0] * num_ec
    for byte in data:
        factor = byte ^ remainder[0]
        remainder = remainder[1:] + [0]
        if factor != 0:
            lf = galois.LOG[factor]
            for i in range(num_ec):
                # gen[0] is the leading 1, already accounted for by the shift.
                g = gen[i + 1]
                if g != 0:
                    remainder[i] ^= galois.EXP[galois.LOG[g] + lf]
    return remainder
