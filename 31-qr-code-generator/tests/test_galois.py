"""GF(256) arithmetic tests against brute-force reference implementations."""
import pytest

from qrgen import galois

PRIM = 0x11D


def _ref_mul(a: int, b: int) -> int:
    """Carry-less multiply modulo the QR primitive polynomial."""
    p = 0
    while b:
        if b & 1:
            p ^= a
        b >>= 1
        a <<= 1
        if a & 0x100:
            a ^= PRIM
    return p


def test_tables_are_inverses():
    for x in range(1, 256):
        assert galois.EXP[galois.LOG[x]] == x
    # exp/log round-trip on the log side too
    for i in range(255):
        assert galois.LOG[galois.EXP[i]] == i


def test_mul_matches_bruteforce():
    for a in range(256):
        for b in range(256):
            assert galois.mul(a, b) == _ref_mul(a, b)


def test_mul_identity_and_zero():
    for a in range(256):
        assert galois.mul(a, 1) == a
        assert galois.mul(a, 0) == 0
        assert galois.mul(0, a) == 0


def test_div_is_mul_inverse():
    for a in range(256):
        for b in range(1, 256):
            q = galois.div(a, b)
            assert galois.mul(q, b) == a


def test_div_by_zero_raises():
    with pytest.raises(ZeroDivisionError):
        galois.div(5, 0)


def test_inverse():
    for a in range(1, 256):
        assert galois.mul(a, galois.inverse(a)) == 1


def test_pow():
    assert galois.pow_(2, 0) == 1
    for n in range(1, 20):
        expected = 1
        for _ in range(n):
            expected = galois.mul(expected, 3)
        assert galois.pow_(3, n) == expected
