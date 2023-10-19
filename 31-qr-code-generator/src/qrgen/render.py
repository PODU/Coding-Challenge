"""Output as terminal text (Unicode half blocks), SVG, or PNG via Pillow."""
from __future__ import annotations

from typing import List, Sequence

Grid = Sequence[Sequence[bool]]


def _with_quiet_zone(grid: Grid, border: int) -> List[List[bool]]:
    if border < 0:
        raise ValueError("border must be >= 0")
    size = len(grid)
    new_size = size + 2 * border
    out = [[False] * new_size for _ in range(new_size)]
    for r in range(size):
        for c in range(size):
            out[r + border][c + border] = bool(grid[r][c])
    return out


def to_terminal(grid: Grid, border: int = 4, invert: bool = False) -> str:
    """Draw two module rows per line of text using half-block characters."""
    g = _with_quiet_zone(grid, border)
    size = len(g)
    lines: List[str] = []
    for r in range(0, size, 2):
        line = []
        for c in range(size):
            top = g[r][c]
            bottom = g[r + 1][c] if r + 1 < size else False
            if invert:
                top, bottom = not top, not bottom
            if top and bottom:
                line.append("█")
            elif top:
                line.append("▀")
            elif bottom:
                line.append("▄")
            else:
                line.append(" ")
        lines.append("".join(line))
    return "\n".join(lines)


def to_svg(grid: Grid, border: int = 4, scale: int = 10, invert: bool = False) -> str:
    """Return an SVG with all dark modules drawn as a single path."""
    if scale <= 0:
        raise ValueError("scale must be > 0 for SVG")
    g = _with_quiet_zone(grid, border)
    size = len(g)
    dim = size * scale
    bg = "#000000" if invert else "#ffffff"
    fg = "#ffffff" if invert else "#000000"
    parts: List[str] = []
    for r in range(size):
        for c in range(size):
            if g[r][c]:
                parts.append(f"M{c * scale},{r * scale}h{scale}v{scale}h-{scale}z")
    path = "".join(parts)
    return (
        f'<?xml version="1.0" encoding="UTF-8"?>\n'
        f'<svg xmlns="http://www.w3.org/2000/svg" version="1.1" '
        f'width="{dim}" height="{dim}" viewBox="0 0 {dim} {dim}" '
        f'shape-rendering="crispEdges">\n'
        f'<rect width="{dim}" height="{dim}" fill="{bg}"/>\n'
        f'<path d="{path}" fill="{fg}"/>\n'
        f'</svg>\n'
    )


def to_png(
    grid: Grid,
    path: str,
    border: int = 4,
    scale: int = 10,
    invert: bool = False,
) -> None:
    """Write a PNG file. Needs Pillow."""
    if scale <= 0:
        raise ValueError("scale must be > 0 for PNG")
    try:
        from PIL import Image
    except ImportError as exc:  # pragma: no cover
        raise RuntimeError(
            "PNG output requires Pillow. Install with: pip install Pillow"
        ) from exc
    g = _with_quiet_zone(grid, border)
    size = len(g)
    dark, light = (255, 0) if invert else (0, 255)
    img = Image.new("L", (size, size), light)
    px = img.load()
    for r in range(size):
        for c in range(size):
            if g[r][c]:
                px[c, r] = dark
    img = img.resize((size * scale, size * scale), Image.NEAREST)
    img.save(path)
