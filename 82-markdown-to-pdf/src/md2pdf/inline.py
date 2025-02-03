"""Turns inline nodes into the HTML-like markup reportlab's Paragraph reads.

All text from the document is XML-escaped, so a stray ``<`` or ``&`` in the
Markdown can't break the markup.
"""

from __future__ import annotations

from xml.sax.saxutils import escape, quoteattr

from .model import Bold, Code, HardBreak, Inline, Italic, Link, SoftBreak, Text
from .style import Style


def escape_text(text: str) -> str:
    """Escape ``&``, ``<`` and ``>`` for reportlab markup."""
    return escape(text)


def inlines_to_markup(nodes: list[Inline], style: Style) -> str:
    return "".join(_node_to_markup(node, style) for node in nodes)


def _node_to_markup(node: Inline, style: Style) -> str:
    if isinstance(node, Text):
        return escape_text(node.text)

    if isinstance(node, Bold):
        return f"<b>{inlines_to_markup(node.children, style)}</b>"

    if isinstance(node, Italic):
        return f"<i>{inlines_to_markup(node.children, style)}</i>"

    if isinstance(node, Code):
        return (
            f'<font face="{style.mono_font}" backColor="{style.code_bg}" '
            f'color="{style.code_color}">{escape_text(node.text)}</font>'
        )

    if isinstance(node, Link):
        inner = inlines_to_markup(node.children, style)
        if not inner:
            inner = escape_text(node.href)
        href = quoteattr(node.href)
        return (
            f'<a href={href} color="{style.link_color}">'
            f"<u>{inner}</u></a>"
        )

    if isinstance(node, SoftBreak):
        return " "

    if isinstance(node, HardBreak):
        return "<br/>"

    return ""  # pragma: no cover
