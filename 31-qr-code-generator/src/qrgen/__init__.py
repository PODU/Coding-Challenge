"""A QR code generator (ISO/IEC 18004) with no QR library underneath.

Usage::

    import qrgen
    qr = qrgen.encode("HELLO CC WORLD", ec="M")
    print(qr.to_terminal())
    qr.to_png("hello.png", scale=10)
"""
from __future__ import annotations

from typing import List, Optional

from . import render
from .analyze import CapacityError, EncodingPlan, ModeError, make_plan
from .matrix import Matrix, build_base_matrix
from .mask import select_mask

__all__ = [
    "QRCode",
    "encode",
    "CapacityError",
    "ModeError",
    "EncodingPlan",
    "__version__",
]

__version__ = "1.0.0"


class QRCode:
    """A finished QR symbol and the parameters it was built with."""

    def __init__(self, matrix: Matrix, plan: EncodingPlan, mask: int) -> None:
        self._matrix = matrix
        self.modules: List[List[bool]] = matrix.modules
        self.size: int = matrix.size
        self.version: int = plan.version
        self.ec: str = plan.ec
        self.mode: str = plan.mode
        self.mask: int = mask
        self.text: str = plan.text

    def to_terminal(self, border: int = 4, invert: bool = False) -> str:
        return render.to_terminal(self.modules, border, invert)

    def to_svg(self, border: int = 4, scale: int = 10, invert: bool = False) -> str:
        return render.to_svg(self.modules, border, scale, invert)

    def to_png(
        self,
        path: str,
        border: int = 4,
        scale: int = 10,
        invert: bool = False,
    ) -> None:
        render.to_png(self.modules, path, border, scale, invert)

    def __str__(self) -> str:  # pragma: no cover - convenience
        return self.to_terminal()


def encode(
    text: str,
    ec: str = "M",
    mode: Optional[str] = None,
    version: Optional[int] = None,
    mask: Optional[int] = None,
) -> QRCode:
    """Encode ``text`` as a :class:`QRCode`.

    ``ec`` is one of L/M/Q/H. ``mode``, ``version`` (1..40) and ``mask``
    (0..7) are chosen automatically when left as ``None``.
    """
    plan = make_plan(text, ec=ec, mode=mode, version=version)
    base = build_base_matrix(plan)
    final_matrix, mask_id = select_mask(base, plan.ec, forced=mask)
    return QRCode(final_matrix, plan, mask_id)
