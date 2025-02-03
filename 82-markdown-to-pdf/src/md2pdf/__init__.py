"""md2pdf converts Markdown to PDF.

``convert_markdown`` turns a string into PDF bytes and ``convert_file`` writes
a ``.pdf`` next to the input. Both take an optional :class:`Style`.
"""

from __future__ import annotations

from .model import Document, build_document
from .render import convert_markdown, render_document
from .style import Style

__all__ = [
    "__version__",
    "Style",
    "Document",
    "build_document",
    "render_document",
    "convert_markdown",
    "convert_file",
]

__version__ = "1.0.0"


def convert_file(
    input_path: str,
    output_path: str | None = None,
    style: "Style | None" = None,
    *,
    title: str | None = None,
) -> str:
    """Convert a Markdown file and return the path of the PDF written.

    ``output_path`` defaults to the input path with a ``.pdf`` extension.
    """
    import os

    with open(input_path, "rb") as fh:
        text = _decode(fh.read())

    if output_path is None:
        base, _ = os.path.splitext(input_path)
        output_path = base + ".pdf"

    data = convert_markdown(text, style=style or Style(), title=title)
    with open(output_path, "wb") as fh:
        fh.write(data)
    return output_path


def _decode(raw: bytes) -> str:
    """Decode as UTF-8 (skipping any BOM), or latin-1 if that fails."""
    try:
        return raw.decode("utf-8-sig")
    except UnicodeDecodeError:
        return raw.decode("latin-1")
