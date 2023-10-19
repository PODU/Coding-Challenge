"""Mask patterns and the penalty score used to pick one."""
from __future__ import annotations

import copy
from typing import Callable, List, Tuple

from .matrix import Matrix

MASK_FUNCS: List[Callable[[int, int], bool]] = [
    lambda i, j: (i + j) % 2 == 0,
    lambda i, j: i % 2 == 0,
    lambda i, j: j % 3 == 0,
    lambda i, j: (i + j) % 3 == 0,
    lambda i, j: (i // 2 + j // 3) % 2 == 0,
    lambda i, j: (i * j) % 2 + (i * j) % 3 == 0,
    lambda i, j: ((i * j) % 2 + (i * j) % 3) % 2 == 0,
    lambda i, j: ((i + j) % 2 + (i * j) % 3) % 2 == 0,
]


def apply_mask(matrix: Matrix, mask_id: int) -> None:
    """Flip every data module the mask pattern selects, in place."""
    func = MASK_FUNCS[mask_id]
    for r in range(matrix.size):
        for c in range(matrix.size):
            if not matrix.is_function[r][c] and func(r, c):
                matrix.modules[r][c] = not matrix.modules[r][c]


def _penalty_n1(grid: List[List[bool]], size: int) -> int:
    penalty = 0
    for lines in (grid, list(zip(*grid))):  # rows, then columns
        for row in lines:
            run_color = None
            run_len = 0
            for cell in row:
                if cell == run_color:
                    run_len += 1
                else:
                    if run_len >= 5:
                        penalty += 3 + (run_len - 5)
                    run_color = cell
                    run_len = 1
            if run_len >= 5:
                penalty += 3 + (run_len - 5)
    return penalty


def _penalty_n2(grid: List[List[bool]], size: int) -> int:
    penalty = 0
    for r in range(size - 1):
        for c in range(size - 1):
            v = grid[r][c]
            if v == grid[r][c + 1] == grid[r + 1][c] == grid[r + 1][c + 1]:
                penalty += 3
    return penalty


def _penalty_n3(grid: List[List[bool]], size: int) -> int:
    pat1 = [True, False, True, True, True, False, True,
            False, False, False, False]
    pat2 = list(reversed(pat1))
    penalty = 0
    cols = [list(col) for col in zip(*grid)]
    for lines in (grid, cols):
        for row in lines:
            row = list(row)
            for c in range(size - 10):
                window = row[c:c + 11]
                if window == pat1 or window == pat2:
                    penalty += 40
    return penalty


def _penalty_n4(grid: List[List[bool]], size: int) -> int:
    dark = sum(cell for row in grid for cell in row)
    total = size * size
    percent = dark * 100 / total
    prev = (int(percent) // 5) * 5
    nxt = prev + 5
    return min(abs(prev - 50), abs(nxt - 50)) // 5 * 10


def penalty(matrix: Matrix) -> int:
    grid = matrix.modules
    size = matrix.size
    return (
        _penalty_n1(grid, size)
        + _penalty_n2(grid, size)
        + _penalty_n3(grid, size)
        + _penalty_n4(grid, size)
    )


def select_mask(base: Matrix, ec: str, forced: int = None) -> Tuple[Matrix, int]:
    """Return the masked matrix with the lowest penalty, and its mask id.

    ``base`` should have its data placed but no mask. The result also has the
    format and version bits drawn.
    """
    candidates = range(8) if forced is None else (forced,)
    best_matrix = None
    best_mask = -1
    best_score = None
    for mask_id in candidates:
        trial = copy.deepcopy(base)
        apply_mask(trial, mask_id)
        trial.draw_format_bits(ec, mask_id)
        trial.draw_version_bits()
        score = penalty(trial)
        if best_score is None or score < best_score:
            best_score = score
            best_mask = mask_id
            best_matrix = trial
    return best_matrix, best_mask
