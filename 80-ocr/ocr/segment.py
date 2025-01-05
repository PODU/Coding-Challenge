"""Split an ink mask into lines and characters, and find word gaps."""

from __future__ import annotations

from dataclasses import dataclass

import numpy as np

from .detect import connected_components, true_runs
from .types import Box


@dataclass
class CharSpan:
    """A character's columns within a line, or the gap before a word."""

    x0: int
    x1: int
    is_space: bool = False


@dataclass
class Line:
    """A line of text with its character spans."""

    y0: int
    y1: int
    spans: list[CharSpan]
    baseline: float  # row, relative to y0
    cap_top: float   # row, relative to y0


def segment_lines_bounds(mask: np.ndarray, min_gap: int = 1) -> list[tuple[int, int]]:
    """Return the ``(y0, y1)`` row range of each line."""
    bands = true_runs(mask.any(axis=1))
    if not bands:
        return []
    # Merge bands separated by a tiny vertical gap (e.g. an 'i' dot).
    merged = [bands[0]]
    for y0, y1 in bands[1:]:
        py0, py1 = merged[-1]
        if y0 - py1 <= min_gap:
            merged[-1] = (py0, y1)
        else:
            merged.append((y0, y1))
    return merged


def _estimate_anchors(
    line: np.ndarray, spans: list[tuple[int, int]]
) -> tuple[float, float]:
    """Estimate the baseline and cap-top rows, relative to the line top.

    Most characters sit on the baseline and only a few have descenders, so the
    baseline is the biggest cluster of character bottoms. The cap top is simply
    the highest ink in the line.
    """
    bottoms: list[int] = []
    tops: list[int] = []
    for x0, x1 in spans:
        sub = line[:, x0:x1]
        rows = np.where(sub.any(axis=1))[0]
        if rows.size:
            tops.append(int(rows[0]))
            bottoms.append(int(rows[-1]))
    if not bottoms:
        return float(line.shape[0]), 0.0

    bottoms_arr = np.asarray(bottoms)
    tol = max(1.0, 0.12 * line.shape[0])
    # Find the bottom value with the most others within tol, and average those.
    best_v = bottoms_arr[0]
    best_count = -1
    for v in bottoms_arr:
        count = int(np.sum(np.abs(bottoms_arr - v) <= tol))
        if count > best_count:
            best_count = count
            best_v = v
    cluster = bottoms_arr[np.abs(bottoms_arr - best_v) <= tol]
    baseline = float(cluster.mean()) + 1.0
    cap_top = float(min(tops))
    return baseline, cap_top


def char_runs(line: np.ndarray, *, split_touching: bool = False) -> list[tuple[int, int]]:
    """Return the ``(x0, x1)`` runs of columns that contain ink.

    ``split_touching=True`` splits runs much wider than the median at their
    thinnest columns, for fonts whose letters touch. It is off by default
    because the bundled font leaves gaps, and splitting would break up wide
    letters like ``W`` and ``M``.
    """
    runs = true_runs(line.any(axis=0))
    if not runs or not split_touching:
        return runs
    median_w = float(np.median([b - a for a, b in runs]))
    refined: list[tuple[int, int]] = []
    for a, b in runs:
        if median_w > 0 and (b - a) > 2.0 * median_w:
            refined.extend(_split_wide(line, a, b, median_w))
        else:
            refined.append((a, b))
    refined.sort()
    return refined


# A gap wider than SPACE_RATIO x cap height counts as a word break. With the
# bundled font some word spaces are narrower than the right side bearing of
# glyphs like "1", so no fixed ratio is perfect; 0.36 keeps numbers together
# while catching most word spaces.
SPACE_RATIO = 0.36


def mark_spaces(
    runs: list[tuple[int, int]],
    cap_height: float,
    *,
    space_ratio: float = SPACE_RATIO,
    min_space: int = 4,
) -> list[CharSpan]:
    """Turn ink runs into spans, adding a space span at each word gap.

    The threshold scales with ``cap_height`` so it works at any font size.
    """
    cap_height = max(1.0, cap_height)
    space_thresh = max(min_space, space_ratio * cap_height)
    spans: list[CharSpan] = []
    prev_end: int | None = None
    for a, b in runs:
        if prev_end is not None and (a - prev_end) >= space_thresh:
            spans.append(CharSpan(prev_end, a, is_space=True))
        spans.append(CharSpan(a, b, is_space=False))
        prev_end = b
    return spans


