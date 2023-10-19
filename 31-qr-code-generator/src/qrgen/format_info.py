"""Format information (BCH 15,5) and version information (BCH 18,6)."""
from __future__ import annotations

from typing import List, Tuple

# Note these are not in L/M/Q/H order.
FORMAT_EC_BITS = {"L": 0b01, "M": 0b00, "Q": 0b11, "H": 0b10}

_FORMAT_GENERATOR = 0x537  # BCH(15,5) generator polynomial
_FORMAT_XOR_MASK = 0x5412  # 101010000010010
_VERSION_GENERATOR = 0x1F25  # BCH(18,6) generator polynomial


def _bch_remainder(data: int, generator: int, fill_bits: int) -> int:
    rem = data << fill_bits
    gen_bits = generator.bit_length()
    for i in range(data.bit_length() + fill_bits - 1, fill_bits - 1, -1):
        if rem & (1 << i):
            rem ^= generator << (i - gen_bits + 1)
    return rem & ((1 << fill_bits) - 1)


def format_bits(ec: str, mask: int) -> int:
    """The 15-bit format word for an EC level and mask, already XOR-masked."""
    if not 0 <= mask <= 7:
        raise ValueError(f"mask out of range: {mask}")
    data = (FORMAT_EC_BITS[ec] << 3) | mask
    rem = _bch_remainder(data, _FORMAT_GENERATOR, 10)
    return ((data << 10) | rem) ^ _FORMAT_XOR_MASK


def version_bits(version: int) -> int:
    """The 18-bit version word. Only versions 7 and up have one."""
    if version < 7:
        raise ValueError("version info only exists for versions >= 7")
    rem = _bch_remainder(version, _VERSION_GENERATOR, 12)
    return (version << 12) | rem


def format_coordinates(size: int) -> List[Tuple[int, int, int]]:
    """Return ``(row, col, bit_index)`` for the 30 format modules.

    Both copies start with bit 14 (the MSB). bit_index 0 is the LSB.
    """
    # Around the top-left finder.
    copy1 = (
        [(8, i) for i in range(6)]
        + [(8, 7), (8, 8), (7, 8)]
        + [(5 - i, 8) for i in range(6)]  # (5,8),(4,8),(3,8),(2,8),(1,8),(0,8)
    )
    # Seven modules beside the bottom-left finder, then eight below the
    # top-right one. The dark module at (size-8, 8) is not part of it.
    copy2 = (
        [(size - 1 - i, 8) for i in range(7)]
        + [(8, size - 8 + i) for i in range(8)]
    )
    coords: List[Tuple[int, int, int]] = []
    for positions in (copy1, copy2):
        for k, (r, c) in enumerate(positions):
            coords.append((r, c, 14 - k))
    return coords


def version_coordinates(size: int) -> List[Tuple[int, int, int]]:
    """Return ``(row, col, bit_index)`` for the 36 version modules."""
    coords: List[Tuple[int, int, int]] = []
    for i in range(18):
        a = size - 11 + i % 3
        b = i // 3
        coords.append((a, b, i))  # bottom-left block
        coords.append((b, a, i))  # top-right block
    return coords
