"""Shared test fixtures and decoder helpers.

Round-trip tests decode our generated images with *independent* libraries
(zxing-cpp and/or OpenCV). Each is optional; tests skip when none is available.
"""
from __future__ import annotations

import io
from typing import List, Optional

import pytest

try:  # pragma: no cover - availability probe
    import numpy as np
except ImportError:  # pragma: no cover
    np = None

try:  # pragma: no cover
    import cv2
except ImportError:  # pragma: no cover
    cv2 = None

try:  # pragma: no cover
    import zxingcpp
except ImportError:  # pragma: no cover
    zxingcpp = None

try:  # pragma: no cover
    from pyzbar import pyzbar
except Exception:  # pragma: no cover - native lib may be missing on Windows
    pyzbar = None

try:  # pragma: no cover
    import qrcode as _qrcode
except ImportError:  # pragma: no cover
    _qrcode = None


HAVE_DECODER = (cv2 is not None or zxingcpp is not None or pyzbar is not None) and np is not None
HAVE_QRCODE = _qrcode is not None and np is not None


def _png_to_gray(png_bytes: bytes):
    arr = np.frombuffer(png_bytes, dtype=np.uint8)
    if cv2 is not None:
        return cv2.imdecode(arr, cv2.IMREAD_GRAYSCALE)
    from PIL import Image  # pragma: no cover
    return np.array(Image.open(io.BytesIO(png_bytes)).convert("L"))


def decode_png(png_bytes: bytes) -> List[str]:
    """Decode a PNG with every available independent decoder; return texts."""
    img = _png_to_gray(png_bytes)
    results: List[str] = []
    if zxingcpp is not None:
        for r in zxingcpp.read_barcodes(img):
            if r.text:
                results.append(r.text)
    if not results and cv2 is not None:
        text, pts, _ = cv2.QRCodeDetector().detectAndDecode(img)
        if pts is not None and text:
            results.append(text)
    if not results and pyzbar is not None:  # pragma: no cover
        for d in pyzbar.decode(img):
            results.append(d.data.decode("latin-1"))
    return results


def decode_one(png_bytes: bytes) -> Optional[str]:
    res = decode_png(png_bytes)
    return res[0] if res else None
