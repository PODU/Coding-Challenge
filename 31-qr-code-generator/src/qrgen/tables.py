"""Constants and capacity tables from ISO/IEC 18004.

Only EC codewords per block and the block count are stored (from Nayuki's
public-domain tables). Everything else, such as data codewords, block sizes
and remainder bits, is computed from them.
"""
from __future__ import annotations

import math
from typing import Dict, List, Tuple

MODE_NUMERIC = "numeric"
MODE_ALPHANUMERIC = "alphanumeric"
MODE_BYTE = "byte"
MODE_KANJI = "kanji"

MODE_INDICATOR: Dict[str, int] = {
    MODE_NUMERIC: 0b0001,
    MODE_ALPHANUMERIC: 0b0010,
    MODE_BYTE: 0b0100,
    MODE_KANJI: 0b1000,
}

# A character's index here is its value in alphanumeric mode.
ALPHANUMERIC_CHARS = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ $%*+-./:"
ALPHANUMERIC_MAP: Dict[str, int] = {c: i for i, c in enumerate(ALPHANUMERIC_CHARS)}

EC_LEVELS = ("L", "M", "Q", "H")
_EC_INDEX = {"L": 0, "M": 1, "Q": 2, "H": 3}  # row in the tables below


def char_count_bits(mode: str, version: int) -> int:
    """Width in bits of the character count field."""
    if version <= 9:
        group = 0
    elif version <= 26:
        group = 1
    else:
        group = 2
    widths = {
        MODE_NUMERIC: (10, 12, 14),
        MODE_ALPHANUMERIC: (9, 11, 13),
        MODE_BYTE: (8, 16, 16),
        MODE_KANJI: (8, 10, 12),
    }
    return widths[mode][group]


# Indexed [ec_index][version]; index 0 of each row is unused.

_ECC_CODEWORDS_PER_BLOCK = [
    [-1, 7, 10, 15, 20, 26, 18, 20, 24, 30, 18, 20, 24, 26, 30, 22, 24, 28, 30,
     28, 28, 28, 28, 30, 30, 26, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30],  # L
    [-1, 10, 16, 26, 18, 24, 16, 18, 22, 22, 26, 30, 22, 22, 24, 24, 28, 28, 26,
     26, 26, 26, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28],  # M
    [-1, 13, 22, 18, 26, 18, 24, 18, 22, 20, 24, 28, 26, 24, 20, 30, 24, 28, 28,
     26, 30, 28, 30, 30, 30, 30, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30],  # Q
    [-1, 17, 28, 22, 16, 22, 28, 26, 26, 24, 28, 24, 28, 22, 24, 24, 30, 28, 28,
     26, 28, 30, 24, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30],  # H
]

_NUM_ERROR_CORRECTION_BLOCKS = [
    [-1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 4, 4, 4, 4, 4, 6, 6, 6, 6, 7, 8, 8, 9, 9,
     10, 12, 12, 12, 13, 14, 15, 16, 17, 18, 19, 19, 20, 21, 22, 24, 25],  # L
    [-1, 1, 1, 1, 2, 2, 4, 4, 4, 5, 5, 5, 8, 9, 9, 10, 10, 11, 13, 14, 16, 17,
     17, 18, 20, 21, 23, 25, 26, 28, 29, 31, 33, 35, 37, 38, 40, 43, 45, 47, 49],  # M
    [-1, 1, 1, 2, 2, 4, 4, 6, 6, 8, 8, 8, 10, 12, 16, 12, 17, 16, 18, 21, 20, 23,
     23, 25, 27, 29, 34, 34, 35, 38, 40, 43, 45, 48, 51, 53, 56, 59, 62, 65, 68],  # Q
    [-1, 1, 1, 2, 4, 4, 4, 5, 6, 8, 8, 11, 11, 16, 16, 18, 16, 19, 21, 25, 25, 25,
     34, 30, 32, 35, 37, 40, 42, 45, 48, 51, 54, 57, 60, 63, 66, 70, 74, 77, 81],  # H
]


def num_raw_data_modules(version: int) -> int:
    """Number of data-bearing module bits (excludes function patterns)."""
    if not 1 <= version <= 40:
        raise ValueError(f"version out of range: {version}")
    result = (16 * version + 128) * version + 64
    if version >= 2:
        num_align = version // 7 + 2
        result -= (25 * num_align - 10) * num_align - 55
        if version >= 7:
            result -= 36
    return result


def total_codewords(version: int) -> int:
    """Total codewords (data + EC) the symbol holds."""
    return num_raw_data_modules(version) // 8


def remainder_bits(version: int) -> int:
    """Trailing remainder bits (0/3/4/7) after the last codeword."""
    return num_raw_data_modules(version) % 8


def ec_codewords_per_block(version: int, ec: str) -> int:
    return _ECC_CODEWORDS_PER_BLOCK[_EC_INDEX[ec]][version]


def num_blocks(version: int, ec: str) -> int:
    return _NUM_ERROR_CORRECTION_BLOCKS[_EC_INDEX[ec]][version]


def num_data_codewords(version: int, ec: str) -> int:
    """Number of *data* codewords available to the payload."""
    return total_codewords(version) - ec_codewords_per_block(version, ec) * num_blocks(version, ec)


def block_structure(version: int, ec: str) -> Tuple[int, List[int]]:
    """Return ``(ec_per_block, [data_len_per_block, ...])``.

    Blocks are split into a short group and (optionally) a longer group whose
    blocks each hold one extra data codeword, per the spec's interleaving rules.
    """
    blocks = num_blocks(version, ec)
    data_total = num_data_codewords(version, ec)
    ecpb = ec_codewords_per_block(version, ec)
    short_len = data_total // blocks
    num_long = data_total % blocks
    lengths = [short_len] * (blocks - num_long) + [short_len + 1] * num_long
    return ecpb, lengths


def data_capacity_bits(version: int, ec: str) -> int:
    """Payload bit budget (data codewords * 8)."""
    return num_data_codewords(version, ec) * 8


def alignment_positions(version: int) -> List[int]:
    """Row and column centres of the alignment patterns (Nayuki's method)."""
    if version == 1:
        return []
    num_align = version // 7 + 2
    if version == 32:
        step = 26
    else:
        step = math.ceil((version * 4 + 4) / (num_align * 2 - 2)) * 2
    size = version * 4 + 17
    result: List[int] = []
    pos = size - 7
    for _ in range(num_align - 1):
        result.insert(0, pos)
        pos -= step
    result.insert(0, 6)
    return result
