"""Find text in an ink mask: connected components and line regions."""

from __future__ import annotations

import numpy as np

from .types import Box


def true_runs(flags: np.ndarray) -> list[tuple[int, int]]:
    """Return the ``(start, end)`` ranges where ``flags`` is True."""
    out: list[tuple[int, int]] = []
    n = len(flags)
    i = 0
    while i < n:
        if flags[i]:
            j = i
            while j < n and flags[j]:
                j += 1
            out.append((i, j))
            i = j
        else:
            i += 1
    return out


def connected_components(mask: np.ndarray) -> tuple[np.ndarray, int]:
    """Label 8-connected components with a flood fill.

    Returns ``(labels, count)``; label 0 is background.
    """
    h, w = mask.shape
    labels = np.zeros((h, w), dtype=np.int32)
    current = 0
    ys, xs = np.nonzero(mask)
    visited = np.zeros((h, w), dtype=bool)
    neighbours = [(-1, -1), (-1, 0), (-1, 1), (0, -1), (0, 1), (1, -1), (1, 0), (1, 1)]
    for sy, sx in zip(ys.tolist(), xs.tolist()):
        if visited[sy, sx]:
            continue
        current += 1
        stack = [(sy, sx)]
        visited[sy, sx] = True
        while stack:
            cy, cx = stack.pop()
            labels[cy, cx] = current
            for dy, dx in neighbours:
                ny, nx = cy + dy, cx + dx
                if 0 <= ny < h and 0 <= nx < w and mask[ny, nx] and not visited[ny, nx]:
                    visited[ny, nx] = True
                    stack.append((ny, nx))
    return labels, current


def _component_boxes(labels: np.ndarray, count: int) -> list[Box]:
    boxes: list[Box] = []
    for lbl in range(1, count + 1):
        ys, xs = np.nonzero(labels == lbl)
        if ys.size == 0:
            continue
        x0, x1 = int(xs.min()), int(xs.max()) + 1
        y0, y1 = int(ys.min()), int(ys.max()) + 1
        boxes.append(Box(x0, y0, x1 - x0, y1 - y0))
    return boxes


def find_text_regions(mask: np.ndarray, *, line_gap: int = 1) -> list[Box]:
    """Return a tight bounding box for each line of text."""
    if not mask.any():
        return []
    bands = true_runs(mask.any(axis=1))
    # Bands only a row or so apart belong to the same line.
    merged: list[tuple[int, int]] = []
    for y0, y1 in bands:
        if merged and y0 - merged[-1][1] <= line_gap:
            merged[-1] = (merged[-1][0], y1)
        else:
            merged.append((y0, y1))

    regions: list[Box] = []
    for y0, y1 in merged:
        sub = mask[y0:y1]
        cols = np.where(sub.any(axis=0))[0]
        rows = np.where(sub.any(axis=1))[0]
        if cols.size == 0 or rows.size == 0:
            continue
        x0, x1 = int(cols[0]), int(cols[-1]) + 1
        ry0, ry1 = y0 + int(rows[0]), y0 + int(rows[-1]) + 1
        regions.append(Box(x0, ry0, x1 - x0, ry1 - ry0))
    return regions


def has_text(mask: np.ndarray, *, min_ink: int = 3) -> bool:
    """True if the mask has at least *min_ink* ink pixels."""
    return int(mask.sum()) >= min_ink
