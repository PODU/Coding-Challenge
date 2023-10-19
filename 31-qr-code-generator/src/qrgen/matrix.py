"""The module grid: function patterns and data placement."""
from __future__ import annotations

from typing import List, Optional

from . import format_info, tables
from .analyze import EncodingPlan
from .encode import final_bitstream


class Matrix:
    """Grid of modules (``True`` is dark), plus which ones are function modules."""

    def __init__(self, version: int) -> None:
        self.version = version
        self.size = version * 4 + 17
        self.modules: List[List[bool]] = [
            [False] * self.size for _ in range(self.size)
        ]
        self.is_function: List[List[bool]] = [
            [False] * self.size for _ in range(self.size)
        ]

    def _set_function(self, row: int, col: int, dark: bool) -> None:
        if 0 <= row < self.size and 0 <= col < self.size:
            self.modules[row][col] = dark
            self.is_function[row][col] = True

    def _place_finder(self, row: int, col: int) -> None:
        """Draw a 7x7 finder with its top-left at (row, col), plus the separator."""
        for r in range(-1, 8):
            for c in range(-1, 8):
                rr, cc = row + r, col + c
                if not (0 <= rr < self.size and 0 <= cc < self.size):
                    continue
                if 0 <= r < 7 and 0 <= c < 7:
                    dark = (
                        r in (0, 6)
                        or c in (0, 6)
                        or (2 <= r <= 4 and 2 <= c <= 4)
                    )
                else:
                    dark = False  # separator ring
                self._set_function(rr, cc, dark)

    def _place_alignment(self, cr: int, cc: int) -> None:
        for r in range(-2, 3):
            for c in range(-2, 3):
                dark = max(abs(r), abs(c)) != 1
                self._set_function(cr + r, cc + c, dark)

    def _place_function_patterns(self) -> None:
        self._place_finder(0, 0)
        self._place_finder(0, self.size - 7)
        self._place_finder(self.size - 7, 0)

        # Timing patterns along row 6 and column 6.
        for i in range(8, self.size - 8):
            dark = (i % 2 == 0)
            if not self.is_function[6][i]:
                self._set_function(6, i, dark)
            if not self.is_function[i][6]:
                self._set_function(i, 6, dark)

        positions = tables.alignment_positions(self.version)
        if positions:
            last = positions[-1]
            skip = {(6, 6), (6, last), (last, 6)}
            for r in positions:
                for c in positions:
                    if (r, c) in skip:
                        continue
                    self._place_alignment(r, c)

        # Reserve the format and version areas; they are drawn after masking.
        for row, col, _ in format_info.format_coordinates(self.size):
            self.is_function[row][col] = True
        self._set_function(self.size - 8, 8, True)  # the "dark module"

        if self.version >= 7:
            for row, col, _ in format_info.version_coordinates(self.size):
                self.is_function[row][col] = True

    def _place_data(self, bits: List[int]) -> None:
        idx = 0
        n = len(bits)
        # Two columns at a time, right to left; skip the vertical timing column.
        for right in range(self.size - 1, 0, -2):
            if right <= 6:
                right -= 1
            upward = ((right + 1) & 2) == 0
            for vert in range(self.size):
                row = (self.size - 1 - vert) if upward else vert
                for j in range(2):
                    col = right - j
                    if not self.is_function[row][col] and idx < n:
                        self.modules[row][col] = bool(bits[idx])
                        idx += 1
        if idx != n:  # pragma: no cover - sanity guard
            raise RuntimeError(f"placed {idx} of {n} data bits")

    def draw_format_bits(self, ec: str, mask: int) -> None:
        value = format_info.format_bits(ec, mask)
        for row, col, bit in format_info.format_coordinates(self.size):
            self.modules[row][col] = bool((value >> bit) & 1)

    def draw_version_bits(self) -> None:
        if self.version < 7:
            return
        value = format_info.version_bits(self.version)
        for row, col, bit in format_info.version_coordinates(self.size):
            self.modules[row][col] = bool((value >> bit) & 1)


def build_base_matrix(plan: EncodingPlan) -> Matrix:
    """Matrix with function patterns and data, before masking."""
    m = Matrix(plan.version)
    m._place_function_patterns()
    m._place_data(final_bitstream(plan))
    return m
