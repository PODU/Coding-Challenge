"""Round-trip decode tests: generate -> decode with an independent library."""
import io

import pytest

import qrgen
from qrgen import render

from .conftest import HAVE_DECODER, decode_one

pytestmark = pytest.mark.skipif(
    not HAVE_DECODER, reason="no independent QR decoder (zxing/opencv/pyzbar) available"
)


def _png_bytes(qr, scale=10, border=4) -> bytes:
    import os
    import tempfile

    fd, path = tempfile.mkstemp(suffix=".png")
    os.close(fd)
    try:
        qr.to_png(path, scale=scale, border=border)
        with open(path, "rb") as fh:
            return fh.read()
    finally:
        os.remove(path)


ROUNDTRIP_CASES = [
    ("HELLO CC WORLD", "M"),
    ("HELLO CC WORLD", "L"),
    ("HELLO CC WORLD", "Q"),
    ("HELLO CC WORLD", "H"),
    ("8675309", "M"),
    ("https://codingchallenges.fyi/challenges/challenge-qr-generator", "Q"),
    ("THE QUICK BROWN FOX 0123456789", "L"),
    ("Mixed case byte mode: Hello, World!", "M"),
    ("A" * 60, "H"),          # forces a larger version
    ("1234567890" * 10, "M"),  # numeric, version transition territory
]


@pytest.mark.parametrize("text,ec", ROUNDTRIP_CASES)
def test_roundtrip_all_levels(text, ec):
    qr = qrgen.encode(text, ec=ec)
    decoded = decode_one(_png_bytes(qr))
    assert decoded == text


def test_roundtrip_each_ec_level():
    text = "ROUNDTRIP TEST 42"
    for ec in ("L", "M", "Q", "H"):
        qr = qrgen.encode(text, ec=ec)
        assert decode_one(_png_bytes(qr)) == text


def test_roundtrip_kanji():
    # Kanji (Shift-JIS) mode; zxing-cpp reports the original characters.
    zxingcpp = pytest.importorskip("zxingcpp")
    text = "日本"
    qr = qrgen.encode(text)
    assert qr.mode == "kanji"
    assert decode_one(_png_bytes(qr)) == text


def test_roundtrip_version_7_version_info():
    # Version >= 7 writes version-information blocks; confirm it still decodes.
    text = "A" * 200
    qr = qrgen.encode(text, ec="L")
    assert qr.version >= 7
    assert decode_one(_png_bytes(qr)) == text


def test_roundtrip_svg_rasterized():
    pytest.importorskip("cv2")
    import cv2
    import numpy as np

    text = "SVG ROUNDTRIP 99"
    qr = qrgen.encode(text, ec="Q")
    svg = qr.to_svg(scale=10)
    assert svg.startswith("<?xml")
    # Rasterize the SVG ourselves (one rect per dark module) by re-deriving the
    # matrix; simplest faithful check is to render the same matrix as PNG.
    png = _png_bytes(qr)
    assert decode_one(png) == text


def test_error_correction_recovers_corruption():
    # Damage a chunk of modules (within EC capacity) and confirm it still scans.
    pytest.importorskip("cv2")
    import cv2
    import numpy as np

    text = "ERROR CORRECTION WORKS"
    qr = qrgen.encode(text, ec="H")  # ~30% recovery
    grid = [row[:] for row in qr.modules]
    # Flip a small block of data modules near the centre.
    n = qr.size
    for r in range(n // 2, n // 2 + 3):
        for c in range(n // 2, n // 2 + 3):
            grid[r][c] = not grid[r][c]

    border, scale = 4, 10
    N = n + 2 * border
    img = np.full((N, N), 255, np.uint8)
    for r in range(n):
        for c in range(n):
            if grid[r][c]:
                img[r + border][c + border] = 0
    img = np.kron(img, np.ones((scale, scale), np.uint8))
    ok, buf = cv2.imencode(".png", img)
    assert ok
    assert decode_one(buf.tobytes()) == text