def segment_chars_bounds(
    line: np.ndarray,
    *,
    min_space: int = 4,
    use_components: bool = True,
    split_touching: bool = False,
) -> list[CharSpan]:
    """Character spans for one line, with word gaps marked."""
    runs = (
        component_runs(line)
        if use_components
        else char_runs(line, split_touching=split_touching)
    )
    if not runs:
        return []
    baseline, cap_top = _estimate_anchors(line, runs)
    return mark_spaces(runs, baseline - cap_top, min_space=min_space)


def _split_wide(
    line: np.ndarray, a: int, b: int, median_w: float
) -> list[tuple[int, int]]:
    """Split the run ``[a, b)`` into roughly equal parts at its thinnest columns."""
    w = b - a
    parts = max(2, int(round(w / median_w)))
    col_sum = line[:, a:b].sum(axis=0).astype(float)
    cuts = [0]
    for k in range(1, parts):
        target = int(round(k * w / parts))
        lo = max(1, target - max(1, int(0.25 * median_w)))
        hi = min(w - 1, target + max(1, int(0.25 * median_w)))
        if hi <= lo:
            cut = target
        else:
            cut = lo + int(np.argmin(col_sum[lo:hi]))
        cuts.append(cut)
    cuts.append(w)
    out: list[tuple[int, int]] = []
    for i in range(len(cuts) - 1):
        s, e = cuts[i], cuts[i + 1]
        if e > s:
            out.append((a + s, a + e))
    return out


def component_runs(line: np.ndarray, *, overlap: float = 0.5) -> list[tuple[int, int]]:
    """Return ``(x0, x1)`` character runs built from connected components.

    Components that overlap horizontally, like the dot and stem of ``i`` or the
    two dots of ``:``, become one character. Glyphs whose columns only touch,
    such as a letter and the comma after it, stay separate, which a plain
    column projection would not manage.
    """
    labels, n = connected_components(line)
    if n == 0:
        return []
    boxes: list[list[int]] = []
    for lbl in range(1, n + 1):
        xs = np.where((labels == lbl).any(axis=0))[0]
        if xs.size == 0:
            continue
        boxes.append([int(xs[0]), int(xs[-1]) + 1])
    boxes.sort()

    merged: list[list[int]] = []
    for x0, x1 in boxes:
        if merged:
            mx0, mx1 = merged[-1]
            ov = min(mx1, x1) - max(mx0, x0)
            if ov > overlap * min(mx1 - mx0, x1 - x0):
                merged[-1][1] = max(mx1, x1)
                merged[-1][0] = min(mx0, x0)
                continue
        merged.append([x0, x1])
    return [(a, b) for a, b in merged]


def segment(
    mask: np.ndarray, *, use_components: bool = True, mark_word_spaces: bool = True
) -> list[Line]:
    """Split a mask into lines of character spans.

    With ``mark_word_spaces=False`` no space spans are added, leaving word
    spacing to the caller (the pipeline uses the recognised characters for it).
    """
    lines: list[Line] = []
    for y0, y1 in segment_lines_bounds(mask):
        sub = mask[y0:y1]
        runs = component_runs(sub) if use_components else char_runs(sub)
        if not runs:
            continue
        baseline, cap_top = _estimate_anchors(sub, runs)
        if mark_word_spaces:
            spans = mark_spaces(runs, baseline - cap_top)
        else:
            spans = [CharSpan(a, b, is_space=False) for a, b in runs]
        lines.append(Line(y0=y0, y1=y1, spans=spans, baseline=baseline, cap_top=cap_top))
    return lines


def char_boxes(lines: list[Line]) -> list[Box]:
    """Character boxes for all lines, each as tall as its line."""
    boxes: list[Box] = []
    for ln in lines:
        for sp in ln.spans:
            if sp.is_space:
                continue
            boxes.append(Box(sp.x0, ln.y0, sp.x1 - sp.x0, ln.y1 - ln.y0))
    return boxes
